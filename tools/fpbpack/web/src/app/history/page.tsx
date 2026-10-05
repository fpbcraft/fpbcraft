'use client';

import Link from 'next/link';
import {useEffect, useState} from 'react';
import {History as HistoryIcon, Play, RotateCcw, Square, X} from 'lucide-react';
import {EmptyState, PageHeader, Pill, formatDate} from '@/components/ui';
import {api} from '@/lib/api';
import {useManagement} from '@/components/management-provider';
import type {HistoryEvent} from '@/lib/management';

export default function HistoryPage() {
  const {state, reload} = useManagement();
  const [events, setEvents] = useState<HistoryEvent[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [confirmRestore, setConfirmRestore] = useState<HistoryEvent | null>(null);
  const [restoring, setRestoring] = useState(false);
  const [serverBusy, setServerBusy] = useState(false);

  const loadHistory = async () => {
    const response = await api<{events: HistoryEvent[]}>('/api/history');
    setEvents(response.events);
  };

  useEffect(() => {
    loadHistory().catch((value: unknown) =>
      setError(value instanceof Error ? value.message : String(value)),
    );
  }, []);

  const serverAction = async (action: 'start' | 'stop') => {
    setServerBusy(true);
    setError(null);
    try {
      await api('/api/server/' + action, {method: 'POST'});
      await reload({silent: true});
    } catch (value: unknown) {
      setError(value instanceof Error ? value.message : String(value));
    } finally {
      setServerBusy(false);
    }
  };

  const restore = async () => {
    if (!confirmRestore?.backup_id) return;
    setRestoring(true);
    setError(null);
    try {
      await api('/api/backups/' + encodeURIComponent(confirmRestore.backup_id) + '/restore', {
        method: 'POST',
        body: JSON.stringify({confirm: true}),
      });
      setConfirmRestore(null);
      await Promise.all([loadHistory(), reload({silent: true})]);
    } catch (value: unknown) {
      setError(value instanceof Error ? value.message : String(value));
    } finally {
      setRestoring(false);
    }
  };

  return (
    <>
      <PageHeader
        eyebrow="Audit"
        title="History"
        description="Persisted plans, applies, restores, and reconciliation events."
        action={<Pill tone="neutral">{events.length} events</Pill>}
      />

      {error ? <div className="alert alert-error mb-4 rounded-box text-sm">{error}</div> : null}

      <section className="panel overflow-hidden">
        <div className="panel-header">
          <div className="flex items-center gap-2">
            <HistoryIcon size={16} className="text-base-content/40" />
            <div>
              <div className="section-label">Audit log</div>
              <h2 className="mt-0.5 text-sm font-semibold">Operations</h2>
            </div>
          </div>
        </div>
        {events.length ? (
          <div>
            {events.map((event) => (
              <div className="data-row" key={event.id + ':' + event.created_at}>
                <div className="min-w-0 flex-1">
                  <div className="text-sm font-medium">{event.summary}</div>
                  <div className="mt-0.5 flex flex-wrap gap-x-3 text-xs text-base-content/45">
                    <span>{formatDate(event.created_at)}</span>
                    <span>{event.mods} mod{event.mods === 1 ? '' : 's'}</span>
                    <span>{event.type}</span>
                  </div>
                </div>
                <Pill tone={event.status === 'ready' ? 'good' : event.status === 'blocked' ? 'bad' : 'neutral'}>{event.status}</Pill>
                <div className="flex shrink-0 gap-1">
                  {event.plan_id ? (
                    <Link className="btn btn-xs btn-ghost" href={'/review?id=' + encodeURIComponent(event.plan_id)}>
                      Review
                    </Link>
                  ) : null}
                  {event.type === 'apply' && event.status === 'success' && event.backup_id ? (
                    <button
                      className="btn btn-xs btn-outline"
                      type="button"
                      onClick={() => setConfirmRestore(event)}
                    >
                      <RotateCcw size={12} /> Restore
                    </button>
                  ) : null}
                </div>
              </div>
            ))}
          </div>
        ) : (
          <div className="p-4">
            <EmptyState title="No operations recorded yet">
              Creating an update plan records the first audit event.
            </EmptyState>
          </div>
        )}
      </section>

      {confirmRestore ? (
        <div className="modal modal-open" role="dialog" aria-modal="true" aria-label="Confirm restore">
          <div className="modal-box max-w-lg">
            <div className="flex items-start justify-between gap-3">
              <div>
                <div className="section-label">Restore</div>
                <h2 className="mt-1 text-lg font-semibold">Restore previous managed mod state?</h2>
              </div>
              <button
                className="btn btn-sm btn-ghost btn-square"
                type="button"
                onClick={() => setConfirmRestore(null)}
                aria-label="Close"
              >
                <X size={15} />
              </button>
            </div>
            <p className="mt-3 text-sm text-base-content/65">
              FPBPack will restore the exact files and accepted management state from{' '}
              <span className="mono">{confirmRestore.backup_id}</span>. Unmanaged artifacts are
              left alone. The server remains stopped afterward.
            </p>
            <div className="mt-4 rounded-box border border-base-300 bg-base-200/40 p-3 text-xs">
              Minecraft server: <strong>{state.status.server_state}</strong>
            </div>
            <div className="modal-action">
              {state.status.server_state === 'running' ? (
                <button
                  className="btn btn-sm btn-warning"
                  type="button"
                  disabled={serverBusy}
                  onClick={() => void serverAction('stop')}
                >
                  {serverBusy ? <span className="loading loading-spinner loading-xs" /> : <Square size={12} />}
                  Stop server
                </button>
              ) : null}
              {state.status.server_state === 'stopped' ? (
                <button
                  className="btn btn-sm btn-error"
                  type="button"
                  disabled={restoring}
                  onClick={() => void restore()}
                >
                  {restoring ? <span className="loading loading-spinner loading-xs" /> : <RotateCcw size={12} />}
                  Confirm restore
                </button>
              ) : null}
            </div>
          </div>
        </div>
      ) : null}

      {!confirmRestore && state.status.server_state === 'stopped' && events.some((event) => event.type === 'restore') ? (
        <div className="mt-4 flex justify-end">
          <button
            className="btn btn-sm btn-success"
            type="button"
            disabled={serverBusy}
            onClick={() => void serverAction('start')}
          >
            {serverBusy ? <span className="loading loading-spinner loading-xs" /> : <Play size={12} />}
            Start server
          </button>
        </div>
      ) : null}
    </>
  );
}
