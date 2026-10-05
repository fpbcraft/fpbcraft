import type {ReactNode} from 'react';

export function PageHeader({
  eyebrow,
  title,
  description,
  action,
}: {
  eyebrow: string;
  title: string;
  description: string;
  action?: ReactNode;
}) {
  return (
    <header className="mb-6 flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
      <div className="min-w-0">
        <div className="section-label mb-1">{eyebrow}</div>
        <h1 className="text-2xl font-semibold tracking-[-0.02em] text-base-content">{title}</h1>
        <p className="mt-1 max-w-3xl text-sm text-base-content/55">{description}</p>
      </div>
      {action ? <div className="shrink-0">{action}</div> : null}
    </header>
  );
}

export function Metric({
  label,
  value,
  detail,
  tone = 'neutral',
}: {
  label: string;
  value: string | number;
  detail: string;
  tone?: 'neutral' | 'good' | 'warn' | 'bad';
}) {
  const toneClass = {
    neutral: 'text-base-content',
    good: 'text-success',
    warn: 'text-warning',
    bad: 'text-error',
  }[tone];

  return (
    <article className="surface rounded-box px-4 py-3">
      <div className="section-label">{label}</div>
      <div className={`mt-1 text-2xl font-semibold tabular-nums ${toneClass}`}>{value}</div>
      <div className="mt-1 text-xs text-base-content/45">{detail}</div>
    </article>
  );
}

export function Pill({
  children,
  tone = 'neutral',
}: {
  children: ReactNode;
  tone?: 'neutral' | 'good' | 'warn' | 'bad' | 'blue';
}) {
  const toneClass = {
    neutral: 'badge-ghost text-base-content/65',
    good: 'badge-success',
    warn: 'badge-warning',
    bad: 'badge-error',
    blue: 'badge-info',
  }[tone];
  return <span className={`badge badge-sm ${toneClass}`}>{children}</span>;
}

export function EmptyState({
  title,
  children,
}: {
  title: string;
  children: ReactNode;
}) {
  return (
    <div className="rounded-md border border-dashed border-base-300 px-5 py-8 text-center">
      <div className="text-sm font-medium">{title}</div>
      <p className="mx-auto mt-1 max-w-xl text-xs text-base-content/45">{children}</p>
    </div>
  );
}

export function formatDate(value: string | null | undefined) {
  if (!value) return 'Unavailable';
  return new Intl.DateTimeFormat('en-CA', {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(new Date(value));
}
