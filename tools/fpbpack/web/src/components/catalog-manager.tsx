'use client';

import {useEffect, useMemo, useState} from 'react';
import {ExternalLink, PackagePlus, Search, X} from 'lucide-react';
import {useRouter} from 'next/navigation';
import {api} from '@/lib/api';
import {Pill, formatDate} from '@/components/ui';
import type {
  CatalogProject,
  CatalogVersion,
  ManagementMod,
  UpdatePlan,
} from '@/lib/management';

type Provider = 'modrinth' | 'curseforge';
type Placement = 'server' | 'client';

export function CatalogManagerDialog({
  mode,
  mod,
  onClose,
}: {
  mode: 'install' | 'version';
  mod?: ManagementMod;
  onClose: () => void;
}) {
  const router = useRouter();
  const fixedProvider =
    mod?.provider === 'modrinth' || mod?.provider === 'curseforge'
      ? mod.provider
      : undefined;
  const [provider, setProvider] = useState<Provider>(fixedProvider ?? 'modrinth');
  const [query, setQuery] = useState('');
  const [projects, setProjects] = useState<CatalogProject[]>([]);
  const [selectedProject, setSelectedProject] = useState<CatalogProject | null>(null);
  const [versions, setVersions] = useState<CatalogVersion[]>([]);
  const [selectedVersionID, setSelectedVersionID] = useState('');
  const [placement, setPlacement] = useState<Placement>(
    mod?.preferred_deployment ?? mod?.deployment ?? 'server',
  );
  const [channel, setChannel] = useState<'all' | 'release' | 'beta' | 'alpha'>('release');
  const [searching, setSearching] = useState(false);
  const [loadingVersions, setLoadingVersions] = useState(false);
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const loadVersions = async (
    projectProvider: Provider,
    projectID: string,
  ) => {
    setLoadingVersions(true);
    setError(null);
    setVersions([]);
    setSelectedVersionID('');
    try {
      const response = await api<{versions: CatalogVersion[]}>(
        '/api/catalog/projects/' +
          encodeURIComponent(projectProvider) +
          '/' +
          encodeURIComponent(projectID) +
          '/versions',
      );
      const next = Array.isArray(response.versions) ? response.versions : [];
      setVersions(next);
      const firstRelease = next.find((version) => version.channel === 'release');
      const defaultVersion = firstRelease ?? next[0];
      setSelectedVersionID(defaultVersion?.id ?? '');
      if (
        defaultVersion?.environment === 'client_only' ||
        defaultVersion?.environment === 'singleplayer_only'
      ) {
        setPlacement('client');
      } else if (
        defaultVersion?.environment === 'server_only' ||
        defaultVersion?.environment === 'dedicated_server_only'
      ) {
        setPlacement('server');
      }
    } catch (value: unknown) {
      setError(value instanceof Error ? value.message : String(value));
    } finally {
      setLoadingVersions(false);
    }
  };

  useEffect(() => {
    if (mode !== 'version' || !mod || !fixedProvider || !mod.project_id) return;
    const project: CatalogProject = {
      provider: fixedProvider,
      project_id: mod.project_id,
      name: mod.name,
      project_url: mod.project_url,
      installed: true,
      installed_key: mod.id,
    };
    setProvider(fixedProvider);
    setSelectedProject(project);
    void loadVersions(fixedProvider, mod.project_id);
  }, [mode, mod, fixedProvider]);

  const searchProjects = async () => {
    if (!query.trim()) return;
    setSearching(true);
    setError(null);
    setProjects([]);
    setSelectedProject(null);
    setVersions([]);
    setSelectedVersionID('');
    try {
      const params = new URLSearchParams({provider, q: query.trim()});
      const response = await api<{projects: CatalogProject[]}>(
        '/api/catalog/search?' + params.toString(),
      );
      setProjects(Array.isArray(response.projects) ? response.projects : []);
    } catch (value: unknown) {
      setError(value instanceof Error ? value.message : String(value));
    } finally {
      setSearching(false);
    }
  };

  const chooseProject = (project: CatalogProject) => {
    if (project.installed) return;
    setSelectedProject(project);
    if (strictClientOnly(project.environment)) {
      setPlacement('client');
    }
    void loadVersions(project.provider as Provider, project.project_id);
  };

  const selectedVersion = versions.find((version) => version.id === selectedVersionID);
  const filteredVersions = useMemo(
    () =>
      versions.filter(
        (version) => channel === 'all' || (version.channel || 'release') === channel,
      ),
    [versions, channel],
  );
  const latestReleaseID = versions.find(
    (version) => (version.channel || 'release') === 'release',
  )?.id;

  const chooseVersion = (version: CatalogVersion) => {
    setSelectedVersionID(version.id);
    if (version.environment === 'client_only' || version.environment === 'singleplayer_only') {
      setPlacement('client');
    } else if (
      version.environment === 'server_only' ||
      version.environment === 'dedicated_server_only'
    ) {
      setPlacement('server');
    }
  };

  const createPlan = async () => {
    if (!selectedProject || !selectedVersionID) return;
    setCreating(true);
    setError(null);
    try {
      const plan = await api<UpdatePlan>('/api/catalog/plans', {
        method: 'POST',
        body: JSON.stringify(
          mode === 'install'
            ? {
                action: 'install',
                provider: selectedProject.provider,
                project_id: selectedProject.project_id,
                version_id: selectedVersionID,
                placement,
              }
            : {
                action: 'version',
                path: mod?.path,
                version_id: selectedVersionID,
                placement,
              },
        ),
      });
      onClose();
      router.push('/review?id=' + encodeURIComponent(plan.id));
    } catch (value: unknown) {
      setError(value instanceof Error ? value.message : String(value));
    } finally {
      setCreating(false);
    }
  };

  return (
    <div className="modal modal-open">
      <div className="modal-box max-w-5xl">
        <div className="flex items-start justify-between gap-4">
          <div>
            <div className="section-label">Catalog management</div>
            <h2 className="mt-1 text-lg font-semibold">
              {mode === 'install' ? 'Add a mod' : 'Change version'}
            </h2>
            <p className="mt-1 text-sm text-base-content/55">
              {mode === 'install'
                ? 'Search compatible provider projects, choose an exact release, then review the verified plan before anything changes.'
                : 'Choose an exact compatible provider release. Downgrades and reinstalls use the same Review → Apply → Restore path.'}
            </p>
          </div>
          <button className="btn btn-sm btn-ghost" type="button" onClick={onClose}>
            <X size={15} />
          </button>
        </div>

        {error ? <div className="alert alert-error mt-4 py-2 text-sm">{error}</div> : null}

        {mode === 'install' ? (
          <div className="mt-5 grid gap-3 sm:grid-cols-[10rem_minmax(0,1fr)_auto]">
            <select
              className="select select-bordered"
              value={provider}
              onChange={(event) => {
                setProvider(event.target.value as Provider);
                setProjects([]);
                setSelectedProject(null);
                setVersions([]);
              }}
            >
              <option value="modrinth">Modrinth</option>
              <option value="curseforge">CurseForge</option>
            </select>
            <input
              className="input input-bordered"
              value={query}
              placeholder="Search mods by name…"
              onChange={(event) => setQuery(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === 'Enter') void searchProjects();
              }}
            />
            <button
              className="btn btn-primary"
              type="button"
              disabled={searching || !query.trim()}
              onClick={() => void searchProjects()}
            >
              {searching ? <span className="loading loading-spinner loading-xs" /> : <Search size={14} />}
              Search
            </button>
          </div>
        ) : null}

        <div className="mt-5 grid min-h-[28rem] gap-4 lg:grid-cols-[minmax(0,0.9fr)_minmax(0,1.25fr)]">
          <section className="panel overflow-hidden">
            <div className="panel-header">
              <div>
                <div className="section-label">
                  {mode === 'install' ? 'Project' : 'Managed mod'}
                </div>
                <h3 className="mt-0.5 text-sm font-semibold">
                  {selectedProject?.name ?? (mode === 'install' ? 'Search results' : mod?.name)}
                </h3>
              </div>
            </div>

            {mode === 'install' && !selectedProject ? (
              projects.length ? (
                <div className="max-h-[31rem] divide-y divide-base-300 overflow-y-auto">
                  {projects.map((project) => (
                    <button
                      className="block w-full p-4 text-left hover:bg-base-200/55 disabled:cursor-not-allowed disabled:opacity-55"
                      type="button"
                      disabled={project.installed}
                      key={project.provider + ':' + project.project_id}
                      onClick={() => chooseProject(project)}
                    >
                      <div className="flex items-center gap-2">
                        <span className="font-medium">{project.name}</span>
                        <Pill tone={project.installed ? 'good' : 'neutral'}>
                          {project.installed ? 'already managed' : project.provider}
                        </Pill>
                      </div>
                      {project.summary ? (
                        <p className="mt-1 line-clamp-2 text-xs leading-5 text-base-content/50">
                          {project.summary}
                        </p>
                      ) : null}
                      <div className="mt-2 text-[0.68rem] text-base-content/35">
                        {project.downloads ? project.downloads.toLocaleString() + ' downloads' : project.project_id}
                      </div>
                    </button>
                  ))}
                </div>
              ) : (
                <div className="p-6 text-sm text-base-content/45">
                  Search Modrinth or CurseForge to choose a compatible project.
                </div>
              )
            ) : selectedProject ? (
              <div className="p-4">
                <div className="font-medium">{selectedProject.name}</div>
                <div className="mono mt-1 text-xs text-base-content/40">
                  {selectedProject.provider}:{selectedProject.project_id}
                </div>
                {selectedProject.summary ? (
                  <p className="mt-3 text-sm leading-6 text-base-content/60">
                    {selectedProject.summary}
                  </p>
                ) : null}
                {selectedProject.project_url ? (
                  <a
                    className="btn btn-sm btn-ghost mt-3"
                    href={selectedProject.project_url}
                    target="_blank"
                    rel="noreferrer"
                  >
                    Provider page <ExternalLink size={13} />
                  </a>
                ) : null}
                <label className="form-control mt-5">
                  <span className="mb-1 text-xs text-base-content/45">Preferred placement</span>
                  <select
                    className="select select-sm select-bordered"
                    value={placement}
                    onChange={(event) => setPlacement(event.target.value as Placement)}
                  >
                    <option value="server">Server/common</option>
                    <option value="client">Client-only (AutoModpack)</option>
                  </select>
                </label>
                {mode === 'install' ? (
                  <button
                    className="btn btn-xs btn-ghost mt-3"
                    type="button"
                    onClick={() => {
                      setSelectedProject(null);
                      setVersions([]);
                      setSelectedVersionID('');
                    }}
                  >
                    Choose another project
                  </button>
                ) : null}
              </div>
            ) : null}
          </section>

          <section className="panel overflow-hidden">
            <div className="panel-header gap-3">
              <div>
                <div className="section-label">Exact version</div>
                <h3 className="mt-0.5 text-sm font-semibold">
                  {selectedVersion?.number || 'Choose a release'}
                </h3>
              </div>
              <select
                className="select select-xs select-bordered"
                value={channel}
                onChange={(event) =>
                  setChannel(event.target.value as 'all' | 'release' | 'beta' | 'alpha')
                }
              >
                <option value="release">Release</option>
                <option value="beta">Beta</option>
                <option value="alpha">Alpha</option>
                <option value="all">All channels</option>
              </select>
            </div>

            {loadingVersions ? (
              <div className="grid place-items-center p-10">
                <span className="loading loading-spinner loading-sm" />
              </div>
            ) : !selectedProject ? (
              <div className="p-6 text-sm text-base-content/45">
                Choose a project first.
              </div>
            ) : filteredVersions.length ? (
              <div className="max-h-[31rem] divide-y divide-base-300 overflow-y-auto">
                {filteredVersions.map((version) => (
                  <button
                    className={[
                      'block w-full p-4 text-left hover:bg-base-200/55',
                      selectedVersionID === version.id ? 'bg-base-200/70' : '',
                    ].join(' ')}
                    type="button"
                    key={version.id}
                    onClick={() => chooseVersion(version)}
                  >
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="font-medium">{version.number || version.name || version.id}</span>
                      <Pill tone={version.channel === 'release' ? 'good' : 'warn'}>
                        {version.channel || 'release'}
                      </Pill>
                      {version.id === latestReleaseID ? (
                        <Pill tone="blue">latest stable</Pill>
                      ) : null}
                      {mod?.installed_version === version.number ? (
                        <Pill tone="neutral">installed</Pill>
                      ) : null}
                    </div>
                    <div className="mt-1 flex flex-wrap gap-x-3 text-[0.7rem] text-base-content/40">
                      <span>{formatDate(version.published_at)}</span>
                      {version.filename ? <span className="mono">{version.filename}</span> : null}
                      {version.environment ? <span>{version.environment}</span> : null}
                    </div>
                    {version.changelog ? (
                      <p className="mt-2 line-clamp-3 whitespace-pre-wrap text-xs leading-5 text-base-content/55">
                        {version.changelog}
                      </p>
                    ) : null}
                  </button>
                ))}
              </div>
            ) : (
              <div className="p-6 text-sm text-base-content/45">
                No versions match this channel. Try “All channels”.
              </div>
            )}
          </section>
        </div>

        <div className="modal-action items-center">
          {selectedVersion?.manual_download ? (
            <span className="mr-auto text-xs text-warning">
              This release requires a manual provider download during Review.
            </span>
          ) : null}
          <button className="btn btn-ghost" type="button" onClick={onClose}>
            Cancel
          </button>
          <button
            className="btn btn-primary"
            type="button"
            disabled={creating || !selectedProject || !selectedVersionID}
            onClick={() => void createPlan()}
          >
            {creating ? <span className="loading loading-spinner loading-xs" /> : <PackagePlus size={14} />}
            Review plan
          </button>
        </div>
      </div>
      <button className="modal-backdrop" type="button" onClick={onClose}>
        close
      </button>
    </div>
  );
}

function strictClientOnly(environments?: string[]) {
  if (!environments?.length) return false;
  return environments.every(
    (environment) => environment === 'client_only' || environment === 'singleplayer_only',
  );
}
