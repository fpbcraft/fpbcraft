import {EmptyState, PageHeader, Pill} from '@/components/ui';

export default function HistoryPage() {
  return (
    <>
      <PageHeader
        eyebrow="Audit"
        title="History"
        description="Update plans, apply operations, restores, and reconciliation events."
        action={<Pill tone="neutral">Not used yet</Pill>}
      />
      <section className="panel">
        <EmptyState title="No operations have been recorded">
          Structured history starts in Plan &amp; Protect. Discover &amp; Decide does not
          mutate the live pack.
        </EmptyState>
      </section>
    </>
  );
}
