'use client';

import {RefreshCw} from 'lucide-react';
import {PageHeader, Pill} from '@/components/ui';
import {useManagement} from '@/components/management-provider';

export default function SettingsPage() {
  const {state, connectionStatus, connectionError, refresh} = useManagement();
  const connected = connectionStatus === 'connected';

  return (
    <>
      <PageHeader
        eyebrow="System"
        title="Settings"
        description="Runtime status and FPBPack service configuration."
        action={<Pill tone={connected ? 'good' : connectionStatus === 'error' ? 'warn' : 'blue'}>
          {connectionStatus === 'loading' ? 'Loading…' : connected ? 'Connected' : 'Connection failed'}
        </Pill>}
      />

      {connectionError ? <div className="alert alert-warning mb-4 rounded-box py-3 text-sm">{connectionError}</div> : null}

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
            ].map(([label, value]) => (
              <div className="flex items-center justify-between gap-4 px-4 py-3" key={label}>
                <dt className="text-base-content/45">{label}</dt>
                <dd className={label === 'API base' ? 'mono' : 'font-medium'}>{value}</dd>
              </div>
            ))}
          </dl>
          <div className="border-t border-base-300 p-3">
            <button className="btn btn-sm btn-ghost" type="button" onClick={() => void refresh()} disabled={connectionStatus === 'loading'}>
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
            <Pill tone={state.status.read_only ? 'blue' : 'warn'}>{state.status.read_only ? 'Read only' : state.status.mode}</Pill>
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
      </div>
    </>
  );
}
