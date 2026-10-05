'use client';

import {useMemo, useState} from 'react';
import {PageHeader, Pill} from '@/components/ui';
import {useManagement} from '@/components/management-provider';
import type {ManagementMod} from '@/lib/management';

const PAGE_SIZE = 50;

function managementTone(
  management: ManagementMod['management'],
): 'good' | 'warn' | 'bad' | 'neutral' {
  if (management === 'managed') return 'good';
  if (management === 'unmanaged') return 'warn';
  if (management === 'unresolved' || management === 'external') return 'bad';
  return 'neutral';
}

export default function ModsPage() {
  const {state, connectionStatus} = useManagement();
  const [q, setQ] = useState('');
  const [deployment, setDeployment] = useState('all');
  const [management, setManagement] = useState('all');
  const [provider, setProvider] = useState('all');
  const [page, setPage] = useState(1);

  const providers = useMemo(
    () =>
      [...new Set(state.mods.map((mod) => mod.provider).filter(Boolean))]
        .map(String)
        .sort((a, b) => a.localeCompare(b)),
    [state.mods],
  );

  const filtered = useMemo(() => {
    const needle = q.trim().toLowerCase();
    return state.mods.filter((mod) => {
      const text = [
        mod.name,
        mod.filename,
        mod.installed_version ?? '',
        mod.provider ?? '',
        mod.project_id ?? '',
      ]
        .join(' ')
        .toLowerCase();

      return (
        (!needle || text.includes(needle)) &&
        (deployment === 'all' || mod.deployment === deployment) &&
        (management === 'all' || mod.management === management) &&
        (provider === 'all' || mod.provider === provider)
      );
    });
  }, [state.mods, q, deployment, management, provider]);

  const totalPages = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE));
  const safePage = Math.min(page, totalPages);
  const start = (safePage - 1) * PAGE_SIZE;
  const rows = filtered.slice(start, start + PAGE_SIZE);

  const updateFilter = (setter: (value: string) => void, value: string) => {
    setter(value);
    setPage(1);
  };

  return (
    <>
      <PageHeader
        eyebrow="Inventory"
        title="Mods"
        description="Installed JARs with management identity, placement, and provider ownership."
        action={
          <Pill tone={state.source === 'api' ? 'good' : 'warn'}>
            {connectionStatus === 'loading'
              ? 'Loading…'
              : state.source === 'api'
                ? 'FPBPack'
                : 'API unavailable'}
          </Pill>
        }
      />

      {state.errors.length ? (
        <section className="notice notice-warn">
          <strong>Data source warning</strong>
          {state.errors.map((error) => (
            <p key={error}>{error}</p>
          ))}
        </section>
      ) : null}

      <div className="filters">
        <input
          value={q}
          onChange={(event) => updateFilter(setQ, event.target.value)}
          placeholder="Search name, filename, version…"
          aria-label="Search mods"
        />
        <select
          value={deployment}
          onChange={(event) => updateFilter(setDeployment, event.target.value)}
          aria-label="Filter by deployment"
        >
          <option value="all">All placements</option>
          <option value="server">Server/common</option>
          <option value="client">Client-only</option>
        </select>
        <select
          value={management}
          onChange={(event) => updateFilter(setManagement, event.target.value)}
          aria-label="Filter by management state"
        >
          <option value="all">All management</option>
          <option value="managed">Managed</option>
          <option value="unmanaged">Unmanaged</option>
          <option value="unresolved">Unresolved</option>
          <option value="external">External change</option>
        </select>
        <select
          value={provider}
          onChange={(event) => updateFilter(setProvider, event.target.value)}
          aria-label="Filter by provider"
        >
          <option value="all">All providers</option>
          {providers.map((item) => (
            <option value={item} key={item}>
              {item}
            </option>
          ))}
        </select>
        <button
          type="button"
          className="secondary-button"
          onClick={() => {
            setQ('');
            setDeployment('all');
            setManagement('all');
            setProvider('all');
            setPage(1);
          }}
        >
          Reset
        </button>
      </div>

      <section className="panel table-panel">
        <div className="table-summary">
          <span>{filtered.length} matching JARs</span>
          <span>
            Page {safePage} of {totalPages}
          </span>
        </div>
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Mod</th>
                <th>Version</th>
                <th>Side</th>
                <th>Source</th>
                <th>Management</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((mod) => (
                <tr key={mod.id + ':' + mod.path}>
                  <td data-label="Mod">
                    <div>
                      <strong>{mod.name}</strong>
                      <span className="subtle mono">{mod.filename}</span>
                    </div>
                  </td>
                  <td data-label="Version">{mod.installed_version || '—'}</td>
                  <td data-label="Side">
                    <Pill tone={mod.side === 'client' ? 'blue' : 'neutral'}>
                      {mod.side}
                    </Pill>
                  </td>
                  <td data-label="Source">
                    {mod.project_url ? (
                      <a href={mod.project_url} target="_blank" rel="noreferrer">
                        {mod.provider ?? 'unknown'}
                      </a>
                    ) : (
                      mod.provider ?? 'unknown'
                    )}
                  </td>
                  <td data-label="Management">
                    <Pill tone={managementTone(mod.management)}>{mod.management}</Pill>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <div className="pagination">
          {safePage > 1 ? (
            <button
              type="button"
              className="secondary-button"
              onClick={() => setPage(safePage - 1)}
            >
              Previous
            </button>
          ) : (
            <span />
          )}
          {safePage < totalPages ? (
            <button
              type="button"
              className="secondary-button"
              onClick={() => setPage(safePage + 1)}
            >
              Next
            </button>
          ) : (
            <span />
          )}
        </div>
      </section>
    </>
  );
}
