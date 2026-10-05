package updates

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/catalog"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/inventory"
)

func discoverCurseForgeCandidate(
	ctx context.Context,
	client *CurseForgeClient,
	entry catalog.Entry,
	installed map[string]catalog.Entry,
	opts Options,
) Candidate {
	candidate := Candidate{
		Key:        "curseforge:" + entry.ProjectID,
		Provider:   "curseforge",
		ProjectID:  entry.ProjectID,
		Name:       entry.Name,
		Side:       entry.Side,
		Deployment: entry.Deployment,
	}

	project, err := client.GetMod(ctx, entry.ProjectID)
	if err != nil {
		candidate.Installed = installedRelease(entry)
		candidate.Classification = ClassificationBlocked
		candidate.Reasons = []Reason{{Code: "provider_lookup_failed", Message: err.Error()}}
		return candidate
	}
	if project.Name != "" {
		candidate.Name = project.Name
	}
	candidate.ProjectURL = project.Links.WebsiteURL
	if candidate.ProjectURL == "" && project.Slug != "" {
		candidate.ProjectURL = "https://www.curseforge.com/minecraft/mc-mods/" + project.Slug
	}
	candidate.IconURL = project.Logo.ThumbnailURL

	current, err := client.GetFile(ctx, entry.ProjectID, entry.FileID)
	if err != nil {
		candidate.Installed = installedRelease(entry)
		candidate.Classification = ClassificationBlocked
		candidate.Reasons = []Reason{{
			Code: "installed_file_lookup_failed",
			Message: "The installed CurseForge file could not be loaded: " + err.Error(),
		}}
		return candidate
	}
	candidate.Installed = releaseFromCurseForge(current, "")
	candidate.Installed.SHA512 = entry.SHA512

	files, err := client.ListFiles(ctx, entry.ProjectID, opts.Minecraft, opts.Loader)
	if err != nil {
		candidate.Classification = ClassificationBlocked
		candidate.Reasons = []Reason{{Code: "provider_lookup_failed", Message: err.Error()}}
		return candidate
	}

	valid := make([]curseForgeFile, 0)
	for _, file := range files {
		if !file.IsAvailable || !file.FileDate.After(current.FileDate) {
			continue
		}
		valid = append(valid, file)
	}
	if len(valid) == 0 {
		candidate.Classification = ClassificationUpToDate
		return candidate
	}
	sort.Slice(valid, func(i, j int) bool { return valid[i].FileDate.After(valid[j].FileDate) })

	targetFile := valid[0]
	targetURL, err := client.DownloadURL(ctx, entry.ProjectID, targetFile)
	if err != nil {
		candidate.Classification = ClassificationBlocked
		candidate.Reasons = []Reason{{
			Code: "target_download_unavailable",
			Message: "CurseForge target download is unavailable: " + err.Error(),
		}}
		return candidate
	}
	target := releaseFromCurseForge(targetFile, targetURL)
	candidate.Target = &target
	candidate.Classification = ClassificationSafe
	if target.SHA1 == "" {
		promote(&candidate, ClassificationBlocked, Reason{
			Code: "target_checksum_missing",
			Message: "CurseForge did not provide a SHA-1 checksum for the target file.",
		})
	}
	if target.Channel != "release" {
		promote(&candidate, ClassificationReview, Reason{
			Code: "prerelease",
			Message: "The newest compatible CurseForge file is a " + target.Channel + " release.",
		})
	}

	ascending := append([]curseForgeFile(nil), valid...)
	sort.Slice(ascending, func(i, j int) bool { return ascending[i].FileDate.Before(ascending[j].FileDate) })
	for _, file := range ascending {
		body, changelogErr := client.Changelog(ctx, entry.ProjectID, file.ID)
		if changelogErr != nil {
			body = ""
		}
		candidate.Changelogs = append(candidate.Changelogs, ChangelogEntry{
			ID: strconv.Itoa(file.ID),
			Number: file.DisplayName,
			Name: file.DisplayName,
			PublishedAt: file.FileDate,
			Channel: curseForgeReleaseType(file.ReleaseType),
			Body: body,
		})
	}

	resolver := curseForgeDependencyResolver{
		ctx: ctx,
		client: client,
		installed: installed,
		opts: opts,
	}
	for _, dependency := range targetFile.Dependencies {
		candidate.Dependencies = append(
			candidate.Dependencies,
			resolver.resolve(dependency.ModID, dependency.RelationType, entry.Deployment, &candidate, map[string]bool{}),
		)
	}
	return candidate
}

type curseForgeDependencyResolver struct {
	ctx       context.Context
	client    *CurseForgeClient
	installed map[string]catalog.Entry
	opts      Options
}

func (r curseForgeDependencyResolver) resolve(
	modID int,
	relationType int,
	parentDeployment inventory.Location,
	candidate *Candidate,
	visiting map[string]bool,
) Dependency {
	projectID := strconv.Itoa(modID)
	result := Dependency{
		Provider: "curseforge",
		ProjectID: projectID,
		Type: curseForgeRelationType(relationType),
		Action: "none",
		Deployment: parentDeployment,
	}

	installedEntry, exists := r.installed[projectID]
	if exists {
		result.InstalledVersion = strconv.FormatUint(uint64(installedEntry.FileID), 10)
		result.Name = installedEntry.Name
		result.Deployment = installedEntry.Deployment
	}

	switch relationType {
	case 3: // required
		if exists {
			result.Action = "satisfied"
			return result
		}

		project, projectErr := r.client.GetMod(r.ctx, projectID)
		if projectErr == nil && project.Name != "" {
			result.Name = project.Name
		}
		if result.Name == "" {
			result.Name = projectID
		}

		files, err := r.client.ListFiles(r.ctx, projectID, r.opts.Minecraft, r.opts.Loader)
		if err != nil {
			result.Action = "unresolved"
			promote(candidate, ClassificationBlocked, Reason{
				Code: "required_dependency_unresolved",
				Message: fmt.Sprintf("Required CurseForge dependency %s could not be resolved: %v", result.Name, err),
			})
			return result
		}
		compatible := make([]curseForgeFile, 0, len(files))
		for _, file := range files {
			if file.IsAvailable {
				compatible = append(compatible, file)
			}
		}
		if len(compatible) == 0 {
			result.Action = "unresolved"
			promote(candidate, ClassificationBlocked, Reason{
				Code: "required_dependency_unresolved",
				Message: "Required CurseForge dependency " + result.Name + " has no compatible NeoForge file.",
			})
			return result
		}
		sort.Slice(compatible, func(i, j int) bool {
			return compatible[i].FileDate.After(compatible[j].FileDate)
		})
		targetFile := compatible[0]
		downloadURL, err := r.client.DownloadURL(r.ctx, projectID, targetFile)
		if err != nil {
			result.Action = "unresolved"
			promote(candidate, ClassificationBlocked, Reason{
				Code: "required_dependency_download_unavailable",
				Message: "Required dependency " + result.Name + " has no downloadable compatible file.",
			})
			return result
		}
		release := releaseFromCurseForge(targetFile, downloadURL)
		if release.SHA1 == "" {
			result.Action = "unresolved"
			promote(candidate, ClassificationBlocked, Reason{
				Code: "required_dependency_checksum_missing",
				Message: "Required dependency " + result.Name + " has no provider checksum.",
			})
			return result
		}
		result.Action = "add"
		result.TargetVersion = release.ID
		result.Target = &release
		promote(candidate, ClassificationReview, Reason{
			Code: "required_dependency_addition",
			Message: "The target file requires the additional CurseForge project " + result.Name + ".",
		})

		visitKey := projectID + ":" + release.ID
		if visiting[visitKey] {
			return result
		}
		visiting[visitKey] = true
		for _, nested := range targetFile.Dependencies {
			result.Dependencies = append(
				result.Dependencies,
				r.resolve(nested.ModID, nested.RelationType, result.Deployment, candidate, visiting),
			)
		}
		delete(visiting, visitKey)

	case 5: // incompatible
		if exists {
			result.Action = "conflict"
			promote(candidate, ClassificationBlocked, Reason{
				Code: "incompatible_dependency_installed",
				Message: "The target CurseForge file declares installed project " + projectID + " incompatible.",
			})
		}
	case 2:
		result.Action = "optional"
	case 1:
		result.Action = "embedded"
	default:
		result.Action = "none"
	}
	return result
}

func releaseFromCurseForge(file curseForgeFile, downloadURL string) Release {
	return Release{
		ID: strconv.Itoa(file.ID),
		Number: file.DisplayName,
		Name: file.DisplayName,
		PublishedAt: file.FileDate,
		Channel: curseForgeReleaseType(file.ReleaseType),
		Filename: file.FileName,
		URL: downloadURL,
		SHA1: curseForgeSHA1(file),
	}
}

func curseForgeRelationType(value int) string {
	switch value {
	case 1:
		return "embedded"
	case 2:
		return "optional"
	case 3:
		return "required"
	case 4:
		return "tool"
	case 5:
		return "incompatible"
	case 6:
		return "include"
	default:
		return "unknown"
	}
}

func curseForgeEntryID(entry catalog.Entry) string {
	if entry.FileID != 0 {
		return strconv.FormatUint(uint64(entry.FileID), 10)
	}
	return strings.TrimSpace(entry.VersionID)
}
