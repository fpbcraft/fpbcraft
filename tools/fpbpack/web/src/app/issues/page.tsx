'use client';

import Link from 'next/link';
import {useEffect, useMemo, useState} from 'react';
import {AlertTriangle, CircleAlert, RefreshCw, Wrench} from 'lucide-react';
import {PageHeader, Pill} from '@/components/ui';
import {useManagement} from '@/components/management-provider';
import {api} from '@/lib/api';
import type {AutoModpackStatus, UpdatePlan} from '@/lib/management';

interface ProviderStatus {
  id: string;
  label: string;
  status: string;
  detail: string;
}

interface Issue {
  id: string;
  level: 'blocking' | 'warning';
  source: string;
  title: string;
  message: string;
  context?: string;
  href?: string;
  action: string;
  actionKind?: 'refresh-all' | 'refresh-inventory';
}

export default function IssuesPage() {
  const {state, reload, refresh, refreshInventory} = useManagement();
  const [autoModpack, setAutoModpack] = useState<AutoModpackStatus | null>(null);
  const [providers, setProviders] = useState<ProviderStatus[]>([]);
  const [reviewedPlan, setReviewedPlan] = useState<UpdatePlan | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    void api<AutoModpackStatus>('/api/automodpack').then(setAutoModpack).catch(() => undefined);
    void api<{providers: ProviderStatus[]}>('/api/providers')
      .then((response) => setProviders(response.providers))
      .catch(() => undefined);
  }, []);

  useEffect(() => {
    const id = state.pending.reviewed_plan_id;
    if (!id) {
      setReviewedPlan(null);
      return;
    }
    void api<UpdatePlan>('/api/plans/' + encodeURIComponent(id))
      .then(setReviewedPlan)
      .catch(() => setReviewedPlan(null));
  }, [state.pending.reviewed_plan_id]);

  const issues = useMemo(() => {
    const result: Issue[] = [];
    const seen = new Set<string>();
    const add = (issue: Issue) => {
      const key =
        issue.level +
        '\x00' +
        issue.title +
        '\x00' +
        issue.message +
        '\x00' +
        (issue.context ?? '');
      if (seen.has(key)) return;
      seen.add(key);
      result.push(issue);
    };

    state.errors.forEach((message, index) =>
      add({
        id: 'service:' + index,
        level: 'warning',
        source: 'FPBPack',
        title: 'Service data unavailable',
        message,
        href: '/settings',
        action: 'Open settings',
      }),
    );

    state.diagnostics.findings
      .filter((finding) => finding.level === 'blocking' || finding.level === 'warning')
      .forEach((finding, index) => {
        const moved = finding.code === 'managed_artifact_moved';
        const providerLookup =
          finding.code === 'modrinth_lookup_failed' ||
          finding.code === 'curseforge_lookup_failed';
        const refreshInventoryIssue = finding.code === 'inventory_schema_mismatch';
        const modHref = finding.path
          ? '/mods?attention=1&mod=' + encodeURIComponent(finding.path)
          : '/mods?attention=1';

        add({
          id: 'diagnostic:' + finding.code + ':' + index,
          level: finding.level === 'blocking' ? 'blocking' : 'warning',
          source: providerLookup ? 'Provider' : 'Inventory',
          title: finding.mod || finding.code.replaceAll('_', ' '),
          message: finding.message,
          context: finding.path,
          href: moved || providerLookup || refreshInventoryIssue ? undefined : modHref,
          action: moved
            ? 'Reconcile moved mods'
            : providerLookup
              ? 'Retry provider refresh'
              : refreshInventoryIssue
                ? 'Refresh inventory'
                : 'Open affected mod',
          actionKind: providerLookup
            ? 'refresh-all'
            : moved || refreshInventoryIssue
              ? 'refresh-inventory'
              : undefined,
        });
      });

    state.updates.candidates.forEach((candidate) => {
      if (candidate.classification === 'blocked') {
        add({
          id: 'update:' + candidate.key,
          level: 'blocking',
          source: 'Updates',
          title: candidate.name,
          message: candidate.reasons?.[0]?.message || 'This update candidate is blocked.',
          context: candidate.target?.filename || candidate.installed.filename,
          href: '/updates',
          action: 'Open updates',
        });
      } else if (candidate.metadata_stale || candidate.refresh_error) {
        add({
          id: 'metadata:' + candidate.key,
          level: 'warning',
          source: 'Provider metadata',
          title: candidate.name,
          message: candidate.refresh_error || 'Provider metadata is stale.',
          context: candidate.installed.filename,
          href: '/mods?attention=1&mod=' + encodeURIComponent(candidate.key),
          action: 'Open affected mod',
        });
      }
    });

    providers
      .filter((provider) => provider.status !== 'ready')
      .forEach((provider) =>
        add({
          id: 'provider:' + provider.id,
          level: 'warning',
          source: 'Provider',
          title: provider.label,
          message: provider.detail,
          href: '/settings',
          action: 'Configure provider',
        }),
      );

    autoModpack?.findings
      .filter((finding) => finding.level === 'error' || finding.level === 'warning')
      .forEach((finding, index) =>
        add({
          id: 'automodpack:' + finding.code + ':' + index,
          level: finding.level === 'error' ? 'blocking' : 'warning',
          source: 'AutoModpack',
          title: finding.group ? finding.group + ': ' + finding.code : finding.code,
          message: finding.message,
          context: finding.group,
          href: '/automodpack',
          action: 'Open AutoModpack',
        }),
      );

    const planFinding = (
      finding: NonNullable<UpdatePlan['blockers']>[number],
      level: Issue['level'],
      index: number,
    ) => {
      const change = reviewedPlan?.changes.find(
        (item) => item.candidate_key === finding.candidate_key,
      );
      add({
        id: 'plan:' + level + ':' + index,
        level,
        source: 'Pending review',
        title: finding.name || change?.name || finding.candidate_key || finding.code,
        message: finding.message,
        context:
          finding.path ||
          change?.operations[0]?.current_path ||
          change?.operations[0]?.target_path ||
          change?.artifact.filename,
        href: reviewedPlan ? '/review?id=' + encodeURIComponent(reviewedPlan.id) : '/pending',
        action: 'Open review',
      });
    };
    reviewedPlan?.blockers?.forEach((finding, index) =>
      planFinding(finding, 'blocking', index),
    );
    reviewedPlan?.warnings?.forEach((finding, index) =>
      planFinding(finding, 'warning', index),
    );

    return result.sort((left, right) => {
      if (left.level !== right.level) return left.level === 'blocking' ? -1 : 1;
      return left.title.localeCompare(right.title);
    });
  }, [state, providers, autoModpack, reviewedPlan]);

  const blocking = issues.filter((issue) => issue.level === 'blocking').length;
  const warnings = issues.length - blocking;

  const runAction = async (kind: NonNullable<Issue['actionKind']>) => {
    setBusy(true);
    setError(null);
    try {
      if (kind === 'refresh-all') {
        await refresh();
      } else {
        await refreshInventory();
      }
      await reload({silent: true});
    } catch (value: unknown) {
      setError(value instanceof Error ? value.message : String(value));
    } finally {
      setBusy(false);
    }
  };

  return (
    <>
      <PageHeader
        eyebrow="Health"
        title="Issues"
        description="Current FPBPack, inventory, provider, pending-review, and AutoModpack problems with a direct path to resolve each one."
        action={
          <button
            className="btn btn-sm btn-outline"
            type="button"
            disabled={busy || state.status.refresh?.refreshing}
            onClick={() => void refresh()}
          >
            <RefreshCw size={14} /> Refresh all
          </button>
        }
      />

      {error ? (
        <div className="alert alert-error mb-4 rounded-box py-3 text-sm">{error}</div>
      ) : null}

      <div className="mb-4 flex flex-wrap gap-2">
        <Pill tone={blocking ? 'bad' : 'good'}>{blocking} blocking</Pill>
        <Pill tone={warnings ? 'warn' : 'good'}>{warnings} warnings</Pill>
      </div>

      {issues.length === 0 ? (
        <section className="panel p-10 text-center">
          <div className="mx-auto grid size-10 place-items-center rounded-full bg-base-200">
            <Wrench size={17} className="text-base-content/35" />
          </div>
          <h2 className="mt-3 text-sm font-semibold">No current issues</h2>
          <p className="mt-1 text-xs text-base-content/45">
            Inventory, providers, pending review, and AutoModpack currently report no
            warnings or blockers.
          </p>
        </section>
      ) : (
        <section className="panel overflow-hidden">
          <div className="divide-y divide-base-300">
            {issues.map((issue) => (
              <div
                className="flex flex-col gap-3 px-4 py-3 sm:flex-row sm:items-center"
                key={issue.id}
              >
                {issue.level === 'blocking' ? (
                  <CircleAlert size={16} className="shrink-0 text-error" />
                ) : (
                  <AlertTriangle size={16} className="shrink-0 text-warning" />
                )}
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="font-medium">{issue.title}</span>
                    <Pill tone={issue.level === 'blocking' ? 'bad' : 'warn'}>
                      {issue.source}
                    </Pill>
                  </div>
                  <div className="mt-1 text-xs text-base-content/55">{issue.message}</div>
                  {issue.context ? (
                    <div className="mono mt-1 truncate text-[0.68rem] text-base-content/35">
                      {issue.context}
                    </div>
                  ) : null}
                </div>
                {issue.actionKind ? (
                  <button
                    className="btn btn-sm btn-primary"
                    type="button"
                    disabled={busy || state.status.refresh?.refreshing}
                    onClick={() => void runAction(issue.actionKind!)}
                  >
                    <RefreshCw size={13} /> {issue.action}
                  </button>
                ) : issue.href ? (
                  <Link className="btn btn-sm btn-outline" href={issue.href}>
                    {issue.action}
                  </Link>
                ) : null}
              </div>
            ))}
          </div>
        </section>
      )}
    </>
  );
}
