'use client';

import Link from 'next/link';
import {useEffect, useState} from 'react';
import {History as HistoryIcon} from 'lucide-react';
import {EmptyState, PageHeader, Pill, formatDate} from '@/components/ui';
import {api} from '@/lib/api';
import type {HistoryEvent} from '@/lib/management';

export default function HistoryPage() {
  const [events, setEvents] = useState<HistoryEvent[]>([]);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api<{events: HistoryEvent[]}>('/api/history')
      .then((response) => setEvents(response.events))
      .catch((value: unknown) => setError(value instanceof Error ? value.message : String(value)));
  }, []);

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
                {event.plan_id ? (
                  <Link className="btn btn-xs btn-ghost" href={'/review?id=' + encodeURIComponent(event.plan_id)}>
                    Review
                  </Link>
                ) : null}
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
    </>
  );
}
