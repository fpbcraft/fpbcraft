'use client';

import {useCallback, useEffect, useMemo, useRef, useState} from 'react';
import {
  ArrowDownToLine,
  ClipboardCopy,
  Download,
  Pause,
  Play,
  RefreshCw,
  ScrollText,
  Search,
} from 'lucide-react';
import {useManagement} from '@/components/management-provider';
import {EmptyState, PageHeader} from '@/components/ui';
import {api} from '@/lib/api';
import type {RuntimeLogEntry} from '@/lib/management';

function formatEntry(entry: RuntimeLogEntry): string {
  const time = new Date(entry.time).toLocaleString();
  return time + '  [' + entry.level.toUpperCase() + '] [' + entry.area + '] ' + entry.message;
}

function levelColor(level: string): string {
  if (level === 'error') return 'text-error';
  if (level === 'warn' || level === 'warning') return 'text-warning';
  return 'text-neutral-content/80';
}

export default function LogsPage() {
  const {state} = useManagement();
  const [entries, setEntries] = useState<RuntimeLogEntry[]>([]);
  const [level, setLevel] = useState('all');
  const [area, setArea] = useState('all');
  const [query, setQuery] = useState('');
  const [live, setLive] = useState(true);
  const [follow, setFollow] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [copied, setCopied] = useState(false);
  const inFlight = useRef(false);
  const consoleRef = useRef<HTMLDivElement>(null);

  const load = useCallback(async (silent = false) => {
    if (inFlight.current) return;
    inFlight.current = true;
    if (!silent) setLoading(true);
    try {
      const response = await api<{entries: RuntimeLogEntry[]}>('/api/logs?limit=1000');
      setEntries(Array.isArray(response.entries) ? response.entries : []);
      setError(null);
    } catch (value: unknown) {
      setError(value instanceof Error ? value.message : String(value));
    } finally {
      inFlight.current = false;
      if (!silent) setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
    if (!live) return;
    const timer = window.setInterval(() => void load(true), 1500);
    return () => window.clearInterval(timer);
  }, [load, live]);

  const areas = useMemo(
    () => [...new Set(entries.map((entry) => entry.area).filter(Boolean))].sort(),
    [entries],
  );

  const filtered = useMemo(() => {
    const search = query.trim().toLowerCase();
    // The API returns newest first; a console should read in execution order.
    return entries
      .filter(
        (entry) =>
          (level === 'all' || entry.level === level) &&
          (area === 'all' || entry.area === area) &&
          (!search ||
            entry.message.toLowerCase().includes(search) ||
            entry.area.toLowerCase().includes(search)),
      )
      .reverse();
  }, [entries, level, area, query]);

  useEffect(() => {
    if (follow && consoleRef.current) {
      consoleRef.current.scrollTop = consoleRef.current.scrollHeight;
    }
  }, [filtered, follow]);

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(filtered.map(formatEntry).join('\n'));
      setCopied(true);
      window.setTimeout(() => setCopied(false), 2000);
    } catch {
      setError('Unable to copy the log. Use the download option instead.');
    }
  };

  const download = () => {
    const content = filtered.map(formatEntry).join('\n') + '\n';
    const blob = new Blob([content], {type: 'text/plain;charset=utf-8'});
    const href = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = href;
    link.download = 'fpbpack-execution.log';
    link.click();
    window.setTimeout(() => URL.revokeObjectURL(href), 1000);
  };

  return (
    <>
      <PageHeader
        eyebrow="Operations"
        title="Execution console"
        description="Live, step-by-step FPBPack activity: scans, provider checks, file changes, backups, installers and AutoModpack console output. The latest 1,000 entries are held in memory until restart."
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
              <h2 className="mt-0.5 text-sm font-semibold">
                {filtered.length} lines
                {state.status.refresh?.refreshing
                  ? ' · ' + (state.status.refresh.message || 'Refresh running')
                  : ''}
              </h2>
            </div>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <button
              className="btn btn-xs btn-outline"
              type="button"
              onClick={() => setLive((value) => !value)}
              aria-pressed={live}
            >
              {live ? <Pause size={13} /> : <Play size={13} />}
              {live ? 'Pause updates' : 'Resume live'}
            </button>
            <button className="btn btn-xs btn-outline" type="button" onClick={() => void copy()}>
              <ClipboardCopy size={13} /> {copied ? 'Copied' : 'Copy'}
            </button>
            <button className="btn btn-xs btn-outline" type="button" onClick={download}>
              <Download size={13} /> Download
            </button>
          </div>
        </div>

        <div className="flex flex-wrap gap-2 border-b border-base-300 p-3">
          <label className="input input-sm input-bordered flex min-w-36 flex-1 items-center gap-2">
            <Search size={14} className="text-base-content/45" />
            <input
              className="grow"
              placeholder="Search console output"
              aria-label="Search console output"
              value={query}
              onChange={(event) => setQuery(event.target.value)}
            />
          </label>
          <select
            className="select select-sm select-bordered"
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
            className="select select-sm select-bordered"
            value={area}
            onChange={(event) => setArea(event.target.value)}
            aria-label="Log area"
          >
            <option value="all">All operations</option>
            {areas.map((value) => (
              <option key={value} value={value}>{value}</option>
            ))}
          </select>
          <button
            className="btn btn-sm btn-ghost"
            type="button"
            onClick={() => {
              setFollow(true);
              if (consoleRef.current) {
                consoleRef.current.scrollTop = consoleRef.current.scrollHeight;
              }
            }}
            aria-pressed={follow}
          >
            <ArrowDownToLine size={14} /> {follow ? 'Following' : 'Jump to latest'}
          </button>
        </div>

        <div
          ref={consoleRef}
          className="max-h-[min(70vh,900px)] min-h-[300px] overflow-auto bg-neutral p-4 font-mono text-xs leading-relaxed text-neutral-content"
          role="log"
          aria-label="FPBPack execution output"
          aria-live="off"
          onScroll={(event) => {
            const node = event.currentTarget;
            if (node.scrollHeight - node.scrollTop - node.clientHeight > 64) {
              setFollow(false);
            }
          }}
        >
          {loading ? (
            <div className="text-neutral-content/60">Connecting to FPBPack log…</div>
          ) : filtered.length ? (
            <div className="min-w-max space-y-0.5">
              {filtered.map((entry) => (
                <div className="flex gap-3 whitespace-pre-wrap break-words" key={entry.id}>
                  <span className="shrink-0 select-none text-neutral-content/45">
                    {new Date(entry.time).toLocaleTimeString()}
                  </span>
                  <span className="shrink-0 select-none text-neutral-content/55">
                    [{entry.area}]
                  </span>
                  <span className={levelColor(entry.level)}>{entry.message}</span>
                </div>
              ))}
            </div>
          ) : (
            <EmptyState title="No matching output">
              Start an operation or adjust the filters to see its individual steps.
            </EmptyState>
          )}
        </div>
      </section>
    </>
  );
}
