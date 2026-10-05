'use client';

import Link from 'next/link';
import {AlertTriangle, ArrowRight, PackageCheck, Server} from 'lucide-react';
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
  const {status, diagnostics, updates} = state;
  const availableUpdates = updates.summary.safe + updates.summary.review;

  return (
    <>
      <PageHeader
        eyebrow="FPBPack"
        title="Overview"
        description="Server inventory, update readiness, and anything that needs attention."
        action={
          <Pill tone={connectionStatus === 'connected' ? 'good' : 'warn'}>
            {connectionStatus === 'connected' ? 'Connected' : connectionStatus === 'loading' ? 'Loading…' : 'Unavailable'}
          </Pill>
        }
      />

      {state.errors.length ? (
        <div className="alert alert-warning mb-4 rounded-box py-3 text-sm">
          <AlertTriangle size={17} />
          <div>{state.errors.join(' · ')}</div>
        </div>
      ) : null}

      <section className="mb-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <Metric
          label="Server"
          value={status.server_state === 'unknown' ? 'Unknown' : status.server_state}
          detail="Crafty control arrives before Apply"
        />
        <Metric
          label="Updates"
          value={availableUpdates}
          detail={updates.summary.safe + ' safe · ' + updates.summary.review + ' review'}
          tone={availableUpdates > 0 ? 'warn' : 'good'}
        />
        <Metric
          label="Needs attention"
          value={diagnostics.summary.actionable}
          detail={diagnostics.summary.blocking + ' blocking'}
          tone={diagnostics.summary.actionable > 0 ? 'warn' : 'good'}
        />
        <Metric
          label="Managed"
          value={status.managed}
          detail={status.unmanaged + ' explicitly unmanaged'}
        />
      </section>

      <section className="grid gap-4 xl:grid-cols-2">
        <article className="panel">
          <div className="panel-header">
            <div>
              <div className="section-label">Inbox</div>
              <h2 className="mt-0.5 text-sm font-semibold">Needs attention</h2>
            </div>
            <Link href="/mods" className="btn btn-ghost btn-xs gap-1">
              Mods <ArrowRight size={13} />
            </Link>
          </div>
          {diagnostics.findings.length ? (
            <div>
              {diagnostics.findings.slice(0, 6).map((finding, index) => (
                <div className="data-row" key={finding.code + ':' + (finding.path ?? '') + ':' + index}>
                  <AlertTriangle size={15} className="shrink-0 text-base-content/35" />
                  <div className="min-w-0 flex-1">
                    <div className="truncate text-sm font-medium">{finding.mod ?? finding.code.replaceAll('_', ' ')}</div>
                    <div className="mt-0.5 text-xs text-base-content/45">{finding.message}</div>
                  </div>
                  <Pill tone={findingTone(finding.level)}>{finding.level}</Pill>
                </div>
              ))}
            </div>
          ) : (
            <div className="p-4">
              <EmptyState title="Nothing needs attention">
                Current inventory and accepted management state agree.
              </EmptyState>
            </div>
          )}
        </article>

        <article className="panel">
          <div className="panel-header">
            <div>
              <div className="section-label">Updates</div>
              <h2 className="mt-0.5 text-sm font-semibold">Available candidates</h2>
            </div>
            <Link href="/updates" className="btn btn-ghost btn-xs gap-1">
              Review <ArrowRight size={13} />
            </Link>
          </div>
          {availableUpdates ? (
            <div>
              {updates.candidates
                .filter((candidate) => candidate.classification === 'safe' || candidate.classification === 'review')
                .slice(0, 6)
                .map((candidate) => (
                  <div className="data-row" key={candidate.key}>
                    <PackageCheck size={15} className="shrink-0 text-base-content/35" />
                    <div className="min-w-0 flex-1">
                      <div className="truncate text-sm font-medium">{candidate.name}</div>
                      <div className="mt-0.5 text-xs text-base-content/45">
                        {candidate.installed.number || 'installed'} → {candidate.target?.number ?? 'unknown'}
                      </div>
                    </div>
                    <Pill tone={candidate.classification === 'safe' ? 'good' : 'warn'}>
                      {candidate.classification}
                    </Pill>
                  </div>
                ))}
            </div>
          ) : (
            <div className="p-4">
              <EmptyState title="No actionable updates">
                FPBPack did not find any safe or review candidates.
              </EmptyState>
            </div>
          )}
        </article>
      </section>

      <section className="panel mt-4">
        <div className="panel-header">
          <div className="flex items-center gap-2">
            <Server size={16} className="text-base-content/40" />
            <div>
              <div className="section-label">Inventory</div>
              <h2 className="mt-0.5 text-sm font-semibold">{status.mods} installed JARs</h2>
            </div>
          </div>
          <Pill tone="blue">Read only</Pill>
        </div>
        <div className="grid divide-y divide-base-300 sm:grid-cols-3 sm:divide-x sm:divide-y-0">
          {[
            [status.managed, 'managed artifacts'],
            [status.unmanaged, 'unmanaged artifacts'],
            [diagnostics.summary.blocking, 'blocking diagnostics'],
          ].map(([value, label]) => (
            <div className="px-4 py-3" key={String(label)}>
              <div className="text-lg font-semibold tabular-nums">{value}</div>
              <div className="text-xs text-base-content/45">{label}</div>
            </div>
          ))}
        </div>
      </section>
    </>
  );
}
