'use client';

import {useCallback, useEffect, useMemo, useState} from 'react';
import {
  AlertTriangle,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  Code2,
  Edit3,
  FileCog,
  FolderTree,
  History,
  Play,
  Plus,
  RefreshCw,
  RotateCcw,
  Save,
  Search,
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
  AutoModpackGenerationDiff,
  AutoModpackGroup,
  AutoModpackPublishedFilesPage,
  AutoModpackStatus,
} from '@/lib/management';

type ConfigMode = 'form' | 'raw';

function cloneConfig(config: AutoModpackConfig): AutoModpackConfig {
  const cloned = JSON.parse(JSON.stringify(config)) as AutoModpackConfig;
  cloned.settings.accepted_loaders = Array.isArray(cloned.settings.accepted_loaders)
    ? cloned.settings.accepted_loaders
    : [];
  cloned.categories = Array.isArray(cloned.categories) ? cloned.categories : [];
  cloned.categories.forEach((category) => {
    category.groups = Array.isArray(category.groups) ? category.groups : [];
    category.groups.forEach((group) => {
      group.requires = Array.isArray(group.requires) ? group.requires : [];
      group.breaks_with = Array.isArray(group.breaks_with) ? group.breaks_with : [];
      group.compatible_platforms = Array.isArray(group.compatible_platforms)
        ? group.compatible_platforms
        : [];
      group.from_server = Array.isArray(group.from_server) ? group.from_server : [];
      group.exclude = Array.isArray(group.exclude) ? group.exclude : [];
      group.editable = Array.isArray(group.editable) ? group.editable : [];
    });
  });
  return cloned;
}

function configsEqual(left: AutoModpackConfig | null, right: AutoModpackConfig | null) {
  if (!left || !right) return left === right;
  return JSON.stringify(left) === JSON.stringify(right);
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

function StringListEditor({
  label,
  values,
  onChange,
  disabled,
  placeholder,
  mono = false,
}: {
  label: string;
  values: string[];
  onChange: (values: string[]) => void;
  disabled: boolean;
  placeholder?: string;
  mono?: boolean;
}) {
  const [input, setInput] = useState('');

  const add = () => {
    const value = input.trim();
    if (!value || values.includes(value)) return;
    onChange([...values, value]);
    setInput('');
  };

  return (
    <div className="form-control">
      <span className="mb-1 text-xs text-base-content/45">{label}</span>
      {values.length ? (
        <div className="mb-2 flex flex-wrap gap-1.5">
          {values.map((value) => (
            <span
              className={
                'inline-flex max-w-full items-center gap-1 rounded-md border border-base-300 bg-base-200/50 px-2 py-1 text-xs ' +
                (mono ? 'font-mono' : '')
              }
              key={value}
            >
              <span className="truncate">{value}</span>
              {disabled ? null : (
                <button
                  className="btn btn-ghost btn-xs h-4 min-h-0 w-4 p-0"
                  type="button"
                  aria-label={'Remove ' + value}
                  onClick={() => onChange(values.filter((item) => item !== value))}
                >
                  <X size={10} />
                </button>
              )}
            </span>
          ))}
        </div>
      ) : (
        <div className="mb-2 text-xs text-base-content/35">No entries.</div>
      )}
      {disabled ? null : (
        <div className="join w-full">
          <input
            className={'input input-sm input-bordered join-item min-w-0 flex-1 ' + (mono ? 'font-mono' : '')}
            value={input}
            placeholder={placeholder}
            onChange={(event) => setInput(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === 'Enter') {
                event.preventDefault();
                add();
              }
            }}
          />
          <button className="btn btn-sm btn-outline join-item" type="button" disabled={!input.trim()} onClick={add}>
            <Plus size={12} /> Add
          </button>
        </div>
      )}
    </div>
  );
}

function PublishedFilesBrowser({group, total}: {group: string; total: number}) {
  const pageSize = 100;
  const [active, setActive] = useState(false);
  const [page, setPage] = useState<AutoModpackPublishedFilesPage | null>(null);
  const [offset, setOffset] = useState(0);
  const [searchInput, setSearchInput] = useState('');
  const [query, setQuery] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    if (!active) return;
    setLoading(true);
    setError(null);
    try {
      const params = new URLSearchParams({
        offset: String(offset),
        limit: String(pageSize),
      });
      if (query) params.set('q', query);
      const result = await api<AutoModpackPublishedFilesPage>(
        '/api/automodpack/groups/' + encodeURIComponent(group) + '/files?' + params.toString(),
      );
      setPage(result);
    } catch (value: unknown) {
      setError(value instanceof Error ? value.message : String(value));
    } finally {
      setLoading(false);
    }
  }, [active, group, offset, query]);

  useEffect(() => {
    void load();
  }, [load]);

  if (!active) {
    return (
      <div className="rounded-box border border-dashed border-base-300 px-3 py-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <div className="text-sm font-medium">Effective published content</div>
            <div className="mt-0.5 text-xs text-base-content/40">
              {total.toLocaleString()} files in AutoModpack&apos;s current projection.
            </div>
          </div>
          <button className="btn btn-sm btn-outline" type="button" onClick={() => setActive(true)}>
            <FolderTree size={13} /> Browse files
          </button>
        </div>
      </div>
    );
  }

  const first = page && page.total ? page.offset + 1 : 0;
  const last = page ? Math.min(page.offset + page.files.length, page.total) : 0;

  return (
    <div className="rounded-box border border-base-300">
      <div className="flex flex-col gap-2 border-b border-base-300 p-3 sm:flex-row sm:items-center">
        <form
          className="join min-w-0 flex-1"
          onSubmit={(event) => {
            event.preventDefault();
            setOffset(0);
            setQuery(searchInput.trim());
          }}
        >
          <input
            className="input input-sm input-bordered join-item min-w-0 flex-1 font-mono"
            value={searchInput}
            placeholder="Filter by path…"
            onChange={(event) => setSearchInput(event.target.value)}
          />
          <button className="btn btn-sm btn-outline join-item" type="submit">
            <Search size={12} /> Filter
          </button>
        </form>
        <div className="whitespace-nowrap text-xs text-base-content/45">
          {loading ? 'Loading…' : page ? first + '–' + last + ' of ' + page.total : total + ' files'}
        </div>
      </div>
      {error ? <div className="alert alert-error m-3 py-2 text-xs">{error}</div> : null}
      {page?.files.length ? (
        <div className="max-h-80 overflow-auto">
          <table className="table table-xs">
            <thead>
              <tr>
                <th>Path</th>
                <th>Type</th>
                <th>Size</th>
                <th>Mode</th>
              </tr>
            </thead>
            <tbody>
              {page.files.map((file) => (
                <tr key={file.path}>
                  <td className="font-mono text-[0.68rem]">{file.path}</td>
                  <td className="text-xs text-base-content/50">{file.type || '—'}</td>
                  <td className="whitespace-nowrap text-xs text-base-content/50">{file.size || '—'}</td>
                  <td className="whitespace-nowrap text-xs">
                    {file.editable ? <Pill tone="blue">editable</Pill> : 'managed'}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : page && !loading ? (
        <div className="p-4 text-xs text-base-content/40">No published files match this filter.</div>
      ) : null}
      <div className="flex items-center justify-between border-t border-base-300 p-2">
        <button
          className="btn btn-xs btn-ghost"
          type="button"
          disabled={loading || offset === 0}
          onClick={() => setOffset(Math.max(0, offset - pageSize))}
        >
          <ChevronLeft size={12} /> Previous
        </button>
        <button
          className="btn btn-xs btn-ghost"
          type="button"
          disabled={loading || !page?.has_more}
          onClick={() => setOffset(offset + pageSize)}
        >
          Next <ChevronRight size={12} />
        </button>
      </div>
    </div>
  );
}

export default function AutoModpackPage() {
  const [status, setStatus] = useState<AutoModpackStatus | null>(null);
  const [draft, setDraft] = useState<AutoModpackConfig | null>(null);
  const [rawDraft, setRawDraft] = useState('');
  const [mode, setMode] = useState<ConfigMode>('form');
  const [editing, setEditing] = useState(false);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [actionBusy, setActionBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const [identityDirty, setIdentityDirty] = useState(false);
  const [publishNotes, setPublishNotes] = useState('');
  const [advancedOpen, setAdvancedOpen] = useState(false);
  const [migration, setMigration] = useState<{oldID: string; newID: string} | null>(null);
  const [lastAction, setLastAction] = useState<AutoModpackActionResult | null>(null);
  const [diff, setDiff] = useState<AutoModpackGenerationDiff | null>(null);
  const [diffLoading, setDiffLoading] = useState<number | null>(null);

  const resetDrafts = useCallback((next: AutoModpackStatus) => {
    setDraft(cloneConfig(next.config));
    setRawDraft(next.raw_config ?? '');
    setIdentityDirty(false);
  }, []);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const next = await api<AutoModpackStatus>('/api/automodpack');
      setStatus(next);
      resetDrafts(next);
    } catch (value: unknown) {
      setError(value instanceof Error ? value.message : String(value));
    } finally {
      setLoading(false);
    }
  }, [resetDrafts]);

  useEffect(() => {
    void load();
  }, [load]);

  const dirty = useMemo(() => {
    if (!status || !draft) return false;
    return mode === 'raw'
      ? rawDraft !== (status.raw_config ?? '')
      : !configsEqual(draft, cloneConfig(status.config));
  }, [draft, mode, rawDraft, status]);

  const groupIDs = useMemo(
    () => draft?.categories.flatMap((category) => category.groups.map((group) => group.id)) ?? [],
    [draft],
  );

  const discard = useCallback(() => {
    if (!status) return;
    resetDrafts(status);
    setError(null);
    setMessage(null);
  }, [resetDrafts, status]);

  const refresh = async () => {
    if (dirty && !window.confirm('Discard unsaved AutoModpack configuration changes and refresh?')) return;
    await load();
  };

  const setEditingSafely = (next: boolean) => {
    if (!next && dirty) {
      if (!window.confirm('Discard unsaved AutoModpack configuration changes and leave edit mode?')) return;
      discard();
    }
    setEditing(next);
  };

  const switchMode = (next: ConfigMode) => {
    if (next === mode) return;
    if (dirty && !window.confirm('Discard unsaved changes before switching editors?')) return;
    discard();
    setMode(next);
  };

  const updateSettings = <K extends keyof AutoModpackConfig['settings']>(
    key: K,
    value: AutoModpackConfig['settings'][K],
  ) => {
    setDraft((current) =>
      current ? {...current, settings: {...current.settings, [key]: value}} : current,
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

  const saveForm = async () => {
    if (!draft || !status?.config_sha256) return;
    if (
      identityDirty &&
      !window.confirm(
        'This changes or removes an AutoModpack category/group identity. Existing player group selections may no longer match. Save these identity changes?',
      )
    ) {
      return;
    }
    const next = await api<AutoModpackStatus>('/api/automodpack/config', {
      method: 'PUT',
      body: JSON.stringify({
        expected_sha256: status.config_sha256,
        config: draft,
        confirm_identity_changes: identityDirty,
      }),
    });
    setStatus(next);
    resetDrafts(next);
  };

  const saveRaw = async (confirmIdentityChanges = false): Promise<void> => {
    if (!status?.config_sha256) return;
    try {
      const next = await api<AutoModpackStatus>('/api/automodpack/config/raw', {
        method: 'PUT',
        body: JSON.stringify({
          expected_sha256: status.config_sha256,
          raw_config: rawDraft,
          confirm_identity_changes: confirmIdentityChanges,
        }),
      });
      setStatus(next);
      resetDrafts(next);
    } catch (value: unknown) {
      const detail = value instanceof Error ? value.message : String(value);
      if (
        !confirmIdentityChanges &&
        detail.includes('identity') &&
        detail.includes('confirmation') &&
        window.confirm(
          detail +
            '\n\nThis can strand saved player group/category selections. Save the raw configuration anyway?',
        )
      ) {
        await saveRaw(true);
        return;
      }
      throw value;
    }
  };

  const save = async () => {
    setSaving(true);
    setError(null);
    setMessage(null);
    try {
      if (mode === 'raw') await saveRaw();
      else await saveForm();
      setMessage(
        'Configuration saved while the server remained online. Use Reload config below to apply runtime-readable changes.',
      );
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
    setLastAction(null);
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
      setLastAction(result);
      const next = await api<AutoModpackStatus>('/api/automodpack');
      setStatus(next);
      if (!editing && !dirty) resetDrafts(next);
    } catch (value: unknown) {
      setError(value instanceof Error ? value.message : String(value));
    } finally {
      setActionBusy(null);
    }
  };

  const loadDiff = async (sequence: number) => {
    setDiffLoading(sequence);
    setError(null);
    try {
      const result = await api<AutoModpackGenerationDiff>(
        '/api/automodpack/generations/' + sequence + '/diff',
      );
      setDiff(result);
    } catch (value: unknown) {
      setError(value instanceof Error ? value.message : String(value));
    } finally {
      setDiffLoading(null);
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
      resetDrafts(next);
      setMigration(null);
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
        <PageHeader
          eyebrow="Distribution"
          title="AutoModpack"
          description="AutoModpack integration is unavailable."
        />
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
        description="Inspect configuration, manage groups, browse published content and operate AutoModpack."
        action={
          <div className="flex flex-wrap items-center gap-2">
            {status.pending_publish ? <Pill tone="warn">publish pending</Pill> : <Pill tone="good">published</Pill>}
            <Pill tone={status.installed ? 'good' : 'warn'}>
              {status.installed ? status.version || 'installed' : 'not detected'}
            </Pill>
            <button className="btn btn-sm btn-ghost" type="button" disabled={loading} onClick={() => void refresh()}>
              <RefreshCw size={14} className={loading ? 'animate-spin' : ''} /> Refresh
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
              <h2 className="mt-0.5 text-sm font-semibold">Configuration &amp; content checks</h2>
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
        <div className="panel-header flex-wrap gap-3">
          <div className="min-w-0 flex-1">
            <div className="section-label">Current configuration</div>
            <h2 className="mt-0.5 text-sm font-semibold">
              {editing ? 'Editing server.conf' : 'Read-only server.conf'}
            </h2>
            <div className="mt-1 text-xs text-base-content/40">
              Saving AutoModpack configuration does not require stopping Minecraft. Reload it from Operations when needed.
            </div>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <div className="join">
              <button
                className={'btn btn-xs join-item ' + (mode === 'form' ? 'btn-active' : 'btn-ghost')}
                type="button"
                onClick={() => switchMode('form')}
              >
                <FileCog size={12} /> Form
              </button>
              <button
                className={'btn btn-xs join-item ' + (mode === 'raw' ? 'btn-active' : 'btn-ghost')}
                type="button"
                onClick={() => switchMode('raw')}
              >
                <Code2 size={12} /> Raw
              </button>
            </div>
            <label className="flex cursor-pointer items-center gap-2 text-xs">
              <span>{editing ? 'Edit on' : 'Edit off'}</span>
              <input
                className="toggle toggle-sm"
                type="checkbox"
                checked={editing}
                onChange={(event) => setEditingSafely(event.target.checked)}
              />
            </label>
          </div>
        </div>

        {mode === 'raw' ? (
          <div className="p-4">
            <textarea
              className="textarea textarea-bordered min-h-[55vh] w-full resize-y font-mono text-xs leading-relaxed"
              value={rawDraft}
              readOnly={!editing}
              spellCheck={false}
              aria-label="Raw AutoModpack server.conf"
              onChange={(event) => setRawDraft(event.target.value)}
            />
            <div className="mt-2 text-xs text-base-content/40">
              Raw saves are parsed and validated before the file is replaced. Comments and unknown settings are preserved exactly as entered.
            </div>
          </div>
        ) : (
          <>
            <div className="grid gap-4 p-4 md:grid-cols-2">
              <label className="form-control">
                <span className="mb-1 text-xs text-base-content/45">Pack name</span>
                <input
                  className="input input-sm input-bordered"
                  value={draft.name}
                  readOnly={!editing}
                  onChange={(event) => setDraft({...draft, name: event.target.value})}
                />
              </label>
              <StringListEditor
                label="Accepted loaders"
                values={draft.settings.accepted_loaders}
                disabled={!editing}
                placeholder="neoforge"
                onChange={(values) => updateSettings('accepted_loaders', values)}
              />
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
                    disabled={!editing}
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

            <div className="border-t border-base-300">
              <div className="panel-header">
                <div>
                  <div className="section-label">Group selection</div>
                  <h3 className="mt-0.5 text-sm font-semibold">Categories &amp; groups</h3>
                </div>
                {editing ? (
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
                ) : null}
              </div>
              <div className="space-y-4 p-4">
                {draft.categories.map((category, categoryIndex) => (
                  <div className="rounded-box border border-base-300" key={category.name + ':' + categoryIndex}>
                    <div className="flex flex-col gap-2 border-b border-base-300 bg-base-200/35 p-3 sm:flex-row sm:items-center">
                      <input
                        className="input input-sm input-bordered min-w-0 flex-1"
                        value={category.name}
                        readOnly={!editing}
                        onChange={(event) => {
                          if (event.target.value !== category.name) setIdentityDirty(true);
                          updateCategory(categoryIndex, {name: event.target.value});
                        }}
                        aria-label="Category name"
                      />
                      {editing ? (
                        <>
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
                        </>
                      ) : null}
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
                                      ? content.files +
                                        ' direct files · ' +
                                        content.published_file_count +
                                        ' published · ' +
                                        bytes(content.bytes)
                                      : 'Content directory not scanned'}
                                  </div>
                                </div>
                              </summary>
                              <div className="grid gap-4 border-t border-base-300 bg-base-100 p-4 md:grid-cols-2">
                                <label className="form-control">
                                  <span className="mb-1 text-xs text-base-content/45">Group ID</span>
                                  <div className="join">
                                    <input
                                      className="input input-sm input-bordered join-item w-full font-mono"
                                      value={group.id}
                                      readOnly
                                    />
                                    {editing ? (
                                      <button
                                        className="btn btn-sm btn-outline join-item"
                                        type="button"
                                        onClick={() => setMigration({oldID: group.id, newID: group.id})}
                                      >
                                        Migrate ID
                                      </button>
                                    ) : null}
                                  </div>
                                </label>
                                <label className="form-control">
                                  <span className="mb-1 text-xs text-base-content/45">Display name</span>
                                  <input
                                    className="input input-sm input-bordered"
                                    value={group.display_name}
                                    readOnly={!editing}
                                    onChange={(event) =>
                                      updateGroup(categoryIndex, groupIndex, {display_name: event.target.value})
                                    }
                                  />
                                </label>
                                <label className="form-control md:col-span-2">
                                  <span className="mb-1 text-xs text-base-content/45">Description</span>
                                  <input
                                    className="input input-sm input-bordered"
                                    value={group.description}
                                    readOnly={!editing}
                                    onChange={(event) =>
                                      updateGroup(categoryIndex, groupIndex, {description: event.target.value})
                                    }
                                  />
                                </label>
                                <div className="flex flex-wrap gap-4 md:col-span-2">
                                  <label className="flex items-center gap-2 text-sm">
                                    <input
                                      type="checkbox"
                                      className="checkbox checkbox-sm"
                                      disabled={!editing}
                                      checked={group.required}
                                      onChange={(event) =>
                                        updateGroup(categoryIndex, groupIndex, {required: event.target.checked})
                                      }
                                    />
                                    Required
                                  </label>
                                  <label className="flex items-center gap-2 text-sm">
                                    <input
                                      type="checkbox"
                                      className="checkbox checkbox-sm"
                                      disabled={!editing || group.required}
                                      checked={group.default_selected}
                                      onChange={(event) =>
                                        updateGroup(categoryIndex, groupIndex, {
                                          default_selected: event.target.checked,
                                        })
                                      }
                                    />
                                    Default selected
                                  </label>
                                </div>

                                <StringListEditor
                                  label="Requires"
                                  values={group.requires}
                                  disabled={!editing}
                                  placeholder="main"
                                  onChange={(values) => updateGroup(categoryIndex, groupIndex, {requires: values})}
                                />
                                <StringListEditor
                                  label="Breaks with"
                                  values={group.breaks_with}
                                  disabled={!editing}
                                  placeholder="alternative-group"
                                  onChange={(values) =>
                                    updateGroup(categoryIndex, groupIndex, {breaks_with: values})
                                  }
                                />
                                <StringListEditor
                                  label="Platforms"
                                  values={group.compatible_platforms}
                                  disabled={!editing}
                                  placeholder="windows"
                                  onChange={(values) =>
                                    updateGroup(categoryIndex, groupIndex, {compatible_platforms: values})
                                  }
                                />
                                <div />
                                <StringListEditor
                                  label="From server"
                                  values={group.from_server}
                                  disabled={!editing}
                                  placeholder="mods/*.jar"
                                  mono
                                  onChange={(values) =>
                                    updateGroup(categoryIndex, groupIndex, {from_server: values})
                                  }
                                />
                                <StringListEditor
                                  label="Exclude"
                                  values={group.exclude}
                                  disabled={!editing}
                                  placeholder="kubejs/server_scripts/**"
                                  mono
                                  onChange={(values) => updateGroup(categoryIndex, groupIndex, {exclude: values})}
                                />
                                <StringListEditor
                                  label="Editable"
                                  values={group.editable}
                                  disabled={!editing}
                                  placeholder="config/**"
                                  mono
                                  onChange={(values) => updateGroup(categoryIndex, groupIndex, {editable: values})}
                                />
                                <div />

                                <div className="md:col-span-2">
                                  <PublishedFilesBrowser
                                    group={group.id}
                                    total={content?.published_file_count ?? 0}
                                  />
                                </div>

                                <div className="flex items-center justify-between border-t border-base-300 pt-3 md:col-span-2">
                                  <div className="mono text-xs text-base-content/35">
                                    {content?.path ?? 'automodpack/host-modpack/' + group.id}
                                  </div>
                                  {editing ? (
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
                                  ) : null}
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
            </div>

            <div className="border-t border-base-300">
              <button
                className="panel-header flex w-full items-center text-left"
                type="button"
                onClick={() => setAdvancedOpen((value) => !value)}
              >
                <div className="flex-1">
                  <div className="section-label">Advanced configuration</div>
                  <h3 className="mt-0.5 text-sm font-semibold">Hosting, security &amp; unmodded clients</h3>
                </div>
                <ChevronDown
                  size={16}
                  className={advancedOpen ? 'rotate-180 transition-transform' : 'transition-transform'}
                />
              </button>
              {advancedOpen ? (
                <div className="grid gap-4 border-t border-base-300 p-4 md:grid-cols-2">
                  <label className="form-control">
                    <span className="mb-1 text-xs text-base-content/45">Connection mode</span>
                    <select
                      className="select select-sm select-bordered"
                      disabled={!editing}
                      value={draft.settings.connection_mode}
                      onChange={(event) => updateSettings('connection_mode', event.target.value)}
                    >
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
                        readOnly={!editing}
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
                        disabled={!editing}
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
                        readOnly={!editing}
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
            </div>
          </>
        )}
      </section>

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
              {actionBusy === 'preview' ? <span className="loading loading-spinner loading-xs" /> : <Play size={13} />}
              Preview generation
            </button>
            <button className="btn btn-sm btn-primary" disabled={!!actionBusy} onClick={() => void runAction('publish')}>
              {actionBusy === 'publish' ? <span className="loading loading-spinner loading-xs" /> : <Send size={13} />}
              Publish
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
            FPBPack captures the terminal lines emitted after each command. Configuration saves themselves do not require a server stop.
          </p>

          {lastAction ? (
            <div className="mt-4 overflow-hidden rounded-box border border-base-300">
              <div className="flex flex-wrap items-center justify-between gap-2 border-b border-base-300 bg-base-200/35 px-3 py-2">
                <div>
                  <div className="text-sm font-medium">{lastAction.message}</div>
                  <div className="mt-0.5 font-mono text-[0.68rem] text-base-content/45">{lastAction.command}</div>
                </div>
                <Pill tone={lastAction.output_error ? 'warn' : 'good'}>{lastAction.status}</Pill>
              </div>
              {lastAction.output.length ? (
                <pre className="max-h-72 overflow-auto whitespace-pre-wrap p-3 font-mono text-[0.72rem] leading-relaxed">
                  {lastAction.output.join('\n')}
                </pre>
              ) : (
                <div className="p-3 text-xs text-base-content/45">No new terminal lines were captured for this command.</div>
              )}
              {lastAction.output_error ? (
                <div className="border-t border-base-300 px-3 py-2 text-xs text-warning">
                  {lastAction.output_error}
                </div>
              ) : null}
            </div>
          ) : null}
        </div>
      </section>

      {status.orphan_group_directories?.length ? (
        <section className="panel mb-4 p-4">
          <div className="section-label">Orphan group directories</div>
          <div className="mt-2 flex flex-wrap gap-2">
            {(status.orphan_group_directories ?? []).map((group) => (
              <Pill key={group} tone="warn">{group}</Pill>
            ))}
          </div>
        </section>
      ) : null}

      <section className="panel mb-20 overflow-hidden">
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
                  return (
                    <tr key={generation.sequence}>
                      <td className="font-mono">#{generation.sequence}</td>
                      <td className="whitespace-nowrap">
                        {formatDate(generation.created_at)}
                        {isHead ? <Pill tone="good">head</Pill> : null}
                        {(generation.restore_of ?? -1) > 0 ? (
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
                      <td className="max-w-xs text-xs text-base-content/55">{generation.notes || '—'}</td>
                      <td className="font-mono text-[0.68rem] text-base-content/40">
                        {generation.content_token.slice(0, 12)}
                      </td>
                      <td>
                        {!isHead ? (
                          <div className="flex justify-end">
                            <button
                              className="btn btn-xs btn-ghost"
                              type="button"
                              disabled={diffLoading !== null}
                              onClick={() => void loadDiff(generation.sequence)}
                            >
                              {diffLoading === generation.sequence ? (
                                <span className="loading loading-spinner loading-xs" />
                              ) : (
                                <RotateCcw size={11} />
                              )}
                              View rollback diff
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

      {dirty ? (
        <div className="fixed bottom-4 left-1/2 z-40 w-[min(94vw,46rem)] -translate-x-1/2 rounded-box border border-primary/30 bg-base-100 p-3 shadow-2xl">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
            <div className="min-w-0 flex-1">
              <div className="text-sm font-semibold">Unsaved AutoModpack configuration changes</div>
              <div className="mt-0.5 text-xs text-base-content/45">
                {mode === 'raw' ? 'Raw server.conf editor' : 'Form editor'} · server may remain online
              </div>
            </div>
            <div className="flex justify-end gap-2">
              <button className="btn btn-sm btn-ghost" type="button" disabled={saving} onClick={discard}>
                <RotateCcw size={13} /> Discard
              </button>
              <button className="btn btn-sm btn-primary" type="button" disabled={saving} onClick={() => void save()}>
                {saving ? <span className="loading loading-spinner loading-xs" /> : <Save size={13} />} Save
              </button>
            </div>
          </div>
        </div>
      ) : null}

      {migration ? (
        <div className="modal modal-open">
          <div className="modal-box max-w-lg">
            <div className="flex items-start justify-between">
              <div>
                <div className="section-label">Identity migration</div>
                <h2 className="mt-1 text-lg font-semibold">Rename group {migration.oldID}</h2>
              </div>
              <button className="btn btn-sm btn-ghost btn-square" onClick={() => setMigration(null)}>
                <X size={15} />
              </button>
            </div>
            <div className="alert alert-warning mt-4 py-3 text-xs">
              This operation also renames the group directory and managed placement references, so FPBPack still requires the server to be stopped for this specific filesystem migration. Ordinary server.conf edits do not.
            </div>
            <label className="form-control mt-4">
              <span className="mb-1 text-xs text-base-content/45">New group ID</span>
              <input
                className="input input-bordered font-mono"
                value={migration.newID}
                onChange={(event) => setMigration({...migration, newID: event.target.value})}
              />
            </label>
            <div className="modal-action">
              <button className="btn btn-ghost" onClick={() => setMigration(null)}>Cancel</button>
              <button
                className="btn btn-warning"
                disabled={actionBusy === 'migrate' || migration.newID.trim() === migration.oldID}
                onClick={() => void migrateGroup()}
              >
                {actionBusy === 'migrate' ? <span className="loading loading-spinner loading-xs" /> : <FolderTree size={14} />}
                Migrate identity
              </button>
            </div>
          </div>
          <button className="modal-backdrop" type="button" onClick={() => setMigration(null)}>close</button>
        </div>
      ) : null}

      {diff ? (
        <div className="modal modal-open">
          <div className="modal-box max-w-5xl">
            <div className="flex items-start justify-between gap-3">
              <div>
                <div className="section-label">Rollback diff</div>
                <h2 className="mt-1 text-lg font-semibold">
                  Head #{diff.head_sequence} → generation #{diff.target_sequence}
                </h2>
                <div className="mt-2 flex gap-2 text-xs">
                  <Pill tone="good">+{diff.added} add</Pill>
                  <Pill tone="neutral">~{diff.changed} change</Pill>
                  <Pill tone="bad">−{diff.removed} remove</Pill>
                </div>
              </div>
              <button className="btn btn-sm btn-ghost btn-square" onClick={() => setDiff(null)}>
                <X size={15} />
              </button>
            </div>
            <div className="mt-4 max-h-[55vh] overflow-auto rounded-box border border-base-300">
              {diff.entries.length ? (
                <table className="table table-xs">
                  <thead>
                    <tr>
                      <th>Action</th>
                      <th>Path</th>
                      <th>Current</th>
                      <th>Rollback target</th>
                    </tr>
                  </thead>
                  <tbody>
                    {diff.entries.map((entry) => (
                      <tr key={entry.path}>
                        <td>
                          <Pill tone={entry.action === 'add' ? 'good' : entry.action === 'remove' ? 'bad' : 'neutral'}>
                            {entry.action}
                          </Pill>
                        </td>
                        <td className="font-mono text-[0.68rem]">{entry.path}</td>
                        <td className="font-mono text-[0.65rem] text-base-content/45">
                          {entry.current_sha1 ? entry.current_sha1.slice(0, 10) + ' · ' + bytes(entry.current_size ?? 0) : 'absent'}
                        </td>
                        <td className="font-mono text-[0.65rem] text-base-content/45">
                          {entry.target_sha1 ? entry.target_sha1.slice(0, 10) + ' · ' + bytes(entry.target_size ?? 0) : 'absent'}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              ) : (
                <div className="p-4 text-sm text-base-content/45">
                  This generation has the same content bytes as the current head. A rollback may still restore older group policy.
                </div>
              )}
            </div>
            <div className="modal-action">
              <button className="btn btn-ghost" onClick={() => setDiff(null)}>Close</button>
              <button
                className="btn btn-warning"
                disabled={!!actionBusy}
                onClick={async () => {
                  const sequence = diff.target_sequence;
                  setDiff(null);
                  await runAction('revert_confirm', sequence);
                }}
              >
                <RotateCcw size={13} /> Publish rollback
              </button>
            </div>
          </div>
          <button className="modal-backdrop" type="button" onClick={() => setDiff(null)}>close</button>
        </div>
      ) : null}
    </>
  );
}
