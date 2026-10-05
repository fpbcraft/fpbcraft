'use client';

import {useCallback, useEffect, useMemo, useState} from 'react';
import {RefreshCw, ScrollText} from 'lucide-react';
import {EmptyState, PageHeader, Pill, formatDate} from '@/components/ui';
import {api} from '@/lib/api';
import type {RuntimeLogEntry} from '@/lib/management';

function logTone(level: string): 'bad' | 'warn' | 'neutral' {
  if (level === 'error') return 'bad';
  if (level === 'warn' || level === 'warning') return 'warn';
  return 'neutral';
}

export default function LogsPage() {
  const [entries, setEntries] = useState<RuntimeLogEntry[]>([]);
  const [level, setLevel] = useState('all');
  const [area, setArea] = useState('all');
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async (silent = false) => {
    if (!silent) setLoading(true);
    setError(null);
    try {
      const response = await api<{entries: RuntimeLogEntry[]}>('/api/logs?limit=500');
      setEntries(Array.isArray(response.entries) ? response.entries : []);
    } catch (value: unknown) {
      setError(value instanceof Error ? value.message : String(value));
    } finally {
      if (!silent) setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
    const timer = window.setInterval(() => void load(true), 5000);
    return () => window.clearInterval(timer);
  }, [load]);

  const areas = useMemo(
    () => [...new Set(entries.map((entry) => entry.area).filter(Boolean))].sort(),
    [entries],
  );

  const filtered = useMemo(
    () =>
      entries.filter(
        (entry) =>
          (level === 'all' || entry.level === level) &&
          (area === 'all' || entry.area === area),
      ),
    [entries, level, area],
  );

  return (
    <>
      <PageHeader
        eyebrow="Operations"
        title="Logs"
        description="Recent FPBPack runtime events. The in-memory log is bounded to the latest 500 entries and resets when the service restarts."
        action={
          <button className="btn btn-sm btn-outline" type="button" onClick={() => void load()}>
            <RefreshCw size={13} /> Refresh
          </button>
        }
      />

      {error ? <div className="alert alert-error mb-4 rounded-box py-3 text-sm">{error}</div> : null}

      <section className="panel overflow-hidden">
        <div className="panel-header gap-3">
          <div className="flex items-center gap-2">
            <ScrollText size={16} className="text-base-content/40" />
            <div>
              <div className="section-label">Runtime</div>
              <h2 className="mt-0.5 text-sm font-semibold">{filtered.length} entries</h2>
            </div>
          </div>
          <div className="flex flex-wrap gap-2">
            <select
              className="select select-xs select-bordered"
              value={level}
              onChange={(event) => setLevel(event.target.value)}
              aria-label="Log level"
            >
              <option value="all">All levels</option>
              <option value="info">Info</option>
              <option value="warn">Warnings</option>
              <option value="error">Errors</option>
            </select>
            <select
              className="select select-xs select-bordered"
              value={area}
              onChange={(event) => setArea(event.target.value)}
              aria-label="Log area"
            >
              <option value="all">All areas</option>
              {areas.map((value) => (
                <option key={value} value={value}>
                  {value}
                </option>
              ))}
            </select>
          </div>
        </div>

        {loading ? (
          <div className="grid place-items-center p-10">
            <span className="loading loading-spinner loading-sm" />
          </div>
        ) : filtered.length ? (
          <div className="divide-y divide-base-300">
            {filtered.map((entry) => (
              <div className="grid gap-2 px-4 py-3 sm:grid-cols-[10rem_5rem_7rem_minmax(0,1fr)]" key={entry.id}>
                <div className="mono text-xs text-base-content/45">{formatDate(entry.time)}</div>
                <div>
                  <Pill tone={logTone(entry.level)}>{entry.level}</Pill>
                </div>
                <div className="mono text-xs text-base-content/55">{entry.area}</div>
                <div className="break-words text-sm">{entry.message}</div>
              </div>
            ))}
          </div>
        ) : (
          <div className="p-4">
            <EmptyState title="No matching log entries">
              FPBPack has not recorded any runtime events matching these filters.
            </EmptyState>
          </div>
        )}
      </section>
    </>
  );
}
