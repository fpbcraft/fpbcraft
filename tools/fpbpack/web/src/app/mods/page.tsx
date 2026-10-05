'use client';

import {useEffect, useMemo, useState} from 'react';
import {
  ExternalLink,
  Github,
  RefreshCw,
  Search,
  ShieldOff,
  Trash2,
  Wrench,
  X,
} from 'lucide-react';
import {PageHeader, Pill, formatDate} from '@/components/ui';
import {useManagement} from '@/components/management-provider';
import {api} from '@/lib/api';
import type {
  DiagnosticFinding,
  ManagementMod,
  UpdateCandidate,
  UpdateRule,
} from '@/lib/management';

const PAGE_SIZE = 50;

interface ModManagementResult {
  action: string;
  path: string;
  management?: string;
  provider?: string;
  project_id?: string;
  message: string;
}

function normalizePath(value?: string) {
  return (value ?? '').replaceAll('\\', '/').replace(/^\.\//, '');
}

function managementTone(
  management: ManagementMod['management'],
): 'good' | 'warn' | 'bad' | 'neutral' {
  if (management === 'managed') return 'good';
  if (management === 'unmanaged') return 'warn';
  if (management === 'unresolved' || management === 'external') return 'bad';
  return 'neutral';
}

function updateTone(candidate?: UpdateCandidate): 'good' | 'warn' | 'bad' | 'neutral' {
  if (!candidate || candidate.classification === 'up_to_date') return 'good';
  if (candidate.classification === 'safe') return 'good';
  if (candidate.classification === 'review') return 'warn';
  if (candidate.classification === 'blocked') return 'bad';
  return 'neutral';
}

export default function ModsPage() {
  const {state, connectionStatus, reload} = useManagement();
  const [q, setQ] = useState('');
  const [deployment, setDeployment] = useState('all');
  const [management, setManagement] = useState('all');
  const [provider, setProvider] = useState('all');
  const [updateStatus, setUpdateStatus] = useState('all');
  const [attentionOnly, setAttentionOnly] = useState(false);
  const [page, setPage] = useState(1);
  const [selectedMod, setSelectedMod] = useState<ManagementMod | null>(null);
  const [rules, setRules] = useState<Record<string, UpdateRule>>({});
  const [ruleError, setRuleError] = useState<string | null>(null);
  const [managementError, setManagementError] = useState<string | null>(null);
  const [managementMessage, setManagementMessage] = useState<string | null>(null);
  const [managementBusy, setManagementBusy] = useState(false);
  const [githubRepository, setGithubRepository] = useState('');
  const [githubTag, setGithubTag] = useState('');
  const [githubAsset, setGithubAsset] = useState('');

  const candidatesByKey = useMemo(
    () => new Map(state.updates.candidates.map((candidate) => [candidate.key, candidate])),
    [state.updates.candidates],
  );

  const blockers = useMemo(
    () => state.diagnostics.findings.filter((finding) => finding.level === 'blocking'),
    [state.diagnostics.findings],
  );

  const blockerPaths = useMemo(
    () => new Set(blockers.map((finding) => normalizePath(finding.path)).filter(Boolean)),
    [blockers],
  );

  useEffect(() => {
    api<{rules: Record<string, UpdateRule>}>('/api/update-rules')
      .then((response) => setRules(response.rules))
      .catch(() => undefined);

    if (new URLSearchParams(window.location.search).get('attention') === '1') {
      setAttentionOnly(true);
    }
  }, []);

  const providers = useMemo(
    () =>
      [...new Set(state.mods.map((mod) => mod.provider).filter(Boolean))]
        .map(String)
        .sort((a, b) => a.localeCompare(b)),
    [state.mods],
  );

  const filtered = useMemo(() => {
    const needle = q.trim().toLowerCase();
    return state.mods.filter((mod) => {
      const text = [
        mod.name,
        mod.filename,
        mod.installed_version ?? '',
        mod.provider ?? '',
        mod.project_id ?? '',
      ]
        .join(' ')
        .toLowerCase();
      const candidate = candidatesByKey.get(mod.id);
      const statusMatches =
        updateStatus === 'all' ||
        (updateStatus === 'available' &&
          !!candidate?.target &&
          candidate.classification !== 'ignored') ||
        (updateStatus === 'ignored' && candidate?.classification === 'ignored') ||
        (updateStatus === 'blocked' && candidate?.classification === 'blocked') ||
        (updateStatus === 'up_to_date' && candidate?.classification === 'up_to_date');
      const needsAttention =
        blockerPaths.has(normalizePath(mod.path)) ||
        mod.management === 'unresolved' ||
        mod.management === 'external';

      return (
        (!needle || text.includes(needle)) &&
        (deployment === 'all' || mod.deployment === deployment) &&
        (management === 'all' || mod.management === management) &&
        (provider === 'all' || mod.provider === provider) &&
        statusMatches &&
        (!attentionOnly || needsAttention)
      );
    });
  }, [
    state.mods,
    candidatesByKey,
    blockerPaths,
    q,
    deployment,
    management,
    provider,
    updateStatus,
    attentionOnly,
  ]);

  const missingFindings = useMemo(
    () =>
      blockers.filter((finding) => {
        if (!finding.path) return true;
        return !state.mods.some(
          (mod) => normalizePath(mod.path) === normalizePath(finding.path),
        );
      }),
    [blockers, state.mods],
  );

  const totalPages = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE));
  const safePage = Math.min(page, totalPages);
  const rows = filtered.slice((safePage - 1) * PAGE_SIZE, safePage * PAGE_SIZE);

  const updateFilter = (setter: (value: string) => void, value: string) => {
    setter(value);
    setPage(1);
  };

  const selectedCandidate = selectedMod ? candidatesByKey.get(selectedMod.id) : undefined;
  const selectedRule = selectedCandidate ? rules[selectedCandidate.key] : undefined;

  const openMod = (mod: ManagementMod) => {
    const candidate = candidatesByKey.get(mod.id);
    setSelectedMod(mod);
    setManagementError(null);
    setManagementMessage(null);
    setGithubRepository(mod.provider === 'github' ? mod.project_id ?? '' : '');
    setGithubTag(mod.provider === 'github' ? candidate?.installed.id ?? '' : '');
    setGithubAsset(mod.filename);
  };

  const applyRule = async (candidate: UpdateCandidate, rule: UpdateRule) => {
    setRuleError(null);
    try {
      await api('/api/update-rules', {
        method: 'PUT',
        body: JSON.stringify({key: candidate.key, rule}),
      });
      setRules((current) => ({...current, [candidate.key]: rule}));
      await reload();
    } catch (error: unknown) {
      setRuleError(error instanceof Error ? error.message : String(error));
    }
  };

  const clearRule = async (candidate: UpdateCandidate) => {
    setRuleError(null);
    try {
      await api('/api/update-rules?key=' + encodeURIComponent(candidate.key), {
        method: 'DELETE',
      });
      setRules((current) => {
        const next = {...current};
        delete next[candidate.key];
        return next;
      });
      await reload();
    } catch (error: unknown) {
      setRuleError(error instanceof Error ? error.message : String(error));
    }
  };

  const manageMod = async (
    action: 'mark_unmanaged' | 'assign_github' | 'forget_missing',
    path: string,
    extra?: {repository?: string; tag?: string; asset?: string},
  ) => {
    setManagementBusy(true);
    setManagementError(null);
    setManagementMessage(null);
    try {
      const result = await api<ModManagementResult>('/api/mod-management', {
        method: 'POST',
        body: JSON.stringify({action, path, ...extra}),
      });
      setManagementMessage(result.message);
      setSelectedMod(null);
      await reload({silent: true});
    } catch (error: unknown) {
      setManagementError(error instanceof Error ? error.message : String(error));
    } finally {
      setManagementBusy(false);
    }
  };

  const refreshModMetadata = async (path: string) => {
    setManagementBusy(true);
    setManagementError(null);
    setManagementMessage(null);
    try {
      await api<{status: string}>('/api/mod-metadata/refresh', {
        method: 'POST',
        body: JSON.stringify({path}),
      });
      setManagementMessage('Metadata refresh started. You can close or reload this page; it will continue on the server.');
      await reload({silent: true});
    } catch (error: unknown) {
      setManagementError(error instanceof Error ? error.message : String(error));
    } finally {
      setManagementBusy(false);
    }
  };

  const findLiveMod = (finding: DiagnosticFinding) =>
    finding.path
      ? state.mods.find(
          (mod) => normalizePath(mod.path) === normalizePath(finding.path),
        )
      : undefined;

  return (
    <>
      <PageHeader
        eyebrow="Inventory"
        title="Mods"
        description="Installed server/common and AutoModpack client-only artifacts."
        action={
          <div className="flex items-center gap-2">
            {blockers.length ? <Pill tone="bad">{blockers.length} blocking</Pill> : null}
            <Pill tone={connectionStatus === 'connected' ? 'good' : 'warn'}>
              {state.mods.length} JARs
            </Pill>
          </div>
        }
      />

      {managementError ? (
        <div className="alert alert-error mb-4 rounded-box py-3 text-sm">{managementError}</div>
      ) : null}
      {managementMessage ? (
        <div className="alert alert-info mb-4 rounded-box py-3 text-sm">{managementMessage}</div>
      ) : null}

      {blockers.length ? (
        <section className="panel mb-4 overflow-hidden">
          <div className="panel-header">
            <div>
              <div className="section-label">Needs attention</div>
              <h2 className="mt-0.5 text-sm font-semibold">
                Resolve blocking catalog issues
              </h2>
            </div>
            <Pill tone="bad">{blockers.length}</Pill>
          </div>
          <div className="divide-y divide-base-300">
            {blockers.map((finding, index) => {
              const liveMod = findLiveMod(finding);
              const canForget =
                finding.code === 'managed_artifact_missing' && !!finding.path;
              return (
                <div
                  className="flex flex-col gap-3 px-4 py-3 sm:flex-row sm:items-center"
                  key={finding.code + ':' + (finding.path ?? finding.mod ?? index)}
                >
                  <Wrench size={14} className="shrink-0 text-error" />
                  <div className="min-w-0 flex-1">
                    <div className="text-sm font-medium">{finding.mod || finding.code}</div>
                    <div className="mt-0.5 text-xs text-base-content/50">
                      {finding.message}
                    </div>
                    {finding.path ? (
                      <div className="mono mt-1 text-[0.68rem] text-base-content/35">
                        {finding.path}
                      </div>
                    ) : null}
                  </div>
                  <div className="flex shrink-0 flex-wrap gap-2">
                    {liveMod ? (
                      <button
                        className="btn btn-xs btn-primary"
                        type="button"
                        onClick={() => openMod(liveMod)}
                      >
                        Fix mod
                      </button>
                    ) : null}
                    {canForget ? (
                      <button
                        className="btn btn-xs btn-ghost"
                        type="button"
                        disabled={managementBusy}
                        onClick={() =>
                          void manageMod('forget_missing', finding.path ?? '')
                        }
                      >
                        <Trash2 size={12} /> Forget missing entry
                      </button>
                    ) : null}
                    {!liveMod && !canForget && finding.mod ? (
                      <button
                        className="btn btn-xs btn-ghost"
                        type="button"
                        onClick={() => {
                          setQ(finding.mod ?? '');
                          setAttentionOnly(false);
                          setPage(1);
                        }}
                      >
                        Find related mods
                      </button>
                    ) : null}
                  </div>
                </div>
              );
            })}
          </div>
        </section>
      ) : null}

      <div className="mb-3 grid gap-2 md:grid-cols-[minmax(220px,1fr)_repeat(4,minmax(125px,auto))_auto_auto]">
        <label className="input input-sm flex w-full items-center gap-2">
          <Search size={14} className="text-base-content/35" />
          <input
            className="grow"
            value={q}
            onChange={(event) => updateFilter(setQ, event.target.value)}
            placeholder="Search mods…"
            aria-label="Search mods"
          />
        </label>
        <select
          className="select select-sm"
          value={updateStatus}
          onChange={(event) => updateFilter(setUpdateStatus, event.target.value)}
        >
          <option value="all">All update states</option>
          <option value="available">Updates available</option>
          <option value="blocked">Blocked</option>
          <option value="ignored">Ignored</option>
          <option value="up_to_date">Up to date</option>
        </select>
        <select
          className="select select-sm"
          value={deployment}
          onChange={(event) => updateFilter(setDeployment, event.target.value)}
        >
          <option value="all">All placements</option>
          <option value="server">Server/common</option>
          <option value="client">Client-only</option>
        </select>
        <select
          className="select select-sm"
          value={management}
          onChange={(event) => updateFilter(setManagement, event.target.value)}
        >
          <option value="all">All management</option>
          <option value="managed">Managed</option>
          <option value="unmanaged">Unmanaged</option>
          <option value="unresolved">Unresolved</option>
          <option value="external">External change</option>
        </select>
        <select
          className="select select-sm"
          value={provider}
          onChange={(event) => updateFilter(setProvider, event.target.value)}
        >
          <option value="all">All providers</option>
          {providers.map((item) => (
            <option value={item} key={item}>
              {item}
            </option>
          ))}
        </select>
        <button
          type="button"
          className={'btn btn-sm ' + (attentionOnly ? 'btn-error' : 'btn-ghost')}
          onClick={() => {
            setAttentionOnly((value) => !value);
            setPage(1);
          }}
        >
          <Wrench size={14} /> Needs attention
        </button>
        <button
          type="button"
          className="btn btn-sm btn-ghost"
          onClick={() => {
            setQ('');
            setUpdateStatus('all');
            setDeployment('all');
            setManagement('all');
            setProvider('all');
            setAttentionOnly(false);
            setPage(1);
          }}
        >
          <X size={14} /> Reset
        </button>
      </div>

      <section className="panel overflow-hidden">
        <div className="flex items-center justify-between border-b border-base-300 px-4 py-2 text-xs text-base-content/45">
          <span>{filtered.length} matching JARs</span>
          <span>
            Page {safePage} / {totalPages}
          </span>
        </div>
        <div className="overflow-x-auto">
          <table className="table table-sm">
            <thead>
              <tr>
                <th>Mod</th>
                <th>Installed</th>
                <th>Latest</th>
                <th>Status</th>
                <th>Side</th>
                <th>Source</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((mod) => {
                const candidate = candidatesByKey.get(mod.id);
                const needsAttention =
                  blockerPaths.has(normalizePath(mod.path)) ||
                  mod.management === 'unresolved' ||
                  mod.management === 'external';
                return (
                  <tr
                    key={mod.id + ':' + mod.path}
                    className="cursor-pointer border-base-300 hover:bg-base-300/25"
                    onClick={() => openMod(mod)}
                  >
                    <td>
                      <div className="flex items-center gap-2">
                        {candidate?.icon_url ? (
                          <img
                            src={candidate.icon_url}
                            alt=""
                            className="size-7 rounded-md border border-base-300 object-cover"
                          />
                        ) : null}
                        <div>
                          <div className="flex items-center gap-2 font-medium">
                            {mod.name}
                            {needsAttention ? <Wrench size={12} className="text-error" /> : null}
                          </div>
                          <div className="mono mt-0.5 text-base-content/35">{mod.filename}</div>
                        </div>
                      </div>
                    </td>
                    <td className="whitespace-nowrap">
                      {mod.installed_version || candidate?.installed.number || '—'}
                    </td>
                    <td className="whitespace-nowrap">
                      {candidate?.target?.number ?? candidate?.installed.number ?? '—'}
                    </td>
                    <td>
                      {candidate ? (
                        <div className="flex flex-wrap gap-1">
                          <Pill tone={updateTone(candidate)}>
                            {candidate.classification.replaceAll('_', ' ')}
                          </Pill>
                          {candidate.metadata_stale ? <Pill tone="warn">stale</Pill> : null}
                        </div>
                      ) : (
                        <Pill tone={managementTone(mod.management)}>{mod.management}</Pill>
                      )}
                    </td>
                    <td>
                      <Pill tone={mod.side === 'client' ? 'blue' : 'neutral'}>{mod.side}</Pill>
                    </td>
                    <td>
                      {candidate?.project_url || mod.project_url ? (
                        <a
                          className="link link-hover text-sm"
                          href={candidate?.project_url || mod.project_url}
                          target="_blank"
                          rel="noreferrer"
                          onClick={(event) => event.stopPropagation()}
                        >
                          {mod.provider ?? candidate?.provider ?? 'unknown'}
                        </a>
                      ) : (
                        <span className="text-base-content/45">
                          {mod.provider ?? 'unknown'}
                        </span>
                      )}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
        <div className="flex items-center justify-between border-t border-base-300 px-3 py-2">
          <button
            className="btn btn-xs btn-ghost"
            type="button"
            disabled={safePage <= 1}
            onClick={() => setPage(safePage - 1)}
          >
            Previous
          </button>
          <button
            className="btn btn-xs btn-ghost"
            type="button"
            disabled={safePage >= totalPages}
            onClick={() => setPage(safePage + 1)}
          >
            Next
          </button>
        </div>
      </section>

      {selectedMod ? (
        <div
          className="modal modal-open"
          role="dialog"
          aria-modal="true"
          aria-label={'Details for ' + selectedMod.name}
        >
          <div className="modal-box max-h-[90vh] max-w-3xl overflow-y-auto max-sm:h-full max-sm:max-h-none max-sm:w-full max-sm:rounded-none">
            <div className="flex items-start gap-3 border-b border-base-300 pb-4">
              {selectedCandidate?.icon_url ? (
                <img
                  src={selectedCandidate.icon_url}
                  alt=""
                  className="size-12 rounded-lg border border-base-300 object-cover"
                />
              ) : (
                <div className="grid size-12 place-items-center rounded-lg border border-base-300 bg-base-200 text-lg font-semibold text-base-content/50">
                  {selectedMod.name.slice(0, 1).toUpperCase()}
                </div>
              )}
              <div className="min-w-0 flex-1">
                <h2 className="text-lg font-semibold">{selectedMod.name}</h2>
                <div className="mt-1 flex flex-wrap items-center gap-2">
                  <Pill tone={managementTone(selectedMod.management)}>
                    {selectedMod.management}
                  </Pill>
                  {selectedCandidate ? (
                    <Pill tone={updateTone(selectedCandidate)}>
                      {selectedCandidate.classification.replaceAll('_', ' ')}
                    </Pill>
                  ) : null}
                  {selectedCandidate?.metadata_stale ? (
                    <Pill tone="warn">stale metadata</Pill>
                  ) : null}
                </div>
              </div>
              <button
                className="btn btn-sm btn-ghost btn-square"
                type="button"
                onClick={() => setSelectedMod(null)}
                aria-label="Close"
              >
                <X size={16} />
              </button>
            </div>

            {ruleError ? (
              <div className="alert alert-error mt-4 py-2 text-xs">{ruleError}</div>
            ) : null}
            {managementError ? (
              <div className="alert alert-error mt-4 py-2 text-xs">{managementError}</div>
            ) : null}
            {managementMessage ? (
              <div className="alert alert-info mt-4 py-2 text-xs">{managementMessage}</div>
            ) : null}

            <section className="panel mt-4">
              <div className="panel-header">
                <div>
                  <div className="section-label">Management</div>
                  <h3 className="mt-0.5 text-sm font-semibold">Source &amp; metadata</h3>
                </div>
              </div>
              <div className="space-y-4 p-4">
                <div className="flex flex-wrap gap-2">
                  <button
                    className="btn btn-sm btn-primary"
                    type="button"
                    disabled={managementBusy || selectedMod.management === 'unmanaged'}
                    onClick={() => void refreshModMetadata(selectedMod.path)}
                  >
                    <RefreshCw
                      size={14}
                      className={
                        state.status.refresh?.refreshing ? 'animate-spin' : ''
                      }
                    />
                    Refresh this mod
                  </button>
                  <button
                    className="btn btn-sm btn-ghost"
                    type="button"
                    disabled={managementBusy}
                    onClick={() =>
                      void manageMod('mark_unmanaged', selectedMod.path)
                    }
                  >
                    <ShieldOff size={14} /> Mark unmanaged
                  </button>
                </div>
                <p className="text-xs text-base-content/45">
                  Refresh retries exact provider metadata for this mod only. Mark unmanaged
                  keeps the installed JAR untouched and excludes it from update planning.
                </p>

                <div className="border-t border-base-300 pt-4">
                  <div className="mb-2 flex items-center gap-2">
                    <Github size={14} />
                    <span className="text-xs font-semibold">Assign verified GitHub release</span>
                  </div>
                  <div className="grid gap-2 sm:grid-cols-2">
                    <label className="form-control">
                      <span className="mb-1 text-xs text-base-content/45">Repository</span>
                      <input
                        className="input input-sm input-bordered"
                        value={githubRepository}
                        onChange={(event) => setGithubRepository(event.target.value)}
                        placeholder="owner/repository"
                      />
                    </label>
                    <label className="form-control">
                      <span className="mb-1 text-xs text-base-content/45">Installed release tag</span>
                      <input
                        className="input input-sm input-bordered"
                        value={githubTag}
                        onChange={(event) => setGithubTag(event.target.value)}
                        placeholder="v1.2.3"
                      />
                    </label>
                    <label className="form-control sm:col-span-2">
                      <span className="mb-1 text-xs text-base-content/45">Release asset</span>
                      <input
                        className="input input-sm input-bordered"
                        value={githubAsset}
                        onChange={(event) => setGithubAsset(event.target.value)}
                        placeholder={selectedMod.filename}
                      />
                    </label>
                  </div>
                  <div className="mt-2 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                    <p className="text-xs text-base-content/40">
                      FPBPack hashes the live JAR and accepts this source only if its SHA-256
                      matches the selected GitHub release asset.
                    </p>
                    <button
                      className="btn btn-sm btn-ghost"
                      type="button"
                      disabled={
                        managementBusy ||
                        !githubRepository.trim() ||
                        !githubTag.trim()
                      }
                      onClick={() =>
                        void manageMod('assign_github', selectedMod.path, {
                          repository: githubRepository,
                          tag: githubTag,
                          asset: githubAsset,
                        })
                      }
                    >
                      <Github size={14} /> Verify &amp; assign
                    </button>
                  </div>
                </div>
              </div>
            </section>

            <div className="grid gap-4 py-4 sm:grid-cols-2">
              <div className="panel">
                <div className="panel-header">
                  <span className="section-label">Version</span>
                </div>
                <dl className="divide-y divide-base-300 text-sm">
                  <div className="flex justify-between gap-4 px-4 py-3">
                    <dt className="muted">Installed</dt>
                    <dd>
                      {selectedMod.installed_version ||
                        selectedCandidate?.installed.number ||
                        '—'}
                    </dd>
                  </div>
                  <div className="flex justify-between gap-4 px-4 py-3">
                    <dt className="muted">Latest valid</dt>
                    <dd>
                      {selectedCandidate?.target?.number ??
                        selectedCandidate?.installed.number ??
                        '—'}
                    </dd>
                  </div>
                  <div className="flex justify-between gap-4 px-4 py-3">
                    <dt className="muted">Side</dt>
                    <dd>{selectedMod.side}</dd>
                  </div>
                </dl>
              </div>

              <div className="panel">
                <div className="panel-header">
                  <span className="section-label">Provider</span>
                </div>
                <div className="p-4 text-sm">
                  <div>
                    {selectedMod.provider ?? 'unknown'}
                    {selectedMod.project_id ? ' · ' + selectedMod.project_id : ''}
                  </div>
                  {selectedCandidate?.project_url || selectedMod.project_url ? (
                    <a
                      className="btn btn-sm btn-ghost mt-3"
                      href={selectedCandidate?.project_url || selectedMod.project_url}
                      target="_blank"
                      rel="noreferrer"
                    >
                      Open project page <ExternalLink size={13} />
                    </a>
                  ) : null}
                  {selectedCandidate?.target?.manual_download &&
                  selectedCandidate.target.manual_url ? (
                    <a
                      className="btn btn-sm btn-warning btn-outline mt-2"
                      href={selectedCandidate.target.manual_url}
                      target="_blank"
                      rel="noreferrer"
                    >
                      Manual download <ExternalLink size={13} />
                    </a>
                  ) : null}
                </div>
              </div>
            </div>

            {selectedCandidate?.refresh_error ? (
              <div className="alert alert-warning mb-4 py-2 text-xs">
                Latest metadata refresh failed. The information below is the last successful
                provider metadata and was intentionally preserved.
              </div>
            ) : null}

            {selectedCandidate?.reasons?.length ? (
              <section className="panel mb-4">
                <div className="panel-header">
                  <span className="section-label">Decision</span>
                </div>
                <div className="space-y-2 p-4 text-xs text-base-content/65">
                  {selectedCandidate.reasons.map((reason) => (
                    <p key={reason.code}>{reason.message}</p>
                  ))}
                </div>
              </section>
            ) : null}

            {selectedCandidate?.dependencies?.length ||
            selectedCandidate?.required_by?.length ? (
              <section className="panel mb-4">
                <div className="panel-header">
                  <span className="section-label">Dependencies</span>
                </div>
                <div className="grid gap-4 p-4 sm:grid-cols-2">
                  <div>
                    <div className="text-xs font-medium">Requires</div>
                    <div className="mt-2 space-y-1 text-xs text-base-content/55">
                      {selectedCandidate.dependencies?.filter(
                        (dependency) => dependency.type === 'required',
                      ).length ? (
                        selectedCandidate.dependencies
                          ?.filter((dependency) => dependency.type === 'required')
                          .map((dependency) => (
                            <div
                              key={
                                (dependency.project_id ?? '') +
                                ':' +
                                (dependency.version_id ?? '')
                              }
                            >
                              {dependency.name || dependency.project_id || 'Unknown'} ·{' '}
                              {dependency.action}
                            </div>
                          ))
                      ) : (
                        <span>None reported</span>
                      )}
                    </div>
                  </div>
                  <div>
                    <div className="text-xs font-medium">Required by</div>
                    <div className="mt-2 space-y-1 text-xs text-base-content/55">
                      {selectedCandidate.required_by?.length ? (
                        selectedCandidate.required_by.map((name) => (
                          <div key={name}>{name}</div>
                        ))
                      ) : (
                        <span>None reported</span>
                      )}
                    </div>
                  </div>
                </div>
              </section>
            ) : null}

            {selectedCandidate?.changelogs?.length ? (
              <section className="panel mb-4">
                <div className="panel-header">
                  <div>
                    <div className="section-label">What changed</div>
                    <h3 className="mt-0.5 text-sm font-semibold">
                      {selectedCandidate.changelogs.length} release
                      {selectedCandidate.changelogs.length === 1 ? '' : 's'}
                    </h3>
                  </div>
                </div>
                <div className="divide-y divide-base-300">
                  {selectedCandidate.changelogs.map((entry) => (
                    <div className="p-4" key={entry.id}>
                      <div className="flex flex-wrap items-baseline gap-2">
                        <span className="text-xs font-semibold">
                          {entry.number || entry.name || entry.id}
                        </span>
                        {entry.published_at ? (
                          <span className="text-[0.68rem] text-base-content/35">
                            {formatDate(entry.published_at)}
                          </span>
                        ) : null}
                      </div>
                      <div className="mt-1 whitespace-pre-wrap text-xs leading-5 text-base-content/65">
                        {entry.body || 'No changelog was provided for this release.'}
                      </div>
                    </div>
                  ))}
                </div>
              </section>
            ) : null}

            {selectedCandidate ? (
              <section className="panel mb-4">
                <div className="panel-header">
                  <span className="section-label">Update preference</span>
                </div>
                <div className="flex flex-wrap gap-2 p-4">
                  <button
                    className="btn btn-xs btn-ghost"
                    type="button"
                    disabled={!selectedCandidate.installed.id}
                    onClick={() =>
                      void applyRule(selectedCandidate, {
                        ...selectedRule,
                        pin_version: selectedCandidate.installed.id,
                        ignore_mod: false,
                        review_after: undefined,
                      })
                    }
                  >
                    Pin current
                  </button>
                  {selectedCandidate.target?.id ? (
                    <button
                      className="btn btn-xs btn-ghost"
                      type="button"
                      onClick={() =>
                        void applyRule(selectedCandidate, {
                          ...selectedRule,
                          ignored_versions: Array.from(
                            new Set([
                              ...(selectedRule?.ignored_versions ?? []),
                              selectedCandidate.target?.id ?? '',
                            ]),
                          ).filter(Boolean),
                        })
                      }
                    >
                      Ignore this version
                    </button>
                  ) : null}
                  <button
                    className="btn btn-xs btn-ghost"
                    type="button"
                    onClick={() =>
                      void applyRule(selectedCandidate, {
                        ...selectedRule,
                        ignore_mod: true,
                      })
                    }
                  >
                    Ignore mod
                  </button>
                  <button
                    className="btn btn-xs btn-ghost"
                    type="button"
                    onClick={() =>
                      void applyRule(selectedCandidate, {
                        ...selectedRule,
                        review_after: new Date(
                          Date.now() + 7 * 24 * 60 * 60 * 1000,
                        ).toISOString(),
                      })
                    }
                  >
                    Review in 7 days
                  </button>
                  {selectedRule ? (
                    <button
                      className="btn btn-xs btn-ghost"
                      type="button"
                      onClick={() => void clearRule(selectedCandidate)}
                    >
                      Clear rule
                    </button>
                  ) : null}
                </div>
              </section>
            ) : null}

            <section className="panel">
              <div className="panel-header">
                <span className="section-label">Advanced</span>
              </div>
              <dl className="grid gap-2 p-4 text-xs sm:grid-cols-[120px_minmax(0,1fr)]">
                <dt className="muted">Filename</dt>
                <dd className="mono break-all">{selectedMod.filename}</dd>
                <dt className="muted">Path</dt>
                <dd className="mono break-all">{selectedMod.path}</dd>
                <dt className="muted">Project ID</dt>
                <dd className="mono break-all">{selectedMod.project_id ?? '—'}</dd>
                <dt className="muted">SHA-512</dt>
                <dd className="mono break-all">{selectedMod.sha512 ?? '—'}</dd>
              </dl>
            </section>
          </div>
          <button
            className="modal-backdrop"
            type="button"
            onClick={() => setSelectedMod(null)}
            aria-label="Close details"
          >
            close
          </button>
        </div>
      ) : null}
    </>
  );
}
