'use client';

import {useState, type ReactNode} from 'react';
import {
  Activity,
  Boxes,
  Download,
  PackageSearch,
  RefreshCw,
  ScanSearch,
  Terminal,
} from 'lucide-react';
import {useManagement} from '@/components/management-provider';
import {PageHeader, Pill, formatDate} from '@/components/ui';
import {api} from '@/lib/api';

type ToolAction = 'inventory' | 'doctor' | 'catalog' | 'updates' | 'full';

interface CatalogPreview {
  summary: {
    inventory_jars: number;
    unique_artifacts: number;
    duplicate_artifacts: number;
    generated_projects: number;
    unresolved: number;
    conflict_projects: number;
    placement_warnings: number;
    pinned_artifacts: number;
  };
}

export default function ToolsPage() {
  const {state, refresh, checkUpdates, refreshInventory} = useManagement();
  const [running, setRunning] = useState<ToolAction | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [catalogPreview, setCatalogPreview] = useState<CatalogPreview | null>(null);

  const run = async (action: ToolAction) => {
    setRunning(action);
    setMessage(null);
    setError(null);
    try {
      if (action === 'updates') {
        await checkUpdates();
        setMessage('Update discovery requested.');
      } else if (action === 'full') {
        await refresh();
        setMessage('Full inventory + provider refresh requested.');
      } else if (action === 'catalog') {
        const report = await api<CatalogPreview>('/api/catalog/preview', {method: 'POST'});
        setCatalogPreview(report);
        setMessage(
          'Catalog preview complete: ' +
            report.summary.generated_projects +
            ' managed project(s), ' +
            report.summary.unresolved +
            ' unresolved artifact(s).',
        );
      } else {
        await refreshInventory();
        setMessage(
          action === 'doctor'
            ? 'Fresh inventory scan requested. Diagnostics will update from the resulting inventory.'
            : 'Inventory scan requested.',
        );
      }
    } catch (value: unknown) {
      setError(value instanceof Error ? value.message : String(value));
    } finally {
      setRunning(null);
    }
  };

  const exports = [
    ['/api/inventory', 'Inventory JSON'],
    ['/api/catalog', 'Accepted catalog JSON'],
    ['/api/diagnostics', 'Doctor / diagnostics JSON'],
    ['/api/updates', 'Update report JSON'],
    ['/api/status', 'Status / version JSON'],
  ] as const;

  return (
    <>
      <PageHeader
        eyebrow="Operations"
        title="Tools"
        description="GUI equivalents for the practical FPBPack CLI commands used against this running server."
        action={<Pill tone="neutral">fpbpack {state.status.version ?? 'dev'}</Pill>}
      />

      {error ? <div className="alert alert-error mb-4 py-2 text-sm">{error}</div> : null}
      {message ? <div className="alert alert-info mb-4 py-2 text-sm">{message}</div> : null}

      <div className="grid gap-4 xl:grid-cols-2">
        <section className="panel">
          <div className="panel-header">
            <div>
              <div className="section-label">Commands</div>
              <h2 className="mt-0.5 text-sm font-semibold">Run against the configured server</h2>
            </div>
          </div>
          <div className="divide-y divide-base-300">
            <ToolRow
              icon={<ScanSearch size={16} />}
              title="inventory"
              description="Rescan server/common and AutoModpack client-only JARs and update the cached inventory."
              disabled={running !== null}
              busy={running === 'inventory'}
              onClick={() => void run('inventory')}
            />
            <ToolRow
              icon={<Activity size={16} />}
              title="doctor"
              description="Run a fresh inventory scan; diagnostics are recomputed against the accepted catalog without changing JARs."
              disabled={running !== null}
              busy={running === 'doctor'}
              onClick={() => void run('doctor')}
            />
            <ToolRow
              icon={<Boxes size={16} />}
              title="catalog preview"
              description="Run the catalog analyzer against the current inventory without mutating accepted state or writing a Packwiz workspace."
              disabled={running !== null}
              busy={running === 'catalog'}
              onClick={() => void run('catalog')}
            />
            <ToolRow
              icon={<PackageSearch size={16} />}
              title="updates"
              description="Query configured providers and rebuild the update report without rescanning the filesystem first."
              disabled={running !== null}
              busy={running === 'updates'}
              onClick={() => void run('updates')}
            />
            <ToolRow
              icon={<RefreshCw size={16} />}
              title="full refresh"
              description="Inventory + provider discovery. This is the manual equivalent of the scheduled refresh."
              disabled={running !== null}
              busy={running === 'full'}
              onClick={() => void run('full')}
            />
          </div>
        </section>

        <section className="panel">
          <div className="panel-header">
            <div>
              <div className="section-label">State</div>
              <h2 className="mt-0.5 text-sm font-semibold">Current command outputs</h2>
            </div>
          </div>
          <dl className="divide-y divide-base-300 text-sm">
            <div className="flex justify-between gap-4 px-4 py-3">
              <dt className="text-base-content/45">Update report generated</dt>
              <dd>{formatDate(state.updates.generated_at || undefined)}</dd>
            </div>
            <div className="flex justify-between gap-4 px-4 py-3">
              <dt className="text-base-content/45">Installed JARs</dt>
              <dd>{state.status.mods}</dd>
            </div>
            <div className="flex justify-between gap-4 px-4 py-3">
              <dt className="text-base-content/45">Blocking diagnostics</dt>
              <dd>{state.diagnostics.summary.blocking}</dd>
            </div>
            <div className="flex justify-between gap-4 px-4 py-3">
              <dt className="text-base-content/45">Update candidates</dt>
              <dd>{state.updates.candidates.length}</dd>
            </div>
            {catalogPreview ? (
              <>
                <div className="flex justify-between gap-4 px-4 py-3">
                  <dt className="text-base-content/45">Preview managed projects</dt>
                  <dd>{catalogPreview.summary.generated_projects}</dd>
                </div>
                <div className="flex justify-between gap-4 px-4 py-3">
                  <dt className="text-base-content/45">Preview unresolved</dt>
                  <dd>{catalogPreview.summary.unresolved}</dd>
                </div>
              </>
            ) : null}
          </dl>
        </section>
      </div>

      <section className="panel mt-4">
        <div className="panel-header">
          <div>
            <div className="section-label">JSON</div>
            <h2 className="mt-0.5 text-sm font-semibold">Inspect/export command state</h2>
          </div>
        </div>
        <div className="grid gap-2 p-4 sm:grid-cols-2 lg:grid-cols-3">
          {exports.map(([href, label]) => (
            <a
              className="btn btn-sm btn-outline justify-start"
              href={href}
              target="_blank"
              rel="noreferrer"
              key={href}
            >
              <Download size={13} /> {label}
            </a>
          ))}
        </div>
      </section>

      <section className="panel mt-4 p-4 text-sm text-base-content/55">
        <div className="flex items-start gap-3">
          <Terminal size={16} className="mt-0.5 shrink-0" />
          <p>
            The GUI runs the read-only analysis portion of <span className="mono">catalog</span>.
            Writing/replacing an arbitrary Packwiz workspace and invoking an external Packwiz helper
            remain CLI-only. <span className="mono">serve</span> is this application itself, and
            <span className="mono">version</span> is shown above.
          </p>
        </div>
      </section>
    </>
  );
}

function ToolRow({
  icon,
  title,
  description,
  disabled,
  busy,
  onClick,
}: {
  icon: ReactNode;
  title: string;
  description: string;
  disabled: boolean;
  busy: boolean;
  onClick: () => void;
}) {
  return (
    <div className="flex flex-col gap-3 p-4 sm:flex-row sm:items-center sm:justify-between">
      <div className="flex items-start gap-3">
        <div className="mt-0.5 text-base-content/45">{icon}</div>
        <div>
          <div className="mono text-sm font-semibold">{title}</div>
          <p className="mt-1 text-xs text-base-content/45">{description}</p>
        </div>
      </div>
      <button className="btn btn-sm btn-primary" type="button" disabled={disabled} onClick={onClick}>
        {busy ? <span className="loading loading-spinner loading-xs" /> : null}
        Run
      </button>
    </div>
  );
}
