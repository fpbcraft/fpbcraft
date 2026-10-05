'use client';

import {EmptyState, PageHeader, Pill} from '@/components/ui';
import {useManagement} from '@/components/management-provider';

export default function UpdatesPage() {
  const {state} = useManagement();

  return (
    <>
      <PageHeader
        eyebrow="Discover"
        title="Updates"
        description="Provider-aware candidates will be classified here before any live changes are allowed."
        action={<Pill tone="blue">Read only</Pill>}
      />

      {state.diagnostics.summary.blocking > 0 ? (
        <section className="notice notice-warn">
          <strong>
            {state.diagnostics.summary.blocking} blocking diagnostic
            {state.diagnostics.summary.blocking === 1 ? '' : 's'}
          </strong>
          <p>
            Managed-file drift or unresolved catalog state must be reconciled before
            future updates can be considered safe.
          </p>
        </section>
      ) : null}

      <section className="two-column">
        <article className="panel">
          <div className="panel-heading">
            <div>
              <span className="eyebrow">Safe</span>
              <h2>Recommended</h2>
            </div>
            <Pill tone="good">0</Pill>
          </div>
          <EmptyState title="No candidate data yet">
            Provider-aware update discovery is the next FPBPack service being added.
          </EmptyState>
        </article>

        <article className="panel">
          <div className="panel-heading">
            <div>
              <span className="eyebrow">Review</span>
              <h2>Needs review</h2>
            </div>
            <Pill tone="warn">0</Pill>
          </div>
          <EmptyState title="No candidate data yet">
            Major jumps, pre-releases, and dependency-driven changes will appear here.
          </EmptyState>
        </article>
      </section>

      <section className="two-column">
        <article className="panel">
          <div className="panel-heading">
            <div>
              <span className="eyebrow">Blocked</span>
              <h2>Cannot update safely</h2>
            </div>
            <Pill tone="bad">0</Pill>
          </div>
          <EmptyState title="No candidate data yet">
            Incompatible loaders, Minecraft versions, and unresolved dependencies will
            be preserved here for diagnostics.
          </EmptyState>
        </article>

        <article className="panel">
          <div className="panel-heading">
            <div>
              <span className="eyebrow">Ignored</span>
              <h2>Pinned and ignored</h2>
            </div>
            <Pill tone="neutral">{state.status.unmanaged}</Pill>
          </div>
          <EmptyState title="No update rules yet">
            Explicitly unmanaged artifacts are already excluded. Persistent pin and
            ignore rules are still pending.
          </EmptyState>
        </article>
      </section>
    </>
  );
}
