'use client';

import {useEffect, useState} from 'react';
import {KeyRound, RefreshCw, Save, Trash2} from 'lucide-react';
import {PageHeader, Pill} from '@/components/ui';
import {useManagement} from '@/components/management-provider';
import {api} from '@/lib/api';

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

  useEffect(() => {
    void Promise.all([
      api<RuntimeSettings>('/api/settings').then((settings) => {
        setRetention(settings.retention_count);
        setSavedRetention(settings.retention_count);
      }),
      api<{providers: ProviderStatus[]}>('/api/providers').then((response) => {
        setProviders(response.providers);
      }),
    ]).catch((error: unknown) => {
      setSettingsError(error instanceof Error ? error.message : String(error));
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
