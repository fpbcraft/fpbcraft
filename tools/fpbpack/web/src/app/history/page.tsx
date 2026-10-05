import {History as HistoryIcon} from 'lucide-react';
import {EmptyState, PageHeader, Pill} from '@/components/ui';

export default function HistoryPage() {
  return (
    <>
      <PageHeader
        eyebrow="Audit"
        title="History"
        description="Plans, applies, restores, and reconciliation events."
        action={<Pill tone="neutral">No events</Pill>}
      />
      <section className="panel p-4">
        <div className="mb-4 flex items-center gap-2 text-sm font-medium">
          <HistoryIcon size={16} className="text-base-content/40" />
          Operation history
        </div>
        <EmptyState title="No operations recorded yet">
          Creating update plans in Plan &amp; Protect will start the structured audit history.
        </EmptyState>
      </section>
    </>
  );
}
