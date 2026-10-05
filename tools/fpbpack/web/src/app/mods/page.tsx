'use client';

import {useMemo, useState} from 'react';
import {Search, X} from 'lucide-react';
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
      const text = [mod.name, mod.filename, mod.installed_version ?? '', mod.provider ?? '', mod.project_id ?? '']
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
  const rows = filtered.slice((safePage - 1) * PAGE_SIZE, safePage * PAGE_SIZE);

  const updateFilter = (setter: (value: string) => void, value: string) => {
    setter(value);
    setPage(1);
  };

  return (
    <>
      <PageHeader
        eyebrow="Inventory"
        title="Mods"
        description="Installed server/common and AutoModpack client-only artifacts."
        action={
          <Pill tone={connectionStatus === 'connected' ? 'good' : 'warn'}>
            {state.mods.length} JARs
          </Pill>
        }
      />

      <div className="mb-3 grid gap-2 md:grid-cols-[minmax(240px,1fr)_repeat(3,minmax(130px,auto))_auto]">
        <label className="input input-sm flex w-full items-center gap-2">
          <Search size={14} className="text-base-content/35" />
          <input
            className="grow"
            value={q}
            onChange={(event) => updateFilter(setQ, event.target.value)}
            placeholder="Search mods…"
            aria-label="Search mods"
          />
        </label>
        <select className="select select-sm" value={deployment} onChange={(event) => updateFilter(setDeployment, event.target.value)}>
          <option value="all">All placements</option>
          <option value="server">Server/common</option>
          <option value="client">Client-only</option>
        </select>
        <select className="select select-sm" value={management} onChange={(event) => updateFilter(setManagement, event.target.value)}>
          <option value="all">All management</option>
          <option value="managed">Managed</option>
          <option value="unmanaged">Unmanaged</option>
          <option value="unresolved">Unresolved</option>
          <option value="external">External change</option>
        </select>
        <select className="select select-sm" value={provider} onChange={(event) => updateFilter(setProvider, event.target.value)}>
          <option value="all">All providers</option>
          {providers.map((item) => <option value={item} key={item}>{item}</option>)}
        </select>
        <button
          type="button"
          className="btn btn-sm btn-ghost"
          onClick={() => {
            setQ('');
            setDeployment('all');
            setManagement('all');
            setProvider('all');
            setPage(1);
          }}
        >
          <X size={14} /> Reset
        </button>
      </div>

      <section className="panel overflow-hidden">
        <div className="flex items-center justify-between border-b border-base-300 px-4 py-2 text-xs text-base-content/45">
          <span>{filtered.length} matching JARs</span>
          <span>Page {safePage} / {totalPages}</span>
        </div>
        <div className="overflow-x-auto">
          <table className="table table-sm">
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
                <tr key={mod.id + ':' + mod.path} className="border-base-300 hover:bg-base-300/25">
                  <td>
                    <div className="font-medium">{mod.name}</div>
                    <div className="mono mt-0.5 text-base-content/35">{mod.filename}</div>
                  </td>
                  <td className="whitespace-nowrap">{mod.installed_version || '—'}</td>
                  <td><Pill tone={mod.side === 'client' ? 'blue' : 'neutral'}>{mod.side}</Pill></td>
                  <td>
                    {mod.project_url ? (
                      <a className="link link-hover text-sm" href={mod.project_url} target="_blank" rel="noreferrer">
                        {mod.provider ?? 'unknown'}
                      </a>
                    ) : (
                      <span className="text-base-content/45">{mod.provider ?? 'unknown'}</span>
                    )}
                  </td>
                  <td><Pill tone={managementTone(mod.management)}>{mod.management}</Pill></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <div className="flex items-center justify-between border-t border-base-300 px-3 py-2">
          <button className="btn btn-xs btn-ghost" type="button" disabled={safePage <= 1} onClick={() => setPage(safePage - 1)}>Previous</button>
          <button className="btn btn-xs btn-ghost" type="button" disabled={safePage >= totalPages} onClick={() => setPage(safePage + 1)}>Next</button>
        </div>
      </section>
    </>
  );
}
