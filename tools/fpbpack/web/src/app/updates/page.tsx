'use client';

import {useMemo, useState} from 'react';
import {useRouter} from 'next/navigation';
import {ExternalLink, RefreshCw} from 'lucide-react';
import {PageHeader, Pill, formatDate} from '@/components/ui';
import {useManagement} from '@/components/management-provider';
import type {UpdateCandidate, UpdateClassification, UpdatePlan} from '@/lib/management';
import {api} from '@/lib/api';

const groups: Array<{
  classification: UpdateClassification;
  title: string;
  tone: 'good' | 'warn' | 'bad' | 'neutral';
}> = [
  {classification: 'safe', title: 'Safe', tone: 'good'},
  {classification: 'review', title: 'Review', tone: 'warn'},
  {classification: 'blocked', title: 'Blocked', tone: 'bad'},
  {classification: 'ignored', title: 'Ignored', tone: 'neutral'},
];

function CandidateRow({
  candidate,
  selected,
  onToggle,
}: {
  candidate: UpdateCandidate;
  selected: boolean;
  onToggle: () => void;
}) {
  const selectable = candidate.classification === 'safe' || candidate.classification === 'review';
  const changelogs = candidate.changelogs ?? [];

  return (
    <div className="border-t border-base-300 first:border-t-0">
      <div className="flex min-h-14 items-center gap-3 px-4 py-3">
        <input
          type="checkbox"
          className="checkbox checkbox-sm"
          checked={selected}
          disabled={!selectable}
          onChange={onToggle}
          aria-label={'Select ' + candidate.name}
        />

        {candidate.icon_url ? (
          <img
            src={candidate.icon_url}
            alt=""
            className="size-8 shrink-0 rounded-md border border-base-300 bg-base-200 object-cover"
          />
        ) : (
          <div className="grid size-8 shrink-0 place-items-center rounded-md border border-base-300 bg-base-200 text-xs font-semibold text-base-content/45">
            {candidate.name.slice(0, 1).toUpperCase()}
          </div>
        )}

        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <span className="truncate text-sm font-medium">{candidate.name}</span>
            {candidate.target?.channel && candidate.target.channel !== 'release' ? (
              <Pill tone="warn">{candidate.target.channel}</Pill>
            ) : null}
          </div>

          <div className="mt-0.5 flex flex-wrap gap-x-3 gap-y-1 text-xs text-base-content/45">
            <span>
              {candidate.installed.number || candidate.installed.name || 'installed'} →{' '}
              {candidate.target?.number ?? '—'}
            </span>
            <span>{candidate.provider}</span>
            {candidate.target?.published_at ? <span>{formatDate(candidate.target.published_at)}</span> : null}
          </div>

          {candidate.reasons?.length ? (
            <div className="mt-1 text-xs text-base-content/50">{candidate.reasons[0].message}</div>
          ) : null}
        </div>

        <div className="flex shrink-0 items-center gap-1">
          {candidate.dependencies?.some(
            (dependency) => dependency.action !== 'none' && dependency.action !== 'satisfied',
          ) ? (
            <Pill tone="blue">dependency</Pill>
          ) : null}

          {candidate.project_url ? (
            <a
              className="btn btn-xs btn-ghost"
              href={candidate.project_url}
              target="_blank"
              rel="noreferrer"
              title={'Open ' + candidate.name + ' on ' + candidate.provider}
            >
              {candidate.provider === 'modrinth'
                ? 'Modrinth'
                : candidate.provider === 'curseforge'
                  ? 'CurseForge'
                  : 'Project'}
              <ExternalLink size={12} />
            </a>
          ) : null}
        </div>
      </div>

      {candidate.target ? (
        <details className="group border-t border-base-300/60 bg-base-200/25">
          <summary className="flex cursor-pointer list-none items-center justify-between gap-3 px-4 py-2 text-xs font-medium text-base-content/60 hover:text-base-content">
            <span>
              What changed
              {changelogs.length > 1 ? ' · ' + changelogs.length + ' releases' : ''}
            </span>
            <span className="text-[0.68rem] font-normal text-base-content/35 group-open:hidden">Show</span>
            <span className="hidden text-[0.68rem] font-normal text-base-content/35 group-open:inline">Hide</span>
          </summary>

          <div className="border-t border-base-300/60 px-4 py-3">
            {changelogs.length ? (
              <div className="space-y-4">
                {changelogs.map((entry) => (
                  <section key={entry.id}>
                    <div className="flex flex-wrap items-baseline gap-x-2 gap-y-1">
                      <h3 className="text-xs font-semibold">
                        {entry.number || entry.name || entry.id}
                      </h3>
                      {entry.name && entry.name !== entry.number ? (
                        <span className="text-xs text-base-content/45">{entry.name}</span>
                      ) : null}
                      {entry.published_at ? (
                        <span className="text-[0.68rem] text-base-content/35">
                          {formatDate(entry.published_at)}
                        </span>
                      ) : null}
                    </div>
                    {entry.body ? (
                      <div className="mt-1 whitespace-pre-wrap text-xs leading-5 text-base-content/65">
                        {entry.body}
                      </div>
                    ) : (
                      <div className="mt-1 text-xs italic text-base-content/35">
                        No changelog was provided for this release.
                      </div>
                    )}
                  </section>
                ))}
              </div>
            ) : (
              <div className="text-xs text-base-content/40">
                No changelog data is available from {candidate.provider} for this update.
              </div>
            )}
          </div>
        </details>
      ) : null}
    </div>
  );
}

export default function UpdatesPage() {
  const {state, checkUpdates} = useManagement();
  const router = useRouter();
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [checking, setChecking] = useState(false);
  const [creatingPlan, setCreatingPlan] = useState(false);
  const [planError, setPlanError] = useState<string | null>(null);

  const selectableSafe = useMemo(
    () => state.updates.candidates.filter((candidate) => candidate.classification === 'safe'),
    [state.updates.candidates],
  );

  const toggle = (key: string) => {
    setSelected((current) => {
      const next = new Set(current);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  };

  const check = async () => {
    setChecking(true);
    try {
      await checkUpdates();
      setSelected(new Set());
    } finally {
      setChecking(false);
    }
  };

  const createPlan = async () => {
    if (selected.size === 0) return;
    setCreatingPlan(true);
    setPlanError(null);
    try {
      const plan = await api<UpdatePlan>('/api/plans', {
        method: 'POST',
        body: JSON.stringify({candidate_keys: [...selected]}),
      });
      router.push('/review?id=' + encodeURIComponent(plan.id));
    } catch (error: unknown) {
      setPlanError(error instanceof Error ? error.message : String(error));
    } finally {
      setCreatingPlan(false);
    }
  };

  return (
    <>
      <PageHeader
        eyebrow="Updates"
        title="Review candidates"
        description="Choose update candidates. Nothing on this page can modify live mod files."
        action={
          <div className="flex gap-2">
            <button className="btn btn-sm btn-ghost" type="button" onClick={() => void check()} disabled={checking}>
              <RefreshCw size={14} className={checking ? 'animate-spin' : ''} />
              Check updates
            </button>
            <button
              className="btn btn-sm btn-primary"
              type="button"
              disabled={selectableSafe.length === 0}
              onClick={() => setSelected(new Set(selectableSafe.map((candidate) => candidate.key)))}
            >
              Select safe
            </button>
          </div>
        }
      />

      {state.diagnostics.summary.blocking > 0 ? (
        <div className="alert alert-warning mb-4 rounded-box py-3 text-sm">
          <span>
            {state.diagnostics.summary.blocking} blocking diagnostic
            {state.diagnostics.summary.blocking === 1 ? '' : 's'} will prevent plan readiness.
          </span>
        </div>
      ) : null}

      {planError ? <div className="alert alert-error mb-4 rounded-box py-3 text-sm">{planError}</div> : null}

      <div className="mb-4 flex items-center justify-between rounded-box border border-base-300 bg-base-100 px-4 py-3">
        <div>
          <div className="text-sm font-medium">{selected.size} selected</div>
          <div className="text-xs text-base-content/45">Safe and review candidates can be included in a plan.</div>
        </div>
        <button
          className="btn btn-sm btn-primary"
          type="button"
          disabled={selected.size === 0 || creatingPlan}
          onClick={() => void createPlan()}
        >
          {creatingPlan ? <span className="loading loading-spinner loading-xs" /> : null}
          Review plan
        </button>
      </div>

      <div className="grid gap-4 xl:grid-cols-2">
        {groups.map((group) => {
          const candidates = state.updates.candidates.filter(
            (candidate) => candidate.classification === group.classification,
          );
          return (
            <section className="panel min-w-0" key={group.classification}>
              <div className="panel-header">
                <div>
                  <div className="section-label">{group.classification}</div>
                  <h2 className="mt-0.5 text-sm font-semibold">{group.title}</h2>
                </div>
                <Pill tone={group.tone}>{candidates.length}</Pill>
              </div>
              {candidates.length ? (
                <div>
                  {candidates.map((candidate) => (
                    <CandidateRow
                      key={candidate.key}
                      candidate={candidate}
                      selected={selected.has(candidate.key)}
                      onToggle={() => toggle(candidate.key)}
                    />
                  ))}
                </div>
              ) : (
                <div className="px-4 py-7 text-center text-xs text-base-content/40">No candidates</div>
              )}
            </section>
          );
        })}
      </div>
    </>
  );
}
