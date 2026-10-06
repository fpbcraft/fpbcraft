'use client';

import {useCallback, useEffect, useMemo, useState} from 'react';
import {
  AlertTriangle,
  ChevronDown,
  FileCog,
  FolderTree,
  History,
  Play,
  Plus,
  RefreshCw,
  Save,
  Send,
  ServerCog,
  Trash2,
  X,
} from 'lucide-react';
import {api} from '@/lib/api';
import {PageHeader, Pill, formatDate} from '@/components/ui';
import type {
  AutoModpackActionResult,
  AutoModpackCategory,
  AutoModpackConfig,
  AutoModpackGroup,
  AutoModpackStatus,
} from '@/lib/management';

function cloneConfig(config: AutoModpackConfig): AutoModpackConfig {
  return JSON.parse(JSON.stringify(config)) as AutoModpackConfig;
}

function listText(values: string[]) {
  return values.join(', ');
}

function parseList(value: string) {
  return [...new Set(value.split(',').map((item) => item.trim()).filter(Boolean))];
}

function bytes(value: number) {
  if (value < 1024) return value + ' B';
  if (value < 1024 * 1024) return (value / 1024).toFixed(1) + ' KiB';
  if (value < 1024 * 1024 * 1024) return (value / (1024 * 1024)).toFixed(1) + ' MiB';
  return (value / (1024 * 1024 * 1024)).toFixed(2) + ' GiB';
}

function newGroup(id: string): AutoModpackGroup {
  return {
    id,
    display_name: '',
    description: '',
    required: false,
    default_selected: false,
    requires: [],
    breaks_with: [],
    compatible_platforms: [],
    from_server: [],
    exclude: [],
    editable: [],
  };
}

export default function AutoModpackPage() {
  const [status, setStatus] = useState<AutoModpackStatus | null>(null);
  const [draft, setDraft] = useState<AutoModpackConfig | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [actionBusy, setActionBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const [identityDirty, setIdentityDirty] = useState(false);
  const [publishNotes, setPublishNotes] = useState('');
  const [advancedOpen, setAdvancedOpen] = useState(false);
  const [migration, setMigration] = useState<{oldID: string; newID: string} | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const next = await api<AutoModpackStatus>('/api/automodpack');
      setStatus(next);
      setDraft(cloneConfig(next.config));
      setIdentityDirty(false);
    } catch (value: unknown) {
      setError(value instanceof Error ? value.message : String(value));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const groupIDs = useMemo(
    () => draft?.categories.flatMap((category) => category.groups.map((group) => group.id)) ?? [],
    [draft],
  );

  const updateSettings = <K extends keyof AutoModpackConfig['settings']>(
    key: K,
    value: AutoModpackConfig['settings'][K],
  ) => {
    setDraft((current) =>
      current
        ? {...current, settings: {...current.settings, [key]: value}}
        : current,
    );
  };

  const updateCategory = (categoryIndex: number, patch: Partial<AutoModpackCategory>) => {
    setDraft((current) => {
      if (!current) return current;
      const next = cloneConfig(current);
      next.categories[categoryIndex] = {...next.categories[categoryIndex], ...patch};
      return next;
    });
  };

  const updateGroup = (
    categoryIndex: number,
    groupIndex: number,
    patch: Partial<AutoModpackGroup>,
  ) => {
    setDraft((current) => {
      if (!current) return current;
      const next = cloneConfig(current);
      next.categories[categoryIndex].groups[groupIndex] = {
        ...next.categories[categoryIndex].groups[groupIndex],
        ...patch,
      };
      return next;
    });
  };

  const save = async () => {
    if (!draft || !status?.config_sha256) return;
    if (
      identityDirty &&
      !window.confirm(
        'This changes or removes an AutoModpack category/group identity. Existing player group selections may no longer match. Save these identity changes?',
      )
    ) {
      return;
    }
    setSaving(true);
    setError(null);
    setMessage(null);
    try {
      const next = await api<AutoModpackStatus>('/api/automodpack/config', {
        method: 'PUT',
        body: JSON.stringify({
          expected_sha256: status.config_sha256,
          config: draft,
          confirm_identity_changes: identityDirty,
        }),
      });
      setStatus(next);
      setDraft(cloneConfig(next.config));
      setIdentityDirty(false);
      setMessage('AutoModpack configuration saved. Reload the config on the running server when ready.');
    } catch (value: unknown) {
      setError(value instanceof Error ? value.message : String(value));
    } finally {
      setSaving(false);
    }
  };

  const runAction = async (action: string, sequence?: number) => {
    if (
      action === 'revert_confirm' &&
      !window.confirm(
        'Publish generation ' + sequence + ' as a new rollback generation? This changes what connecting clients receive.',
      )
    ) {
      return;
    }
    setActionBusy(action + (sequence ? ':' + sequence : ''));
    setError(null);
    setMessage(null);
    try {
      const result = await api<AutoModpackActionResult>('/api/automodpack/action', {
        method: 'POST',
        body: JSON.stringify({
          action,
          sequence,
          notes:
            action === 'publish' || action === 'preview' || action === 'revert_confirm'
              ? publishNotes
              : undefined,
        }),
      });
      setMessage(result.message + ' ' + result.command);
      const next = await api<AutoModpackStatus>('/api/automodpack');
      setStatus(next);
      setDraft((current) => current ?? cloneConfig(next.config));
    } catch (value: unknown) {
      setError(value instanceof Error ? value.message : String(value));
    } finally {
      setActionBusy(null);
    }
  };

  const migrateGroup = async () => {
    if (!migration || !status?.config_sha256 || !migration.newID.trim()) return;
    setActionBusy('migrate');
    setError(null);
    try {
      const next = await api<AutoModpackStatus>('/api/automodpack/groups/migrate', {
        method: 'POST',
        body: JSON.stringify({
          expected_sha256: status.config_sha256,
          old_id: migration.oldID,
          new_id: migration.newID.trim(),
        }),
      });
      setStatus(next);
      setDraft(cloneConfig(next.config));
      setMigration(null);
      setIdentityDirty(false);
      setMessage('Group identity, references, directory and FPBPack catalog assignment migrated.');
    } catch (value: unknown) {
      setError(value instanceof Error ? value.message : String(value));
    } finally {
      setActionBusy(null);
    }
  };

  if (loading && !status) {
    return (
      <div className="grid min-h-[40vh] place-items-center">
        <span className="loading loading-spinner loading-md" />
      </div>
    );
  }

  if (!status || !draft) {
    return (
      <>
        <PageHeader eyebrow="Distribution" title="AutoModpack" description="AutoModpack integration is unavailable." />
        <div className="alert alert-error">{error ?? 'Unable to load AutoModpack status.'}</div>
      </>
    );
  }

  const errorCount = status.findings.filter((finding) => finding.level === 'error').length;
  const warningCount = status.findings.filter((finding) => finding.level === 'warning').length;

  return (
    <>
      <PageHeader
        eyebrow="Distribution"
        title="AutoModpack"
        description="Manage server.conf, optional groups, client content and publication from FPBPack."
        action={
          <div className="flex flex-wrap items-center gap-2">
            {status.pending_publish ? <Pill tone="warn">publish pending</Pill> : <Pill tone="good">published</Pill>}
            <Pill tone={status.installed ? 'good' : 'warn'}>
              {status.installed ? status.version || 'installed' : 'not detected'}
            </Pill>
            <button className="btn btn-sm btn-ghost" type="button" disabled={loading} onClick={() => void load()}>
              <RefreshCw size={14} className={loading ? 'animate-spin' : ''} /> Refresh
            </button>
            <button className="btn btn-sm btn-primary" type="button" disabled={saving || !status.config_present} onClick={() => void save()}>
              {saving ? <span className="loading loading-spinner loading-xs" /> : <Save size={14} />} Save config
            </button>
          </div>
        }
      />

      {error ? <div className="alert alert-error mb-4 py-3 text-sm">{error}</div> : null}
      {message ? <div className="alert alert-info mb-4 py-3 text-sm">{message}</div> : null}

      {!status.config_present ? (
        <div className="alert alert-warning mb-4">
          <AlertTriangle size={16} />
          AutoModpack has not created <span className="mono">automodpack/server.conf</span> yet.
        </div>
      ) : null}

      <div className="mb-4 grid gap-3 md:grid-cols-4">
        <section className="panel p-4">
          <div className="section-label">Pack</div>
          <div className="mt-1 text-lg font-semibold">{draft.name || 'Unnamed pack'}</div>
          <div className="mt-1 text-xs text-base-content/45">{status.config_path}</div>
        </section>
        <section className="panel p-4">
          <div className="section-label">Groups</div>
          <div className="mt-1 text-lg font-semibold">{groupIDs.length}</div>
          <div className="mt-1 text-xs text-base-content/45">{draft.categories.length} categories</div>
        </section>
        <section className="panel p-4">
          <div className="section-label">Diagnostics</div>
          <div className="mt-1 flex gap-2">
            <Pill tone={errorCount ? 'bad' : 'good'}>{errorCount} errors</Pill>
            <Pill tone={warningCount ? 'warn' : 'neutral'}>{warningCount} warnings</Pill>
          </div>
        </section>
        <section className="panel p-4">
          <div className="section-label">Publication</div>
          <div className="mt-1 text-sm font-semibold">
            {status.pending_publish ? 'Changes not yet published' : 'No pending FPBPack changes'}
          </div>
          <div className="mt-1 text-xs text-base-content/45">
            {status.last_changed_at ? 'Changed ' + formatDate(status.last_changed_at) : 'No tracked changes yet'}
          </div>
        </section>
      </div>

      {status.findings.length ? (
        <section className="panel mb-4 overflow-hidden">
          <div className="panel-header">
            <div>
              <div className="section-label">Diagnostics</div>
              <h2 className="mt-0.5 text-sm font-semibold">AutoModpack configuration &amp; content</h2>
            </div>
            <Pill tone={errorCount ? 'bad' : warningCount ? 'warn' : 'neutral'}>{status.findings.length}</Pill>
          </div>
          <div className="divide-y divide-base-300">
            {status.findings.map((finding, index) => (
              <div className="flex gap-3 px-4 py-3 text-sm" key={finding.code + ':' + (finding.group ?? '') + ':' + index}>
                <AlertTriangle
                  size={14}
                  className={
                    finding.level === 'error'
                      ? 'mt-0.5 shrink-0 text-error'
                      : finding.level === 'warning'
                        ? 'mt-0.5 shrink-0 text-warning'
                        : 'mt-0.5 shrink-0 text-info'
                  }
                />
                <div>
                  <div className="font-medium">
                    {finding.group ? finding.group + ' · ' : ''}
                    {finding.code.replaceAll('_', ' ')}
                  </div>
                  <div className="mt-0.5 text-xs text-base-content/55">{finding.message}</div>
                </div>
              </div>
            ))}
          </div>
        </section>
      ) : null}

      <section className="panel mb-4 overflow-hidden">
        <div className="panel-header">
          <div>
            <div className="section-label">Operations</div>
            <h2 className="mt-0.5 text-sm font-semibold">Reload, preview and publish</h2>
          </div>
          <ServerCog size={16} className="text-base-content/40" />
        </div>
        <div className="p-4">
          <div className="grid gap-2 md:grid-cols-[minmax(0,1fr)_auto_auto]">
            <input
              className="input input-sm input-bordered"
              value={publishNotes}
              onChange={(event) => setPublishNotes(event.target.value)}
              placeholder="Optional generation notes"
            />
            <button className="btn btn-sm btn-outline" disabled={!!actionBusy} onClick={() => void runAction('preview')}>
              {actionBusy === 'preview' ? <span className="loading loading-spinner loading-xs" /> : <Play size={13} />} Preview generation
            </button>
            <button className="btn btn-sm btn-primary" disabled={!!actionBusy} onClick={() => void runAction('publish')}>
              {actionBusy === 'publish' ? <span className="loading loading-spinner loading-xs" /> : <Send size={13} />} Publish
            </button>
          </div>
          <div className="mt-3 flex flex-wrap gap-2">
            <button className="btn btn-xs btn-ghost" disabled={!!actionBusy} onClick={() => void runAction('reload')}>
              <FileCog size={12} /> Reload config
            </button>
            <button className="btn btn-xs btn-ghost" disabled={!!actionBusy} onClick={() => void runAction('host_restart')}>
              <RefreshCw size={12} /> Restart AutoModpack host
            </button>
            <button className="btn btn-xs btn-ghost" disabled={!!actionBusy} onClick={() => void runAction('groups')}>
              <FolderTree size={12} /> Server group summary
            </button>
            <button className="btn btn-xs btn-ghost" disabled={!!actionBusy} onClick={() => void runAction('host_activity')}>
              Host activity
            </button>
          </div>
          <p className="mt-3 text-xs text-base-content/40">
            Commands are sent through Crafty to the running Minecraft server. AutoModpack's server console is authoritative if generation validation rejects a command.
          </p>
        </div>
      </section>

      <section className="panel mb-4 overflow-hidden">
        <div className="panel-header">
          <div>
            <div className="section-label">Generations</div>
            <h2 className="mt-0.5 text-sm font-semibold">Published AutoModpack history</h2>
          </div>
          <div className="flex items-center gap-2">
            <History size={15} className="text-base-content/40" />
            <Pill tone="neutral">{status.generations.length}</Pill>
          </div>
        </div>
        {status.generations.length ? (
          <div className="overflow-x-auto">
            <table className="table table-sm">
              <thead>
                <tr>
                  <th>Seq</th>
                  <th>Published</th>
                  <th>Changes</th>
                  <th>Notes</th>
                  <th>Token</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {status.generations.map((generation, index) => {
                  const isHead = index === 0;
                  const busyForGeneration =
                    actionBusy === 'revert_preview:' + generation.sequence ||
                    actionBusy === 'revert_confirm:' + generation.sequence;
                  return (
                    <tr key={generation.sequence}>
                      <td className="font-mono">#{generation.sequence}</td>
                      <td className="whitespace-nowrap">
                        {formatDate(generation.created_at)}
                        {isHead ? <Pill tone="good">head</Pill> : null}
                        {generation.restore_of > 0 ? (
                          <div className="mt-1 text-[0.68rem] text-base-content/40">
                            restore of #{generation.restore_of}
                          </div>
                        ) : null}
                      </td>
                      <td className="whitespace-nowrap text-xs">
                        <span className="text-success">+{generation.summary.added}</span>
                        {' · '}
                        <span>~{generation.summary.changed}</span>
                        {' · '}
                        <span className="text-error">−{generation.summary.removed}</span>
                      </td>
                      <td className="max-w-xs text-xs text-base-content/55">
                        {generation.notes || '—'}
                      </td>
                      <td className="font-mono text-[0.68rem] text-base-content/40">
                        {generation.content_token.slice(0, 12)}
                      </td>
                      <td>
                        {!isHead ? (
                          <div className="flex justify-end gap-1">
                            <button
                              className="btn btn-xs btn-ghost"
                              type="button"
                              disabled={!!actionBusy}
                              onClick={() => void runAction('revert_preview', generation.sequence)}
                            >
                              {busyForGeneration && actionBusy?.startsWith('revert_preview') ? (
                                <span className="loading loading-spinner loading-xs" />
                              ) : null}
                              Preview revert
                            </button>
                            <button
                              className="btn btn-xs btn-outline btn-warning"
                              type="button"
                              disabled={!!actionBusy}
                              onClick={() => void runAction('revert_confirm', generation.sequence)}
                            >
                              {busyForGeneration && actionBusy?.startsWith('revert_confirm') ? (
                                <span className="loading loading-spinner loading-xs" />
                              ) : null}
                              Publish rollback
                            </button>
                          </div>
                        ) : null}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        ) : (
          <div className="p-4 text-sm text-base-content/45">
            AutoModpack has not published a generation yet, or its journal is not present.
          </div>
        )}
      </section>

      <section className="panel mb-4 overflow-hidden">
        <div className="panel-header">
          <div>
            <div className="section-label">Pack identity</div>
            <h2 className="mt-0.5 text-sm font-semibold">Name &amp; general behavior</h2>
          </div>
        </div>
        <div className="grid gap-4 p-4 md:grid-cols-2">
          <label className="form-control">
            <span className="mb-1 text-xs text-base-content/45">Pack name</span>
            <input className="input input-sm input-bordered" value={draft.name} onChange={(event) => setDraft({...draft, name: event.target.value})} />
          </label>
          <label className="form-control">
            <span className="mb-1 text-xs text-base-content/45">Accepted loaders</span>
            <input
              className="input input-sm input-bordered"
              value={listText(draft.settings.accepted_loaders)}
              onChange={(event) => updateSettings('accepted_loaders', parseList(event.target.value))}
              placeholder="neoforge"
            />
          </label>
          {[
            ['modpack_host', 'Run built-in modpack host'],
            ['generate_modpack_on_start', 'Generate changed content on server start'],
            ['auto_exclude_server_side_mods', 'Auto-exclude server-side mods'],
            ['require_modpack', 'Require AutoModpack for joining'],
            ['advertise_versions_to_sync', 'Advertise Minecraft/loader versions'],
            ['self_updater', 'Enable AutoModpack self-updater'],
          ].map(([key, label]) => (
            <label className="flex items-center gap-3 rounded-box border border-base-300 px-3 py-2" key={key}>
              <input
                type="checkbox"
                className="toggle toggle-sm"
                checked={Boolean(draft.settings[key as keyof typeof draft.settings])}
                onChange={(event) =>
                  updateSettings(
                    key as keyof AutoModpackConfig['settings'],
                    event.target.checked as never,
                  )
                }
              />
              <span className="text-sm">{label}</span>
            </label>
          ))}
        </div>
      </section>

      <section className="panel mb-4 overflow-hidden">
        <div className="panel-header">
          <div>
            <div className="section-label">Group selection</div>
            <h2 className="mt-0.5 text-sm font-semibold">Categories &amp; groups</h2>
          </div>
          <button
            className="btn btn-xs btn-ghost"
            type="button"
            onClick={() =>
              setDraft({
                ...draft,
                categories: [...draft.categories, {name: 'New category', groups: []}],
              })
            }
          >
            <Plus size={12} /> Category
          </button>
        </div>
        <div className="space-y-4 p-4">
          {draft.categories.map((category, categoryIndex) => (
            <div className="rounded-box border border-base-300" key={category.name + ':' + categoryIndex}>
              <div className="flex flex-col gap-2 border-b border-base-300 bg-base-200/35 p-3 sm:flex-row sm:items-center">
                <input
                  className="input input-sm input-bordered min-w-0 flex-1"
                  value={category.name}
                  onChange={(event) => {
                    if (event.target.value !== category.name) setIdentityDirty(true);
                    updateCategory(categoryIndex, {name: event.target.value});
                  }}
                  aria-label="Category name"
                />
                <button
                  className="btn btn-xs btn-ghost"
                  type="button"
                  onClick={() => {
                    let seed = 'group';
                    let suffix = 1;
                    while (groupIDs.includes(seed)) seed = 'group-' + suffix++;
                    updateCategory(categoryIndex, {
                      groups: [...category.groups, newGroup(seed)],
                    });
                  }}
                >
                  <Plus size={12} /> Group
                </button>
                <button
                  className="btn btn-xs btn-ghost text-error"
                  type="button"
                  disabled={draft.categories.length <= 1}
                  onClick={() => {
                    setIdentityDirty(true);
                    setDraft({
                      ...draft,
                      categories: draft.categories.filter((_, index) => index !== categoryIndex),
                    });
                  }}
                >
                  <Trash2 size={12} /> Category
                </button>
              </div>

              {category.groups.length ? (
                <div className="divide-y divide-base-300">
                  {category.groups.map((group, groupIndex) => {
                    const content = status.groups.find((item) => item.id === group.id);
                    return (
                      <details className="group" key={group.id} open={group.id === 'main'}>
                        <summary className="flex cursor-pointer list-none items-center gap-3 px-4 py-3 hover:bg-base-200/30">
                          <ChevronDown size={14} className="transition-transform group-open:rotate-180" />
                          <div className="min-w-0 flex-1">
                            <div className="flex flex-wrap items-center gap-2">
                              <span className="font-medium">{group.display_name || group.id}</span>
                              <span className="mono text-xs text-base-content/35">{group.id}</span>
                              {group.required ? <Pill tone="good">required</Pill> : null}
                              {group.default_selected && !group.required ? <Pill tone="blue">default</Pill> : null}
                            </div>
                            <div className="mt-1 text-xs text-base-content/40">
                              {content
                                ? content.files + ' files · ' + content.mods + ' JARs · ' + bytes(content.bytes)
                                : 'Content directory not scanned'}
                            </div>
                          </div>
                        </summary>
                        <div className="grid gap-4 border-t border-base-300 bg-base-100 p-4 md:grid-cols-2">
                          <label className="form-control">
                            <span className="mb-1 text-xs text-base-content/45">Group ID</span>
                            <div className="join">
                              <input className="input input-sm input-bordered join-item w-full font-mono" value={group.id} readOnly />
                              <button className="btn btn-sm btn-outline join-item" type="button" onClick={() => setMigration({oldID: group.id, newID: group.id})}>
                                Migrate ID
                              </button>
                            </div>
                          </label>
                          <label className="form-control">
                            <span className="mb-1 text-xs text-base-content/45">Display name</span>
                            <input className="input input-sm input-bordered" value={group.display_name} onChange={(event) => updateGroup(categoryIndex, groupIndex, {display_name: event.target.value})} />
                          </label>
                          <label className="form-control md:col-span-2">
                            <span className="mb-1 text-xs text-base-content/45">Description</span>
                            <input className="input input-sm input-bordered" value={group.description} onChange={(event) => updateGroup(categoryIndex, groupIndex, {description: event.target.value})} />
                          </label>
                          <div className="flex flex-wrap gap-4 md:col-span-2">
                            <label className="flex items-center gap-2 text-sm">
                              <input type="checkbox" className="checkbox checkbox-sm" checked={group.required} onChange={(event) => updateGroup(categoryIndex, groupIndex, {required: event.target.checked})} />
                              Required
                            </label>
                            <label className="flex items-center gap-2 text-sm">
                              <input type="checkbox" className="checkbox checkbox-sm" checked={group.default_selected} disabled={group.required} onChange={(event) => updateGroup(categoryIndex, groupIndex, {default_selected: event.target.checked})} />
                              Default selected
                            </label>
                          </div>
                          {[
                            ['requires', 'Requires'],
                            ['breaks_with', 'Breaks with'],
                            ['compatible_platforms', 'Platforms'],
                            ['from_server', 'From server'],
                            ['exclude', 'Exclude'],
                            ['editable', 'Editable'],
                          ].map(([key, label]) => (
                            <label className="form-control" key={key}>
                              <span className="mb-1 text-xs text-base-content/45">{label}</span>
                              <input
                                className="input input-sm input-bordered font-mono"
                                value={listText(group[key as keyof AutoModpackGroup] as string[])}
                                onChange={(event) =>
                                  updateGroup(categoryIndex, groupIndex, {
                                    [key]: parseList(event.target.value),
                                  })
                                }
                                placeholder={key === 'from_server' ? 'mods/*.jar, kubejs/**' : ''}
                              />
                            </label>
                          ))}
                          <div className="flex items-center justify-between border-t border-base-300 pt-3 md:col-span-2">
                            <div className="mono text-xs text-base-content/35">
                              {content?.path ?? 'automodpack/host-modpack/' + group.id}
                            </div>
                            <button
                              className="btn btn-xs btn-ghost text-error"
                              type="button"
                              disabled={group.id === 'main'}
                              onClick={() => {
                                setIdentityDirty(true);
                                updateCategory(categoryIndex, {
                                  groups: category.groups.filter((_, index) => index !== groupIndex),
                                });
                              }}
                            >
                              <Trash2 size={12} /> Delete group
                            </button>
                          </div>
                        </div>
                      </details>
                    );
                  })}
                </div>
              ) : (
                <div className="p-4 text-sm text-base-content/45">No groups in this category.</div>
              )}
            </div>
          ))}
        </div>
      </section>

      <section className="panel mb-4 overflow-hidden">
        <button
          className="panel-header flex w-full items-center text-left"
          type="button"
          onClick={() => setAdvancedOpen((value) => !value)}
        >
          <div className="flex-1">
            <div className="section-label">Advanced configuration</div>
            <h2 className="mt-0.5 text-sm font-semibold">Hosting, security &amp; unmodded clients</h2>
          </div>
          <ChevronDown size={16} className={advancedOpen ? 'rotate-180 transition-transform' : 'transition-transform'} />
        </button>
        {advancedOpen ? (
          <div className="grid gap-4 border-t border-base-300 p-4 md:grid-cols-2">
            <label className="form-control">
              <span className="mb-1 text-xs text-base-content/45">Connection mode</span>
              <select className="select select-sm select-bordered" value={draft.settings.connection_mode} onChange={(event) => updateSettings('connection_mode', event.target.value)}>
                <option value="HOLEPUNCH">HOLEPUNCH</option>
                <option value="MAGIC">MAGIC</option>
                <option value="HTTP">HTTP</option>
              </select>
            </label>
            {[
              ['bind_address', 'Bind address', 'text'],
              ['bind_port', 'Bind port', 'number'],
              ['advertised_endpoint_host', 'Advertised endpoint host', 'text'],
              ['advertised_endpoint_port', 'Advertised endpoint port', 'number'],
              ['bandwidth_limit', 'Bandwidth limit (MiB/s/client)', 'number'],
              ['secret_lifetime', 'Secret lifetime (hours)', 'number'],
              ['export_http_directory', 'HTTP export directory', 'text'],
            ].map(([key, label, type]) => (
              <label className="form-control" key={key}>
                <span className="mb-1 text-xs text-base-content/45">{label}</span>
                <input
                  className="input input-sm input-bordered"
                  type={type}
                  value={String(draft.settings[key as keyof typeof draft.settings])}
                  onChange={(event) =>
                    updateSettings(
                      key as keyof AutoModpackConfig['settings'],
                      (type === 'number' ? Number(event.target.value) : event.target.value) as never,
                    )
                  }
                />
              </label>
            ))}
            {[
              ['disable_internal_tls', 'Disable internal TLS'],
              ['accept_proxy_protocol', 'Accept PROXY protocol'],
              ['validate_secrets', 'Validate download secrets'],
              ['export_http_include_all', 'Export all HTTP objects'],
              ['nag_unmodded_clients', 'Nag unmodded clients'],
            ].map(([key, label]) => (
              <label className="flex items-center gap-3 rounded-box border border-base-300 px-3 py-2" key={key}>
                <input
                  type="checkbox"
                  className="toggle toggle-sm"
                  checked={Boolean(draft.settings[key as keyof typeof draft.settings])}
                  onChange={(event) =>
                    updateSettings(
                      key as keyof AutoModpackConfig['settings'],
                      event.target.checked as never,
                    )
                  }
                />
                <span className="text-sm">{label}</span>
              </label>
            ))}
            {[
              ['nag_message', 'Nag message'],
              ['nag_clickable_message', 'Clickable message'],
              ['nag_clickable_link', 'Clickable link'],
            ].map(([key, label]) => (
              <label className="form-control md:col-span-2" key={key}>
                <span className="mb-1 text-xs text-base-content/45">{label}</span>
                <input
                  className="input input-sm input-bordered"
                  value={String(draft.settings[key as keyof typeof draft.settings])}
                  onChange={(event) =>
                    updateSettings(
                      key as keyof AutoModpackConfig['settings'],
                      event.target.value as never,
                    )
                  }
                />
              </label>
            ))}
          </div>
        ) : null}
      </section>

      {status.orphan_group_directories?.length ? (
        <section className="panel mb-4 p-4">
          <div className="section-label">Orphan group directories</div>
          <div className="mt-2 flex flex-wrap gap-2">
            {status.orphan_group_directories.map((group) => (
              <Pill key={group} tone="warn">{group}</Pill>
            ))}
          </div>
        </section>
      ) : null}

      {migration ? (
        <div className="modal modal-open">
          <div className="modal-box max-w-lg">
            <div className="flex items-start justify-between">
              <div>
                <div className="section-label">Identity migration</div>
                <h2 className="mt-1 text-lg font-semibold">Rename group {migration.oldID}</h2>
              </div>
              <button className="btn btn-sm btn-ghost btn-square" onClick={() => setMigration(null)}><X size={15} /></button>
            </div>
            <div className="alert alert-warning mt-4 py-3 text-xs">
              This changes a persistent AutoModpack identity. FPBPack will update requires/breaks-with references, rename the host-modpack directory, and migrate managed catalog assignments. Players' saved selection for the old ID may still need to be chosen again.
            </div>
            <label className="form-control mt-4">
              <span className="mb-1 text-xs text-base-content/45">New group ID</span>
              <input className="input input-bordered font-mono" value={migration.newID} onChange={(event) => setMigration({...migration, newID: event.target.value})} />
            </label>
            <div className="modal-action">
              <button className="btn btn-ghost" onClick={() => setMigration(null)}>Cancel</button>
              <button className="btn btn-warning" disabled={actionBusy === 'migrate' || migration.newID.trim() === migration.oldID} onClick={() => void migrateGroup()}>
                {actionBusy === 'migrate' ? <span className="loading loading-spinner loading-xs" /> : <FolderTree size={14} />}
                Migrate identity
              </button>
            </div>
          </div>
          <button className="modal-backdrop" type="button" onClick={() => setMigration(null)}>close</button>
        </div>
      ) : null}
    </>
  );
}
