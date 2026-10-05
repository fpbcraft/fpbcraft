'use client';

import Link from 'next/link';
import {useEffect, useState} from 'react';
import {
  AlertTriangle,
  ArrowLeft,
  FileArchive,
  HardDriveDownload,
  ShieldCheck,
} from 'lucide-react';
import {PageHeader, Pill, formatDate} from '@/components/ui';
import {api} from '@/lib/api';
import type {UpdatePlan} from '@/lib/management';

export default function ReviewPage() {
  const [plan, setPlan] = useState<UpdatePlan | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const id = new URLSearchParams(window.location.search).get('id');
    if (!id) {
      setError('No plan ID was provided.');
      return;
    }
    api<UpdatePlan>('/api/plans/' + encodeURIComponent(id))
      .then(setPlan)
      .catch((value: unknown) => setError(value instanceof Error ? value.message : String(value)));
  }, []);

  if (error) {
    return (
      <>
        <PageHeader eyebrow="Plan" title="Review update plan" description="The requested plan could not be loaded." />
        <div className="alert alert-error rounded-box text-sm">{error}</div>
      </>
    );
  }

  if (!plan) {
    return (
      <>
        <PageHeader eyebrow="Plan" title="Review update plan" description="Loading persisted plan…" />
        <div className="flex justify-center py-16"><span className="loading loading-spinner loading-md" /></div>
      </>
    );
  }

  const ready = plan.status === 'ready';

  return (
    <>
      <div className="mb-3">
        <Link href="/updates" className="btn btn-ghost btn-xs gap-1 px-0">
          <ArrowLeft size={13} /> Back to updates
        </Link>
      </div>
      <PageHeader
        eyebrow="Plan & Protect"
        title="Review update plan"
        description={'Persisted ' + formatDate(plan.created_at) + ' · ' + plan.id}
        action={<Pill tone={ready ? 'good' : 'bad'}>{plan.status}</Pill>}
      />

      <section className="mb-4 grid gap-3 sm:grid-cols-3">
        <div className="surface rounded-box px-4 py-3">
          <div className="section-label">Changes</div>
          <div className="mt-1 text-xl font-semibold">{plan.changes.length}</div>
          <div className="mt-1 text-xs text-base-content/45">managed artifacts</div>
        </div>
        <div className="surface rounded-box px-4 py-3">
          <div className="section-label">Protection</div>
          <div className="mt-2 flex items-center gap-2 text-sm">
            <FileArchive size={15} className="text-base-content/40" />
            {plan.requires_backup ? 'Restore point required' : 'No backup required'}
          </div>
        </div>
        <div className="surface rounded-box px-4 py-3">
          <div className="section-label">Server</div>
          <div className="mt-2 flex items-center gap-2 text-sm">
            <ShieldCheck size={15} className="text-base-content/40" />
            {plan.requires_server_stop ? 'Must be stopped before Apply' : 'No stop required'}
          </div>
        </div>
      </section>

      {plan.blockers?.length ? (
        <section className="panel mb-4 border-error/30">
          <div className="panel-header">
            <div className="flex items-center gap-2">
              <AlertTriangle size={16} className="text-error" />
              <div>
                <div className="section-label">Blocked</div>
                <h2 className="mt-0.5 text-sm font-semibold">{plan.blockers.length} issue{plan.blockers.length === 1 ? '' : 's'} must be resolved</h2>
              </div>
            </div>
            <Pill tone="bad">{plan.blockers.length}</Pill>
          </div>
          <div>
            {plan.blockers.map((blocker, index) => (
              <div className="data-row" key={blocker.code + ':' + index}>
                <div className="min-w-0">
                  <div className="text-xs font-medium">{blocker.code.replaceAll('_', ' ')}</div>
                  <div className="mt-0.5 text-xs text-base-content/50">{blocker.message}</div>
                </div>
              </div>
            ))}
          </div>
        </section>
      ) : null}

      {plan.warnings?.length ? (
        <section className="panel mb-4 border-warning/25">
          <div className="panel-header">
            <div>
              <div className="section-label">Review notes</div>
              <h2 className="mt-0.5 text-sm font-semibold">{plan.warnings.length} warning{plan.warnings.length === 1 ? '' : 's'}</h2>
            </div>
            <Pill tone="warn">{plan.warnings.length}</Pill>
          </div>
          <div>
            {plan.warnings.map((warning, index) => (
              <div className="data-row text-xs text-base-content/55" key={warning.code + ':' + index}>
                {warning.message}
              </div>
            ))}
          </div>
        </section>
      ) : null}

      <section className="panel overflow-hidden">
        <div className="panel-header">
          <div>
            <div className="section-label">Exact plan</div>
            <h2 className="mt-0.5 text-sm font-semibold">Artifact changes</h2>
          </div>
          <Pill tone="neutral">{plan.changes.length}</Pill>
        </div>
        <div className="divide-y divide-base-300">
          {plan.changes.map((change) => (
            <details className="group" key={change.candidate_key}>
              <summary className="flex cursor-pointer list-none items-center gap-3 px-4 py-3 hover:bg-base-300/20">
                <HardDriveDownload size={16} className="shrink-0 text-base-content/35" />
                <div className="min-w-0 flex-1">
                  <div className="text-sm font-medium">{change.name}</div>
                  <div className="mt-0.5 text-xs text-base-content/45">
                    {change.installed.number || change.installed.name || 'installed'} → {change.target.number || change.target.name || change.target.id}
                  </div>
                </div>
                <Pill tone={change.classification === 'safe' ? 'good' : 'warn'}>{change.classification}</Pill>
              </summary>
              <div className="border-t border-base-300 bg-base-200/40 px-4 py-3">
                <dl className="grid gap-2 text-xs sm:grid-cols-[130px_minmax(0,1fr)]">
                  <dt className="text-base-content/40">Target file</dt>
                  <dd className="mono break-all">{change.artifact.filename}</dd>
                  <dt className="text-base-content/40">Target SHA-512</dt>
                  <dd className="mono break-all">{change.artifact.sha512}</dd>
                  <dt className="text-base-content/40">Provider</dt>
                  <dd>{change.artifact.provider} · {change.artifact.project_id} · {change.artifact.version_id}</dd>
                  {change.operations.map((operation, index) => (
                    <div className="contents" key={index}>
                      <dt className="text-base-content/40">{operation.action}</dt>
                      <dd className="mono break-all">{operation.current_path || '—'} → {operation.target_path}</dd>
                    </div>
                  ))}
                </dl>
              </div>
            </details>
          ))}
        </div>
      </section>

      <div className="sticky bottom-0 mt-4 flex items-center justify-between gap-4 border-t border-base-300 bg-base-200/95 py-3 backdrop-blur">
        <div className="text-xs text-base-content/45">
          {ready ? 'Plan is structurally ready. Artifact prefetch/verification is still required before Apply.' : 'This plan cannot proceed while blockers remain.'}
        </div>
        <button className="btn btn-sm btn-primary" type="button" disabled>
          Apply unavailable until Slice 3
        </button>
      </div>
    </>
  );
}
