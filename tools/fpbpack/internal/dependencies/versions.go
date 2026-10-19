package dependencies

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Masterminds/semver/v3"
)

var numericMavenVersion = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+){0,2}$`)

// Matches returns an error for unsupported syntax instead of accepting a guess.
// Fabric arrays are alternatives. Forge/NeoForge intervals have Maven boundaries.
func Matches(version string, alternatives []string, loader string) (bool, error) {
	if len(alternatives) == 0 {
		return true, nil
	}
	var unknown error
	for _, expression := range alternatives {
		expression = strings.TrimSpace(expression)
		if expression == "" || expression == "*" {
			return true, nil
		}
		if expression == version || (loader != "fabric" && expression == "["+version+"]") {
			return true, nil
		}
		if loader != "fabric" && !strings.ContainsAny(expression, "[]()") {
			if version == expression {
				return true, nil
			}
			unknown = fmt.Errorf("unbounded Maven recommendation %q is not an enforced range", expression)
			continue
		}
		if loader != "fabric" {
			if !numericMavenVersion.MatchString(version) {
				unknown = fmt.Errorf("non-numeric Maven version requires loader validation: %s", version)
				continue
			}
			converted, err := mavenConstraint(expression)
			if err != nil {
				unknown = err
				continue
			}
			expression = converted
		}
		parsed, err := semver.NewVersion(version)
		if err != nil {
			unknown = err
			continue
		}
		constraint, err := semver.NewConstraint(expression)
		if err != nil {
			unknown = err
			continue
		}
		// Maven qualifiers and Fabric's prerelease rules are not interchangeable
		// with npm's prerelease exclusion. Do not label such comparisons safe.
		if parsed.Prerelease() != "" || (loader != "fabric" && parsed.Metadata() != "") {
			unknown = fmt.Errorf("prerelease range requires loader-specific validation: %s", version)
			continue
		}
		if constraint.Check(parsed) {
			return true, nil
		}
	}
	return false, unknown
}

func mavenConstraint(expression string) (string, error) {
	parts := []string{}
	for expression != "" {
		expression = strings.TrimSpace(expression)
		if len(expression) < 2 || (expression[0] != '[' && expression[0] != '(') {
			return "", fmt.Errorf("unsupported Maven range %q", expression)
		}
		end := strings.IndexAny(expression, "])")
		if end < 0 {
			return "", fmt.Errorf("unterminated Maven range %q", expression)
		}
		interval := expression[:end+1]
		bounds := strings.Split(interval[1:len(interval)-1], ",")
		constraints := []string{}
		for index, bound := range bounds {
			bound = strings.TrimSpace(bound)
			if bound == "" {
				continue
			}
			if !numericMavenVersion.MatchString(bound) {
				return "", fmt.Errorf("unsupported Maven bound %q", bound)
			}
			parsed, err := semver.NewVersion(bound)
			if err != nil || parsed.Prerelease() != "" || parsed.Metadata() != "" {
				return "", fmt.Errorf("unsupported Maven bound %q", bound)
			}
			bounds[index] = parsed.String()
		}
		if len(bounds) == 1 && interval[0] == '[' && interval[len(interval)-1] == ']' {
			constraints = append(constraints, "= "+strings.TrimSpace(bounds[0]))
		} else if len(bounds) == 2 {
			if lower := strings.TrimSpace(bounds[0]); lower != "" {
				op := ">"
				if interval[0] == '[' {
					op = ">="
				}
				constraints = append(constraints, op+" "+lower)
			}
			if upper := strings.TrimSpace(bounds[1]); upper != "" {
				op := "<"
				if interval[len(interval)-1] == ']' {
					op = "<="
				}
				constraints = append(constraints, op+" "+upper)
			}
		} else {
			return "", fmt.Errorf("unsupported Maven interval %q", interval)
		}
		if len(constraints) == 0 {
			constraints = append(constraints, "*")
		}
		parts = append(parts, strings.Join(constraints, ", "))
		expression = strings.TrimSpace(expression[end+1:])
		if expression != "" {
			if expression[0] != ',' {
				return "", fmt.Errorf("unsupported Maven union")
			}
			expression = expression[1:]
		}
	}
	return strings.Join(parts, " || "), nil
}
