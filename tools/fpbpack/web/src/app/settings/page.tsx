'use client';

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
        description="FPBPack serves this interface and its API from the same process."
        action={
          <Pill tone={connected ? 'good' : connectionStatus === 'error' ? 'warn' : 'blue'}>
            {connectionStatus === 'loading'
              ? 'Loading…'
              : connected
                ? 'Connected'
                : 'Connection failed'}
          </Pill>
        }
      />

      {connectionError ? (
        <section className="notice notice-warn">
          <strong>Service warning</strong>
          <p>{connectionError}</p>
        </section>
      ) : null}

      <section className="panel">
        <div className="panel-heading">
          <div>
            <span className="eyebrow">FPBPack service</span>
            <h2>Same-origin GUI + API</h2>
          </div>
          <Pill tone={connected ? 'good' : 'neutral'}>
            {connected ? 'Live backend' : 'Unavailable'}
          </Pill>
        </div>

        <div className="connection-meta">
          <div><span>Transport</span><strong>Same origin</strong></div>
          <div><span>API base</span><strong className="mono">/api</strong></div>
          <div><span>GUI delivery</span><strong>Embedded static assets</strong></div>
        </div>

        <div className="connection-actions">
          <button
            type="button"
            className="secondary-button"
            onClick={() => void refresh()}
            disabled={connectionStatus === 'loading'}
          >
            Refresh service state
          </button>
        </div>
      </section>

      <section className="panel">
        <div className="panel-heading">
          <div>
            <span className="eyebrow">Status</span>
            <h2>Current backend</h2>
          </div>
          <Pill tone={state.status.read_only ? 'blue' : 'warn'}>
            {state.status.read_only ? 'Read only' : state.status.mode}
          </Pill>
        </div>
        <dl className="detail-list">
          <div><dt>FPBPack version</dt><dd>{state.status.version ?? 'Unavailable'}</dd></div>
          <div><dt>Server state</dt><dd>{state.status.server_state}</dd></div>
          <div><dt>Installed JARs</dt><dd>{state.status.mods}</dd></div>
          <div><dt>Managed</dt><dd>{state.status.managed}</dd></div>
          <div><dt>Explicitly unmanaged</dt><dd>{state.status.unmanaged}</dd></div>
        </dl>
      </section>
    </>
  );
}
