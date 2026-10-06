'use client';

import {useEffect, useState} from 'react';
import {ArrowDownUp, KeyRound, RefreshCw, Save, Trash2} from 'lucide-react';
import {PageHeader, Pill} from '@/components/ui';
import {useManagement} from '@/components/management-provider';
import {api} from '@/lib/api';
import type {CraftyStatus, NeoForgeChangeResult, NeoForgeStatus} from '@/lib/management';

interface RuntimeSettings {
  retention_count: number;
}

interface ProviderStatus {
  id: string;
  label: string;
  status: string;
  detail: string;
  credential_configurable?: boolean;
  credential_source?: 'saved' | 'environment';
}

export default function SettingsPage() {
  const {state, connectionStatus, connectionError, refresh} = useManagement();
  const connected = connectionStatus === 'connected';
  const [retention, setRetention] = useState(20);
  const [savedRetention, setSavedRetention] = useState(20);
  const [settingsError, setSettingsError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [providers, setProviders] = useState<ProviderStatus[]>([]);
  const [curseForgeKey, setCurseForgeKey] = useState('');
  const [savingCurseForge, setSavingCurseForge] = useState(false);
  const [clearingCurseForge, setClearingCurseForge] = useState(false);
  const [crafty, setCrafty] = useState<CraftyStatus | null>(null);
  const [craftyURL, setCraftyURL] = useState('');
  const [craftyServerID, setCraftyServerID] = useState('');
  const [craftyToken, setCraftyToken] = useState('');
  const [craftyInsecure, setCraftyInsecure] = useState(false);
  const [savingCrafty, setSavingCrafty] = useState(false);
  const [clearingCrafty, setClearingCrafty] = useState(false);
  const [neoForge, setNeoForge] = useState<NeoForgeStatus | null>(null);
  const [neoForgeTarget, setNeoForgeTarget] = useState('');
  const [neoForgeBusy, setNeoForgeBusy] = useState(false);
  const [neoForgeError, setNeoForgeError] = useState<string | null>(null);
  const [neoForgeMessage, setNeoForgeMessage] = useState<string | null>(null);

  useEffect(() => {
    void Promise.all([
      api<RuntimeSettings>('/api/settings').then((settings) => {
        setRetention(settings.retention_count);
        setSavedRetention(settings.retention_count);
      }),
      api<{providers: ProviderStatus[]}>('/api/providers').then((response) => {
        setProviders(response.providers);
      }),
      api<CraftyStatus>('/api/crafty').then((status) => {
        setCrafty(status);
        setCraftyURL(status.url ?? '');
        setCraftyServerID(status.server_id ?? '');
        setCraftyInsecure(status.allow_insecure ?? false);
      }),
    ]).catch((error: unknown) => {
      setSettingsError(error instanceof Error ? error.message : String(error));
    });

    void api<NeoForgeStatus>('/api/neoforge')
      .then((status) => {
        setNeoForge(status);
        setNeoForgeTarget(status.current_version ?? status.latest_version ?? status.versions[0]?.version ?? '');
      })
      .catch((error: unknown) => {
        setNeoForgeError(error instanceof Error ? error.message : String(error));
      });
  }, []);

  const saveRetention = async () => {
    setSaving(true);
    setSettingsError(null);
    try {
      const updated = await api<RuntimeSettings>('/api/settings', {
        method: 'PUT',
        body: JSON.stringify({retention_count: retention}),
      });
      setRetention(updated.retention_count);
      setSavedRetention(updated.retention_count);
    } catch (error: unknown) {
      setSettingsError(error instanceof Error ? error.message : String(error));
    } finally {
      setSaving(false);
    }
  };

  const updateProvider = (updated: ProviderStatus) => {
    setProviders((current) =>
      current.map((provider) => (provider.id === updated.id ? updated : provider)),
    );
  };

  const saveCurseForgeKey = async () => {
    const apiKey = curseForgeKey.trim();
    if (!apiKey) return;

    setSavingCurseForge(true);
    setSettingsError(null);
    try {
      const updated = await api<ProviderStatus>('/api/providers/curseforge/credentials', {
        method: 'PUT',
        body: JSON.stringify({api_key: apiKey}),
      });
      updateProvider(updated);
      setCurseForgeKey('');
    } catch (error: unknown) {
      setSettingsError(error instanceof Error ? error.message : String(error));
    } finally {
      setSavingCurseForge(false);
    }
  };

  const clearCurseForgeKey = async () => {
    setClearingCurseForge(true);
    setSettingsError(null);
    try {
      const updated = await api<ProviderStatus>('/api/providers/curseforge/credentials', {
        method: 'DELETE',
      });
      updateProvider(updated);
      setCurseForgeKey('');
    } catch (error: unknown) {
      setSettingsError(error instanceof Error ? error.message : String(error));
    } finally {
      setClearingCurseForge(false);
    }
  };

  const saveCrafty = async () => {
    setSavingCrafty(true);
    setSettingsError(null);
    try {
      const updated = await api<CraftyStatus>('/api/crafty', {
        method: 'PUT',
        body: JSON.stringify({
          url: craftyURL,
          server_id: craftyServerID,
          api_token: craftyToken || undefined,
          allow_insecure: craftyInsecure,
        }),
      });
      setCrafty(updated);
      setCraftyURL(updated.url ?? craftyURL);
      setCraftyServerID(updated.server_id ?? craftyServerID);
      setCraftyInsecure(updated.allow_insecure ?? craftyInsecure);
      setCraftyToken('');
    } catch (error: unknown) {
      setSettingsError(error instanceof Error ? error.message : String(error));
    } finally {
      setSavingCrafty(false);
    }
  };

  const clearCraftyCredential = async () => {
    setClearingCrafty(true);
    setSettingsError(null);
    try {
      const updated = await api<CraftyStatus>('/api/crafty/credentials', {method: 'DELETE'});
      setCrafty(updated);
      setCraftyToken('');
    } catch (error: unknown) {
      setSettingsError(error instanceof Error ? error.message : String(error));
    } finally {
      setClearingCrafty(false);
    }
  };

  const refreshNeoForge = async () => {
    setNeoForgeError(null);
    try {
      const status = await api<NeoForgeStatus>('/api/neoforge');
      setNeoForge(status);
      setNeoForgeTarget((current) => {
        if (current && status.versions.some((version) => version.version === current)) {
          return current;
        }
        return status.current_version ?? status.latest_version ?? status.versions[0]?.version ?? '';
      });
    } catch (error: unknown) {
      setNeoForgeError(error instanceof Error ? error.message : String(error));
    }
  };

  const changeNeoForge = async () => {
    if (!neoForgeTarget) return;
    setNeoForgeBusy(true);
    setNeoForgeError(null);
    setNeoForgeMessage(null);
    try {
      const result = await api<NeoForgeChangeResult>('/api/neoforge/change', {
        method: 'POST',
        body: JSON.stringify({version: neoForgeTarget}),
      });
      await refreshNeoForge();
      setNeoForgeMessage(
        `NeoForge ${result.direction} completed: ${result.from_version} → ${result.to_version}. The server remains stopped.`,
      );
    } catch (error: unknown) {
      setNeoForgeError(error instanceof Error ? error.message : String(error));
    } finally {
      setNeoForgeBusy(false);
    }
  };

  const currentNeoForgeIndex =
    neoForge?.versions.findIndex((version) => version.version === neoForge.current_version) ?? -1;
  const targetNeoForgeIndex =
    neoForge?.versions.findIndex((version) => version.version === neoForgeTarget) ?? -1;
  const neoForgeAction =
    neoForgeTarget && neoForgeTarget === neoForge?.current_version
      ? 'Reinstall'
      : currentNeoForgeIndex >= 0 && targetNeoForgeIndex >= 0 && targetNeoForgeIndex < currentNeoForgeIndex
        ? 'Upgrade'
        : currentNeoForgeIndex >= 0 && targetNeoForgeIndex > currentNeoForgeIndex
          ? 'Downgrade'
          : 'Change version';

  return (
    <>
      <PageHeader
        eyebrow="System"
        title="Settings"
        description="Runtime status and FPBPack-owned management settings."
        action={
          <Pill tone={connected ? 'good' : connectionStatus === 'error' ? 'warn' : 'blue'}>
            {connectionStatus === 'loading' ? 'Loading…' : connected ? 'Connected' : 'Connection failed'}
          </Pill>
        }
      />

      {connectionError ? (
        <div className="alert alert-warning mb-4 rounded-box py-3 text-sm">{connectionError}</div>
      ) : null}
      {settingsError ? (
        <div className="alert alert-error mb-4 rounded-box py-3 text-sm">{settingsError}</div>
      ) : null}
      {state.status.refresh?.last_error ? (
        <div className="alert alert-warning mb-4 rounded-box py-3 text-sm">
          Background refresh failed: {state.status.refresh.last_error}
        </div>
      ) : null}

      <div className="grid gap-4 xl:grid-cols-2">
        <section className="panel">
          <div className="panel-header">
            <div>
              <div className="section-label">FPBPack service</div>
              <h2 className="mt-0.5 text-sm font-semibold">Connection</h2>
            </div>
            <Pill tone={connected ? 'good' : 'neutral'}>{connected ? 'Live' : 'Unavailable'}</Pill>
          </div>
          <dl className="divide-y divide-base-300 text-sm">
            {[
              ['Transport', 'Same origin'],
              ['API base', '/api'],
              ['GUI delivery', 'Embedded static assets'],
              ['FPBPack version', state.status.version ?? 'Unavailable'],
              ['Refresh state', state.status.refresh?.refreshing ? 'Refreshing' : state.status.refresh?.last_error ? 'Failed' : 'Idle'],
              ['Last refresh', state.status.refresh?.last_success ? new Date(state.status.refresh.last_success).toLocaleString() : '—'],
            ].map(([label, value]) => (
              <div className="flex items-center justify-between gap-4 px-4 py-3" key={label}>
                <dt className="text-base-content/45">{label}</dt>
                <dd className={label === 'API base' ? 'mono' : 'font-medium'}>{value}</dd>
              </div>
            ))}
          </dl>
          <div className="border-t border-base-300 p-3">
            <button
              className="btn btn-sm btn-ghost"
              type="button"
              onClick={() => void refresh()}
              disabled={connectionStatus === 'loading'}
            >
              <RefreshCw size={14} /> Refresh state
            </button>
          </div>
        </section>

        <section className="panel">
          <div className="panel-header">
            <div>
              <div className="section-label">Runtime</div>
              <h2 className="mt-0.5 text-sm font-semibold">Current backend</h2>
            </div>
            <Pill tone={state.status.read_only ? 'blue' : 'warn'}>
              {state.status.read_only ? 'Read only' : state.status.mode}
            </Pill>
          </div>
          <dl className="divide-y divide-base-300 text-sm">
            {[
              ['Server state', state.status.server_state],
              ['Installed JARs', String(state.status.mods)],
              ['Managed', String(state.status.managed)],
              ['Explicitly unmanaged', String(state.status.unmanaged)],
            ].map(([label, value]) => (
              <div className="flex items-center justify-between gap-4 px-4 py-3" key={label}>
                <dt className="text-base-content/45">{label}</dt>
                <dd className="font-medium">{value}</dd>
              </div>
            ))}
          </dl>
        </section>

        <section className="panel xl:col-span-2">
          <div className="panel-header">
            <div>
              <div className="section-label">Crafty Controller</div>
              <h2 className="mt-0.5 text-sm font-semibold">Minecraft server control</h2>
            </div>
            <Pill tone={crafty?.connected ? 'good' : crafty?.configured ? 'warn' : 'neutral'}>
              {crafty?.connected
                ? crafty.state
                : crafty?.configured
                  ? 'Connection failed'
                  : 'Not configured'}
            </Pill>
          </div>
          <div className="p-4">
            <p className="mb-3 text-xs text-base-content/45">
              Apply and Restore require Crafty to positively report this server as stopped.
              The API token is saved in secrets.json with owner-only permissions and is never returned
              to the browser.
            </p>
            {crafty?.detail ? (
              <div className="mb-3 text-xs text-base-content/55">{crafty.detail}</div>
            ) : null}
            <div className="grid gap-3 md:grid-cols-2">
              <label className="form-control">
                <span className="mb-1 text-xs text-base-content/45">Crafty URL</span>
                <input
                  className="input input-sm input-bordered"
                  value={craftyURL}
                  onChange={(event) => setCraftyURL(event.target.value)}
                  placeholder="https://crafty:8443"
                />
              </label>
              <label className="form-control">
                <span className="mb-1 text-xs text-base-content/45">Server ID / UUID</span>
                <input
                  className="input input-sm input-bordered"
                  value={craftyServerID}
                  onChange={(event) => setCraftyServerID(event.target.value)}
                  placeholder="Crafty server ID"
                />
              </label>
              <label className="form-control md:col-span-2">
                <span className="mb-1 text-xs text-base-content/45">
                  {crafty?.credential_source ? 'Replace API token' : 'API token'}
                </span>
                <div className="input input-sm input-bordered flex items-center gap-2">
                  <KeyRound size={13} className="text-base-content/35" />
                  <input
                    className="min-w-0 grow"
                    type="password"
                    autoComplete="new-password"
                    value={craftyToken}
                    onChange={(event) => setCraftyToken(event.target.value)}
                    placeholder={crafty?.credential_source ? 'Leave blank to keep current token' : 'Paste Crafty API token'}
                  />
                </div>
              </label>
            </div>
            <label className="mt-3 flex cursor-pointer items-center gap-2 text-xs">
              <input
                className="checkbox checkbox-sm"
                type="checkbox"
                checked={craftyInsecure}
                onChange={(event) => setCraftyInsecure(event.target.checked)}
              />
              Allow Crafty's self-signed TLS certificate
            </label>
            <div className="mt-4 flex flex-wrap gap-2">
              <button
                className="btn btn-sm btn-primary"
                type="button"
                disabled={savingCrafty || !craftyURL.trim() || !craftyServerID.trim() || (!craftyToken.trim() && !crafty?.credential_source)}
                onClick={() => void saveCrafty()}
              >
                {savingCrafty ? <span className="loading loading-spinner loading-xs" /> : <Save size={13} />}
                Validate & save
              </button>
              {crafty?.credential_source === 'saved' ? (
                <button
                  className="btn btn-sm btn-ghost"
                  type="button"
                  disabled={clearingCrafty}
                  onClick={() => void clearCraftyCredential()}
                >
                  {clearingCrafty ? <span className="loading loading-spinner loading-xs" /> : <Trash2 size={13} />}
                  Clear saved token
                </button>
              ) : null}
            </div>
          </div>
        </section>

        <section className="panel xl:col-span-2">
          <div className="panel-header">
            <div>
              <div className="section-label">NeoForge runtime</div>
              <h2 className="mt-0.5 text-sm font-semibold">Server loader version</h2>
            </div>
            <Pill tone={neoForge?.current_version ? 'good' : 'warn'}>
              {neoForge?.current_version ? `NeoForge ${neoForge.current_version}` : 'Version unknown'}
            </Pill>
          </div>
          <div className="p-4">
            {neoForgeError ? (
              <div className="alert alert-error mb-4 rounded-box py-3 text-sm">{neoForgeError}</div>
            ) : null}
            {neoForgeMessage ? (
              <div className="alert alert-success mb-4 rounded-box py-3 text-sm">{neoForgeMessage}</div>
            ) : null}
            <div className="grid gap-4 lg:grid-cols-[1fr_auto] lg:items-end">
              <div>
                <p className="text-sm text-base-content/65">
                  Install an exact NeoForge version for Minecraft {neoForge?.minecraft || state.updates.minecraft || '—'}.
                  FPBPack verifies the official installer SHA-512, runs it against the mounted server directory,
                  preserves <span className="mono">user_jvm_args.txt</span>, and updates Crafty's launch paths.
                </p>
                <p className="mt-1 text-xs text-base-content/40">
                  The server must already be stopped. FPBPack does not restart it after an upgrade or downgrade,
                  so you can review the result before starting it again.
                </p>
                {neoForge?.detail ? (
                  <p className="mt-2 text-xs text-base-content/45">{neoForge.detail}</p>
                ) : null}
              </div>
              <button
                className="btn btn-sm btn-ghost"
                type="button"
                disabled={neoForgeBusy}
                onClick={() => void refreshNeoForge()}
              >
                <RefreshCw size={13} /> Refresh versions
              </button>
            </div>

            <div className="mt-4 grid gap-3 md:grid-cols-[minmax(0,1fr)_auto] md:items-end">
              <label className="form-control">
                <span className="mb-1 text-xs text-base-content/45">Target NeoForge version</span>
                <select
                  className="select select-sm select-bordered w-full"
                  value={neoForgeTarget}
                  disabled={neoForgeBusy || !neoForge}
                  onChange={(event) => setNeoForgeTarget(event.target.value)}
                >
                  {!neoForgeTarget ? <option value="">Select a version</option> : null}
                  {neoForge?.versions.map((version) => (
                    <option value={version.version} key={version.version}>
                      {version.version}
                      {version.channel !== 'release' ? ` — ${version.channel}` : ''}
                      {version.current ? ' — current' : ''}
                    </option>
                  ))}
                </select>
                <span className="mt-1 text-xs text-base-content/35">
                  Latest release: {neoForge?.latest_version ?? '—'}
                </span>
              </label>
              <button
                className={`btn btn-sm ${neoForgeAction === 'Downgrade' ? 'btn-warning' : 'btn-primary'}`}
                type="button"
                disabled={
                  neoForgeBusy ||
                  !neoForgeTarget ||
                  !neoForge?.current_version ||
                  neoForge?.server_state !== 'stopped'
                }
                onClick={() => void changeNeoForge()}
              >
                {neoForgeBusy ? (
                  <span className="loading loading-spinner loading-xs" />
                ) : (
                  <ArrowDownUp size={14} />
                )}
                {neoForgeBusy ? 'Installing…' : neoForgeAction}
              </button>
            </div>

            {neoForge?.server_state !== 'stopped' ? (
              <div className="mt-3 alert alert-warning rounded-box py-2 text-xs">
                Stop the Minecraft server before changing NeoForge. Current Crafty state: {neoForge?.server_state ?? 'unknown'}.
              </div>
            ) : null}
            {neoForgeAction === 'Downgrade' ? (
              <div className="mt-3 alert alert-warning rounded-box py-2 text-xs">
                Downgrading the loader can make installed mods incompatible. FPBPack keeps the old NeoForge runtime
                installed and leaves the server stopped so the change can be reviewed before startup.
              </div>
            ) : null}
          </div>
        </section>

        <section className="panel xl:col-span-2">
          <div className="panel-header">
            <div>
              <div className="section-label">Providers</div>
              <h2 className="mt-0.5 text-sm font-semibold">Update sources</h2>
            </div>
          </div>
          <div className="divide-y divide-base-300">
            {providers.map((provider) => (
              <div className="px-4 py-3" key={provider.id}>
                <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
                  <div>
                    <div className="flex flex-wrap items-center gap-2">
                      <div className="text-sm font-medium">{provider.label}</div>
                      {provider.credential_source ? (
                        <Pill tone="neutral">
                          {provider.credential_source === 'saved' ? 'Saved in FPBPack' : 'Environment'}
                        </Pill>
                      ) : null}
                    </div>
                    <div className="mt-0.5 text-xs text-base-content/45">{provider.detail}</div>
                  </div>
                  <Pill tone={provider.status === 'ready' ? 'good' : 'warn'}>
                    {provider.status === 'ready' ? 'Ready' : 'Needs configuration'}
                  </Pill>
                </div>

                {provider.id === 'curseforge' && provider.credential_configurable ? (
                  <div className="mt-3 border-t border-base-300 pt-3">
                    <p className="mb-2 text-xs text-base-content/40">
                      Saved locally in the FPBPack state directory with owner-only file permissions.
                      Existing credentials are never returned to the browser.
                    </p>
                    <div className="flex flex-col gap-2 sm:flex-row sm:items-end">
                    <label className="form-control min-w-0 flex-1">
                      <span className="mb-1 text-xs text-base-content/45">
                        {provider.credential_source === 'saved' ? 'Replace API key' : 'CurseForge API key'}
                      </span>
                      <div className="input input-sm input-bordered flex items-center gap-2">
                        <KeyRound size={13} className="text-base-content/35" />
                        <input
                          className="min-w-0 grow"
                          type="password"
                          autoComplete="new-password"
                          value={curseForgeKey}
                          onChange={(event) => setCurseForgeKey(event.target.value)}
                          placeholder={provider.status === 'ready' ? 'Enter a new key to replace it' : 'Paste API key'}
                          aria-label="CurseForge API key"
                        />
                      </div>
                    </label>
                    <button
                      className="btn btn-sm btn-primary"
                      type="button"
                      disabled={savingCurseForge || !curseForgeKey.trim()}
                      onClick={() => void saveCurseForgeKey()}
                    >
                      {savingCurseForge ? <span className="loading loading-spinner loading-xs" /> : <Save size={13} />}
                      Validate & save
                    </button>
                    {provider.credential_source === 'saved' ? (
                      <button
                        className="btn btn-sm btn-ghost"
                        type="button"
                        disabled={clearingCurseForge}
                        onClick={() => void clearCurseForgeKey()}
                      >
                        {clearingCurseForge ? <span className="loading loading-spinner loading-xs" /> : <Trash2 size={13} />}
                        Clear saved key
                      </button>
                    ) : null}
                    </div>
                  </div>
                ) : null}
              </div>
            ))}
          </div>
        </section>

        <section className="panel xl:col-span-2">
          <div className="panel-header">
            <div>
              <div className="section-label">Plan &amp; Protect</div>
              <h2 className="mt-0.5 text-sm font-semibold">History and restore-point retention</h2>
            </div>
            <Pill tone="neutral">{savedRetention} retained</Pill>
          </div>
          <div className="flex flex-col gap-4 p-4 sm:flex-row sm:items-end sm:justify-between">
            <div className="max-w-2xl">
              <p className="text-sm text-base-content/65">
                Keep the newest plan/history records and their linked restore points.
              </p>
              <p className="mt-1 text-xs text-base-content/40">
                Valid range: 1–100. Reducing this value prunes older completed plan records immediately.
              </p>
            </div>
            <div className="flex items-end gap-2">
              <label className="form-control w-32">
                <span className="mb-1 text-xs text-base-content/45">Records</span>
                <input
                  className="input input-sm input-bordered w-full"
                  type="number"
                  min={1}
                  max={100}
                  value={retention}
                  onChange={(event) => setRetention(Number(event.target.value))}
                />
              </label>
              <button
                className="btn btn-sm btn-primary"
                type="button"
                disabled={saving || retention === savedRetention || retention < 1 || retention > 100}
                onClick={() => void saveRetention()}
              >
                {saving ? <span className="loading loading-spinner loading-xs" /> : <Save size={14} />}
                Save
              </button>
            </div>
          </div>
        </section>
      </div>
    </>
  );
}
