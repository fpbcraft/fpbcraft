'use client';

import {useEffect, useMemo, useState} from 'react';
import {ExternalLink, Search, X} from 'lucide-react';
import {PageHeader, Pill, formatDate} from '@/components/ui';
import {useManagement} from '@/components/management-provider';
import {api} from '@/lib/api';
import type {ManagementMod, UpdateCandidate, UpdateRule} from '@/lib/management';

const PAGE_SIZE = 50;

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
  const [page, setPage] = useState(1);
  const [selectedMod, setSelectedMod] = useState<ManagementMod | null>(null);
  const [rules, setRules] = useState<Record<string, UpdateRule>>({});
  const [ruleError, setRuleError] = useState<string | null>(null);

  const candidatesByKey = useMemo(
    () => new Map(state.updates.candidates.map((candidate) => [candidate.key, candidate])),
    [state.updates.candidates],
  );

  useEffect(() => {
    api<{rules: Record<string, UpdateRule>}>('/api/update-rules')
      .then((response) => setRules(response.rules))
      .catch(() => undefined);
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
      const text = [mod.name, mod.filename, mod.installed_version ?? '', mod.provider ?? '', mod.project_id ?? '']
        .join(' ')
        .toLowerCase();
      const candidate = candidatesByKey.get(mod.id);
      const statusMatches =
        updateStatus === 'all' ||
        (updateStatus === 'available' && !!candidate?.target && candidate.classification !== 'ignored') ||
        (updateStatus === 'ignored' && candidate?.classification === 'ignored') ||
        (updateStatus === 'blocked' && candidate?.classification === 'blocked') ||
        (updateStatus === 'up_to_date' && candidate?.classification === 'up_to_date');

      return (
        (!needle || text.includes(needle)) &&
        (deployment === 'all' || mod.deployment === deployment) &&
        (management === 'all' || mod.management === management) &&
        (provider === 'all' || mod.provider === provider) &&
        statusMatches
      );
    });
  }, [state.mods, candidatesByKey, q, deployment, management, provider, updateStatus]);

  const totalPages = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE));
  const safePage = Math.min(page, totalPages);
  const rows = filtered.slice((safePage - 1) * PAGE_SIZE, safePage * PAGE_SIZE);

  const updateFilter = (setter: (value: string) => void, value: string) => {
    setter(value);
    setPage(1);
  };

  const selectedCandidate = selectedMod ? candidatesByKey.get(selectedMod.id) : undefined;
  const selectedRule = selectedCandidate ? rules[selectedCandidate.key] : undefined;

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
      await api('/api/update-rules?key=' + encodeURIComponent(candidate.key), {method: 'DELETE'});
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

  return (
    <>
      <PageHeader
        eyebrow="Inventory"
        title="Mods"
        description="Installed server/common and AutoModpack client-only artifacts."
        action={
          <Pill tone={connectionStatus === 'connected' ? 'good' : 'warn'}>
            {state.mods.length} JARs
          </Pill>
        }
      />

      <div className="mb-3 grid gap-2 md:grid-cols-[minmax(220px,1fr)_repeat(4,minmax(125px,auto))_auto]">
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
        <select className="select select-sm" value={updateStatus} onChange={(event) => updateFilter(setUpdateStatus, event.target.value)}>
          <option value="all">All update states</option>
          <option value="available">Updates available</option>
          <option value="blocked">Blocked</option>
          <option value="ignored">Ignored</option>
          <option value="up_to_date">Up to date</option>
        </select>
        <select className="select select-sm" value={deployment} onChange={(event) => updateFilter(setDeployment, event.target.value)}>
          <option value="all">All placements</option>
          <option value="server">Server/common</option>
          <option value="client">Client-only</option>
        </select>
        <select className="select select-sm" value={management} onChange={(event) => updateFilter(setManagement, event.target.value)}>
          <option value="all">All management</option>
          <option value="managed">Managed</option>
          <option value="unmanaged">Unmanaged</option>
          <option value="unresolved">Unresolved</option>
          <option value="external">External change</option>
        </select>
        <select className="select select-sm" value={provider} onChange={(event) => updateFilter(setProvider, event.target.value)}>
          <option value="all">All providers</option>
          {providers.map((item) => <option value={item} key={item}>{item}</option>)}
        </select>
        <button
          type="button"
          className="btn btn-sm btn-ghost"
          onClick={() => {
            setQ('');
            setUpdateStatus('all');
            setDeployment('all');
            setManagement('all');
            setProvider('all');
            setPage(1);
          }}
        >
          <X size={14} /> Reset
        </button>
      </div>

      <section className="panel overflow-hidden">
        <div className="flex items-center justify-between border-b border-base-300 px-4 py-2 text-xs text-base-content/45">
          <span>{filtered.length} matching JARs</span>
          <span>Page {safePage} / {totalPages}</span>
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
                return (
                  <tr
                    key={mod.id + ':' + mod.path}
                    className="cursor-pointer border-base-300 hover:bg-base-300/25"
                    onClick={() => setSelectedMod(mod)}
                  >
                    <td>
                      <div className="flex items-center gap-2">
                        {candidate?.icon_url ? (
                          <img src={candidate.icon_url} alt="" className="size-7 rounded-md border border-base-300 object-cover" />
                        ) : null}
                        <div>
                          <div className="font-medium">{mod.name}</div>
                          <div className="mono mt-0.5 text-base-content/35">{mod.filename}</div>
                        </div>
                      </div>
                    </td>
                    <td className="whitespace-nowrap">{mod.installed_version || candidate?.installed.number || '—'}</td>
                    <td className="whitespace-nowrap">{candidate?.target?.number ?? candidate?.installed.number ?? '—'}</td>
                    <td>
                      {candidate ? (
                        <Pill tone={updateTone(candidate)}>{candidate.classification.replaceAll('_', ' ')}</Pill>
                      ) : (
                        <Pill tone={managementTone(mod.management)}>{mod.management}</Pill>
                      )}
                    </td>
                    <td><Pill tone={mod.side === 'client' ? 'blue' : 'neutral'}>{mod.side}</Pill></td>
                    <td>
                      {(candidate?.project_url || mod.project_url) ? (
                        <a
                          className="link link-hover text-sm"
                          href={candidate?.project_url || mod.project_url}
                          target="_blank"
                          rel="noreferrer"
                          onClick={(event) => event.stopPropagation()}
                        >
                          {mod.provider ?? 'unknown'}
                        </a>
                      ) : (
                        <span className="text-base-content/45">{mod.provider ?? 'unknown'}</span>
                      )}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
        <div className="flex items-center justify-between border-t border-base-300 px-3 py-2">
          <button className="btn btn-xs btn-ghost" type="button" disabled={safePage <= 1} onClick={() => setPage(safePage - 1)}>Previous</button>
          <button className="btn btn-xs btn-ghost" type="button" disabled={safePage >= totalPages} onClick={() => setPage(safePage + 1)}>Next</button>
        </div>
      </section>

      {selectedMod ? (
        <div className="modal modal-open" role="dialog" aria-modal="true" aria-label={'Details for ' + selectedMod.name}>
          <div className="modal-box max-h-[90vh] max-w-3xl overflow-y-auto max-sm:h-full max-sm:max-h-none max-sm:w-full max-sm:rounded-none">
            <div className="flex items-start gap-3 border-b border-base-300 pb-4">
              {selectedCandidate?.icon_url ? (
                <img src={selectedCandidate.icon_url} alt="" className="size-12 rounded-lg border border-base-300 object-cover" />
              ) : (
                <div className="grid size-12 place-items-center rounded-lg border border-base-300 bg-base-200 text-lg font-semibold text-base-content/50">
                  {selectedMod.name.slice(0, 1).toUpperCase()}
                </div>
              )}
              <div className="min-w-0 flex-1">
                <h2 className="text-lg font-semibold">{selectedMod.name}</h2>
                <div className="mt-1 flex flex-wrap items-center gap-2">
                  <Pill tone={managementTone(selectedMod.management)}>{selectedMod.management}</Pill>
                  {selectedCandidate ? <Pill tone={updateTone(selectedCandidate)}>{selectedCandidate.classification.replaceAll('_', ' ')}</Pill> : null}
                </div>
              </div>
              <button className="btn btn-sm btn-ghost btn-square" type="button" onClick={() => setSelectedMod(null)} aria-label="Close">
                <X size={16} />
              </button>
            </div>

            {ruleError ? <div className="alert alert-error mt-4 py-2 text-xs">{ruleError}</div> : null}

            <div className="grid gap-4 py-4 sm:grid-cols-2">
              <div className="panel">
                <div className="panel-header"><span className="section-label">Version</span></div>
                <dl className="divide-y divide-base-300 text-sm">
                  <div className="flex justify-between gap-4 px-4 py-3"><dt className="muted">Installed</dt><dd>{selectedMod.installed_version || selectedCandidate?.installed.number || '—'}</dd></div>
                  <div className="flex justify-between gap-4 px-4 py-3"><dt className="muted">Latest valid</dt><dd>{selectedCandidate?.target?.number ?? selectedCandidate?.installed.number ?? '—'}</dd></div>
                  <div className="flex justify-between gap-4 px-4 py-3"><dt className="muted">Side</dt><dd>{selectedMod.side}</dd></div>
                </dl>
              </div>

              <div className="panel">
                <div className="panel-header"><span className="section-label">Provider</span></div>
                <div className="p-4 text-sm">
                  <div>{selectedMod.provider ?? 'unknown'}{selectedMod.project_id ? ' · ' + selectedMod.project_id : ''}</div>
                  {(selectedCandidate?.project_url || selectedMod.project_url) ? (
                    <a className="btn btn-sm btn-ghost mt-3" href={selectedCandidate?.project_url || selectedMod.project_url} target="_blank" rel="noreferrer">
                      Open project page <ExternalLink size={13} />
                    </a>
                  ) : null}
                </div>
              </div>
            </div>

            {selectedCandidate?.reasons?.length ? (
              <section className="panel mb-4">
                <div className="panel-header"><span className="section-label">Decision</span></div>
                <div className="space-y-2 p-4 text-xs text-base-content/65">
                  {selectedCandidate.reasons.map((reason) => <p key={reason.code}>{reason.message}</p>)}
                </div>
              </section>
            ) : null}

            {selectedCandidate?.dependencies?.length || selectedCandidate?.required_by?.length ? (
              <section className="panel mb-4">
                <div className="panel-header"><span className="section-label">Dependencies</span></div>
                <div className="grid gap-4 p-4 sm:grid-cols-2">
                  <div>
                    <div className="text-xs font-medium">Requires</div>
                    <div className="mt-2 space-y-1 text-xs text-base-content/55">
                      {selectedCandidate.dependencies?.filter((dependency) => dependency.type === 'required').length ? (
                        selectedCandidate.dependencies
                          ?.filter((dependency) => dependency.type === 'required')
                          .map((dependency) => (
                            <div key={(dependency.project_id ?? '') + ':' + (dependency.version_id ?? '')}>
                              {dependency.name || dependency.project_id || 'Unknown'} · {dependency.action}
                            </div>
                          ))
                      ) : <span>None reported</span>}
                    </div>
                  </div>
                  <div>
                    <div className="text-xs font-medium">Required by</div>
                    <div className="mt-2 space-y-1 text-xs text-base-content/55">
                      {selectedCandidate.required_by?.length
                        ? selectedCandidate.required_by.map((name) => <div key={name}>{name}</div>)
                        : <span>None reported</span>}
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
                    <h3 className="mt-0.5 text-sm font-semibold">{selectedCandidate.changelogs.length} release{selectedCandidate.changelogs.length === 1 ? '' : 's'}</h3>
                  </div>
                </div>
                <div className="divide-y divide-base-300">
                  {selectedCandidate.changelogs.map((entry) => (
                    <div className="p-4" key={entry.id}>
                      <div className="flex flex-wrap items-baseline gap-2">
                        <span className="text-xs font-semibold">{entry.number || entry.name || entry.id}</span>
                        {entry.published_at ? <span className="text-[0.68rem] text-base-content/35">{formatDate(entry.published_at)}</span> : null}
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
                <div className="panel-header"><span className="section-label">Update preference</span></div>
                <div className="flex flex-wrap gap-2 p-4">
                  <button
                    className="btn btn-xs btn-ghost"
                    type="button"
                    disabled={!selectedCandidate.installed.id}
                    onClick={() => void applyRule(selectedCandidate, {...selectedRule, pin_version: selectedCandidate.installed.id, ignore_mod: false, review_after: undefined})}
                  >
                    Pin current
                  </button>
                  {selectedCandidate.target?.id ? (
                    <button
                      className="btn btn-xs btn-ghost"
                      type="button"
                      onClick={() => void applyRule(selectedCandidate, {
                        ...selectedRule,
                        ignored_versions: Array.from(new Set([...(selectedRule?.ignored_versions ?? []), selectedCandidate.target?.id ?? ''])).filter(Boolean),
                      })}
                    >
                      Ignore this version
                    </button>
                  ) : null}
                  <button className="btn btn-xs btn-ghost" type="button" onClick={() => void applyRule(selectedCandidate, {...selectedRule, ignore_mod: true})}>
                    Ignore mod
                  </button>
                  <button className="btn btn-xs btn-ghost" type="button" onClick={() => void applyRule(selectedCandidate, {...selectedRule, review_after: new Date(Date.now() + 7 * 24 * 60 * 60 * 1000).toISOString()})}>
                    Review in 7 days
                  </button>
                  {selectedRule ? (
                    <button className="btn btn-xs btn-ghost" type="button" onClick={() => void clearRule(selectedCandidate)}>
                      Clear rule
                    </button>
                  ) : null}
                </div>
              </section>
            ) : null}

            <section className="panel">
              <div className="panel-header"><span className="section-label">Advanced</span></div>
              <dl className="grid gap-2 p-4 text-xs sm:grid-cols-[120px_minmax(0,1fr)]">
                <dt className="muted">Filename</dt><dd className="mono break-all">{selectedMod.filename}</dd>
                <dt className="muted">Path</dt><dd className="mono break-all">{selectedMod.path}</dd>
                <dt className="muted">Project ID</dt><dd className="mono break-all">{selectedMod.project_id ?? '—'}</dd>
                <dt className="muted">SHA-512</dt><dd className="mono break-all">{selectedMod.sha512 ?? '—'}</dd>
              </dl>
            </section>
          </div>
          <button className="modal-backdrop" type="button" onClick={() => setSelectedMod(null)} aria-label="Close details">close</button>
        </div>
      ) : null}
    </>
  );
}
