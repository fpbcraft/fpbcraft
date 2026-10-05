'use client';

import Link from 'next/link';
import {EmptyState, Metric, PageHeader, Pill} from '@/components/ui';
import {useManagement} from '@/components/management-provider';
import type {DiagnosticLevel} from '@/lib/management';

function findingTone(level: DiagnosticLevel): 'bad' | 'warn' | 'neutral' {
  if (level === 'blocking') return 'bad';
  if (level === 'warning') return 'warn';
  return 'neutral';
}

export default function OverviewPage() {
  const {state, connectionStatus} = useManagement();
  const {status, diagnostics} = state;
  const serverTone =
    status.server_state === 'running'
      ? 'good'
      : status.server_state === 'stopped'
        ? 'warn'
        : 'neutral';

  return (
    <>
      <PageHeader
        eyebrow="FPBPack"
        title="Overview"
        description="Current mod-management state and anything that needs attention."
        action={
          <div className="header-pills">
            <Pill tone={state.source === 'api' ? 'good' : 'warn'}>
              {connectionStatus === 'loading'
                ? 'Loading…'
                : state.source === 'api'
                  ? 'FPBPack'
                  : 'API unavailable'}
            </Pill>
            <Pill tone={diagnostics.summary.blocking > 0 ? 'bad' : 'good'}>
              {diagnostics.summary.blocking > 0
                ? String(diagnostics.summary.blocking) + ' blocking'
                : 'No blockers'}
            </Pill>
          </div>
        }
      />

      {state.errors.length ? (
        <section className="notice notice-warn">
          <strong>Data source warning</strong>
          {state.errors.map((error) => (
            <p key={error}>{error}</p>
          ))}
        </section>
      ) : null}

      <section className="metrics-grid">
        <Metric
          label="Server"
          value={status.server_state === 'unknown' ? 'Unknown' : status.server_state}
          detail="Crafty state will be connected before Apply"
          tone={serverTone}
        />
        <Metric
          label="Updates"
          value="—"
          detail="Provider-aware discovery is being connected in Slice 1"
        />
        <Metric
          label="Needs attention"
          value={diagnostics.summary.actionable}
          detail="Blocking findings and warnings"
          tone={diagnostics.summary.actionable > 0 ? 'warn' : 'good'}
        />
        <Metric
          label="Managed mods"
          value={status.managed}
          detail={String(status.unmanaged) + ' explicitly unmanaged'}
          tone="good"
        />
      </section>

      <section className="two-column">
        <article className="panel">
          <div className="panel-heading">
            <div>
              <span className="eyebrow">Inbox</span>
              <h2>Needs attention</h2>
            </div>
            <Link href="/mods">View mods</Link>
          </div>

          {diagnostics.findings.length ? (
            <div className="stack-list">
              {diagnostics.findings.slice(0, 6).map((finding, index) => (
                <div
                  className="stack-row"
                  key={finding.code + ':' + (finding.path ?? '') + ':' + String(index)}
                >
                  <div>
                    <strong>{finding.mod ?? finding.code.replaceAll('_', ' ')}</strong>
                    <span>{finding.message}</span>
                    {finding.path ? <span className="subtle mono">{finding.path}</span> : null}
                  </div>
                  <Pill tone={findingTone(finding.level)}>{finding.level}</Pill>
                </div>
              ))}
            </div>
          ) : (
            <EmptyState title="Nothing needs attention">
              The current accepted inventory has no diagnostics findings.
            </EmptyState>
          )}
        </article>

        <article className="panel">
          <div className="panel-heading">
            <div>
              <span className="eyebrow">Updates</span>
              <h2>Available updates</h2>
            </div>
            <Link href="/updates">Open updates</Link>
          </div>
          <EmptyState title="Update discovery not connected yet">
            The GUI is served by FPBPack and reads management state from the same-origin API.
          </EmptyState>
        </article>
      </section>

      <section className="panel">
        <div className="panel-heading">
          <div>
            <span className="eyebrow">Inventory</span>
            <h2>{status.mods} installed JARs</h2>
          </div>
          <Pill tone="blue">Read only</Pill>
        </div>
        <div className="safety-grid">
          <div>
            <strong>{status.managed}</strong>
            <span>managed artifacts</span>
          </div>
          <div>
            <strong>{status.unmanaged}</strong>
            <span>explicitly unmanaged artifacts</span>
          </div>
          <div>
            <strong>{diagnostics.summary.blocking}</strong>
            <span>blocking diagnostics</span>
          </div>
        </div>
      </section>
    </>
  );
}
