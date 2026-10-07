'use client';

import {useState} from 'react';
import {useRouter} from 'next/navigation';
import {ArrowRight, PackageMinus, PackagePlus, RefreshCw, RotateCcw, Trash2} from 'lucide-react';
import {PageHeader, Pill, formatDate} from '@/components/ui';
import {useManagement} from '@/components/management-provider';
import {api} from '@/lib/api';
import type {PendingChanges, UpdatePlan} from '@/lib/management';

function actionLabel(action: string) {
  if (action === 'install') return 'Add';
  if (action === 'remove') return 'Remove';
  if (action === 'version') return 'Change version';
  if (action === 'update') return 'Update';
  if (action === 'placement') return 'Move';
  return action;
}

function actionTone(action: string): 'good' | 'warn' | 'bad' | 'neutral' {
  if (action === 'install') return 'good';
  if (action === 'remove') return 'bad';
  if (action === 'version' || action === 'update' || action === 'placement') return 'warn';
  return 'neutral';
}

export default function PendingChangesPage() {
  const {state, reload} = useManagement();
  const router = useRouter();
  const [busyID, setBusyID] = useState<string | null>(null);
  const [discarding, setDiscarding] = useState(false);
  const [reviewing, setReviewing] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const removeChange = async (id: string) => {
    setBusyID(id);
    setError(null);
    try {
      await api<PendingChanges>('/api/pending-changes/' + encodeURIComponent(id), {
        method: 'DELETE',
      });
      await reload({silent: true});
    } catch (value: unknown) {
      setError(value instanceof Error ? value.message : String(value));
    } finally {
      setBusyID(null);
    }
  };

  const discard = async () => {
    if (
      state.pending.changes.length > 0 &&
      !window.confirm(
        'Discard all pending changes? No live mod files have been changed yet.',
      )
    ) {
      return;
    }
    setDiscarding(true);
    setError(null);
    try {
      await api<PendingChanges>('/api/pending-changes', {method: 'DELETE'});
      await reload({silent: true});
    } catch (value: unknown) {
      setError(value instanceof Error ? value.message : String(value));
    } finally {
      setDiscarding(false);
    }
  };

  const review = async () => {
    setReviewing(true);
    setError(null);
    try {
      const reviewed = await api<UpdatePlan>('/api/pending-changes/review', {
        method: 'POST',
      });
      router.push('/review?id=' + encodeURIComponent(reviewed.id));
      void reload({silent: true});
    } catch (value: unknown) {
      setError(value instanceof Error ? value.message : String(value));
    } finally {
      setReviewing(false);
    }
  };

  return (
    <>
      <PageHeader
        eyebrow="Pending changes"
        title="Changes waiting to be applied"
        description="Add, remove, update, and change versions across the app. Nothing here touches live mod files until you review and apply the complete set."
        action={
          state.pending.changes.length ? (
            <button
              className="btn btn-sm btn-ghost"
              type="button"
              disabled={discarding || reviewing}
              onClick={() => void discard()}
            >
              {discarding ? (
                <span className="loading loading-spinner loading-xs" />
              ) : (
                <RotateCcw size={14} />
              )}
              Discard pending changes
            </button>
          ) : null
        }
      />

      {error ? (
        <div className="alert alert-error mb-4 rounded-box py-3 text-sm">{error}</div>
      ) : null}

      {state.pending.changes.length === 0 ? (
        <section className="panel">
          <div className="p-10 text-center">
            <div className="mx-auto grid size-10 place-items-center rounded-full bg-base-200">
              <RefreshCw size={17} className="text-base-content/40" />
            </div>
            <h2 className="mt-3 text-sm font-semibold">No pending changes</h2>
            <p className="mx-auto mt-1 max-w-md text-xs leading-5 text-base-content/45">
              Select updates, add mods, change versions, or select mods for removal. Those
              actions accumulate here until you decide to review and apply them.
            </p>
          </div>
        </section>
      ) : (
        <>
          <section className="panel overflow-hidden">
            <div className="panel-header">
              <div>
                <div className="section-label">Current transaction</div>
                <h2 className="mt-0.5 text-sm font-semibold">
                  {state.pending.changes.length} pending change
                  {state.pending.changes.length === 1 ? '' : 's'}
                </h2>
              </div>
              <div className="flex items-center gap-2">
                {state.pending.reviewed_plan_id ? (
                  <Pill tone="good">reviewed</Pill>
                ) : (
                  <Pill tone="warn">editable</Pill>
                )}
                {state.pending.updated_at ? (
                  <span className="text-xs text-base-content/35">
                    {formatDate(state.pending.updated_at)}
                  </span>
                ) : null}
              </div>
            </div>

            <div className="divide-y divide-base-300">
              {state.pending.changes.map((change) => (
                <div
                  key={change.id}
                  className="flex flex-col gap-3 px-4 py-3 sm:flex-row sm:items-center"
                >
                  <div className="grid size-8 shrink-0 place-items-center rounded-md border border-base-300 bg-base-200">
                    {change.action === 'remove' ? (
                      <PackageMinus size={14} />
                    ) : (
                      <PackagePlus size={14} />
                    )}
                  </div>
                  <div className="min-w-0 flex-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="font-medium">{change.name}</span>
                      <Pill tone={actionTone(change.action)}>
                        {actionLabel(change.action)}
                      </Pill>
                    </div>
                    <div className="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-xs text-base-content/45">
                      {change.action === 'remove' ? (
                        <span>{change.installed_version || 'installed'} → removed</span>
                      ) : change.action === 'placement' ? (
                        <span>
                          Move to{' '}
                          {change.placement === 'client'
                            ? 'AutoModpack/' + (change.automodpack_group || 'main')
                            : 'server/common'}
                        </span>
                      ) : (
                        <span>
                          {change.installed_version || 'not installed'} →{' '}
                          {change.target_version || 'selected version'}
                        </span>
                      )}
                      {change.provider ? <span>{change.provider}</span> : null}
                      {change.placement ? (
                        <span>
                          {change.placement === 'client'
                            ? 'AutoModpack/' + (change.automodpack_group || 'main')
                            : 'server/common'}
                        </span>
                      ) : null}
                    </div>
                    {change.path ? (
                      <div className="mono mt-1 truncate text-[0.68rem] text-base-content/30">
                        {change.path}
                      </div>
                    ) : null}
                  </div>
                  <button
                    className="btn btn-sm btn-ghost btn-square"
                    type="button"
                    disabled={busyID !== null || reviewing || discarding}
                    onClick={() => void removeChange(change.id)}
                    aria-label={'Remove ' + change.name + ' from pending changes'}
                    title="Remove from pending changes"
                  >
                    {busyID === change.id ? (
                      <span className="loading loading-spinner loading-xs" />
                    ) : (
                      <Trash2 size={14} />
                    )}
                  </button>
                </div>
              ))}
            </div>
          </section>

          <div className="mt-4 flex flex-col gap-3 rounded-box border border-base-300 bg-base-100 px-4 py-4 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <div className="text-sm font-medium">
                {reviewing ? 'Preparing the review…' : 'Ready to review the complete set?'}
              </div>
              <div className="mt-0.5 text-xs text-base-content/45" aria-live="polite">
                {reviewing
                  ? 'Resolving dependency metadata, verifying exact artifacts, and preparing the restore point. Large change sets can take a moment.'
                  : 'FPBPack will resolve dependencies, verify exact artifacts, and prepare the restore point before Apply becomes available.'}
              </div>
            </div>
            <button
              className="btn btn-primary"
              type="button"
              disabled={reviewing || discarding}
              onClick={() => void review()}
            >
              {reviewing ? (
                <span className="loading loading-spinner loading-xs" />
              ) : (
                <ArrowRight size={14} />
              )}
              {reviewing ? 'Preparing review…' : 'Review pending changes'}
            </button>
          </div>
        </>
      )}
    </>
  );
}
