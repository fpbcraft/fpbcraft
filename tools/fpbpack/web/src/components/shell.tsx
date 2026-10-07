'use client';

import Link from 'next/link';
import {usePathname} from 'next/navigation';
import type {ReactNode} from 'react';
import {
  Boxes,
  Cable,
  History,
  Home,
  PackageSearch,
  ListChecks,
  ScrollText,
  Settings,
  ShieldCheck,
  Terminal,
} from 'lucide-react';
import {useManagement} from '@/components/management-provider';

const nav = [
  {href: '/', label: 'Overview', icon: Home},
  {href: '/updates', label: 'Updates', icon: PackageSearch},
  {href: '/mods', label: 'Mods', icon: Boxes},
  {href: '/pending', label: 'Pending changes', icon: ListChecks},
  {href: '/automodpack', label: 'AutoModpack', icon: Cable},
  {href: '/history', label: 'History', icon: History},
  {href: '/tools', label: 'Tools', icon: Terminal},
  {href: '/logs', label: 'Logs', icon: ScrollText},
  {href: '/settings', label: 'Settings', icon: Settings},
];

export function Shell({children}: {children: ReactNode}) {
  const pathname = usePathname();
  const {state, connectionStatus} = useManagement();

  return (
    <div className="min-h-screen bg-base-200 lg:grid lg:grid-cols-[220px_minmax(0,1fr)]">
      <aside className="border-b border-base-300 bg-base-100 lg:sticky lg:top-0 lg:h-screen lg:border-r lg:border-b-0">
        <div className="flex h-full flex-col">
          <div className="flex h-16 items-center gap-3 border-b border-base-300 px-4">
            <div className="grid size-8 place-items-center rounded-md border border-base-300 bg-base-200 text-xs font-bold tracking-tight">
              FP
            </div>
            <div className="min-w-0">
              <div className="text-sm font-semibold leading-tight">FPBPack</div>
              <div className="truncate text-[0.7rem] text-base-content/45">FPBCraft mod manager</div>
            </div>
          </div>

          <nav className="flex gap-1 overflow-x-auto p-2 lg:flex-col lg:overflow-visible">
            {nav.map((item) => {
              const Icon = item.icon;
              const active =
                item.href === '/' ? pathname === '/' : pathname.startsWith(item.href);
              return (
                <Link
                  key={item.href}
                  href={item.href}
                  className={[
                    'flex h-9 shrink-0 items-center gap-2 rounded-md px-3 text-sm transition-colors',
                    active
                      ? 'bg-base-300 font-medium text-base-content'
                      : 'text-base-content/60 hover:bg-base-300/60 hover:text-base-content',
                  ].join(' ')}
                >
                  <Icon size={16} strokeWidth={1.8} aria-hidden="true" />
                  <span>{item.label}</span>
                  {item.href === '/pending' && state.pending.changes.length > 0 ? (
                    <span className="badge badge-primary badge-xs ml-auto">
                      {state.pending.changes.length}
                    </span>
                  ) : null}
                </Link>
              );
            })}
          </nav>

          <div className="mt-auto hidden border-t border-base-300 p-3 lg:block">
            <div className="flex items-center gap-2 text-xs text-base-content/55">
              <span
                className={[
                  'status-dot',
                  connectionStatus === 'connected' ? 'status-dot-online' : '',
                ].join(' ')}
              />
              <span>
                {connectionStatus === 'connected'
                  ? state.status.refresh?.refreshing
                    ? state.status.refresh.message || 'Refreshing…'
                    : 'FPBPack connected'
                  : connectionStatus === 'loading'
                    ? 'Connecting…'
                    : 'Service unavailable'}
              </span>
            </div>
            {state.status.refresh?.refreshing ? (
              <div className="mt-2">
                <progress
                  className="progress progress-primary h-1.5 w-full"
                  max={100}
                  value={state.status.refresh.percent ?? 0}
                />
                <div className="mt-1 flex justify-between text-[0.65rem] text-base-content/35">
                  <span>{state.status.refresh.phase ?? 'refresh'}</span>
                  <span className="tabular-nums">{state.status.refresh.percent ?? 0}%</span>
                </div>
              </div>
            ) : null}
            <div className="mt-2 flex items-center gap-2 text-[0.7rem] text-base-content/35">
              <ShieldCheck size={13} aria-hidden="true" />
              <span>Verified changes · protected Apply</span>
            </div>
          </div>
        </div>
      </aside>

      <main className="min-w-0">
        <div className="mx-auto w-full max-w-[1500px] px-4 py-6 sm:px-6 lg:px-8 lg:py-8">
          {state.status.refresh?.refreshing ? (
            <div className="surface mb-4 rounded-box px-4 py-3">
              <div className="flex items-center justify-between gap-4 text-xs">
                <div className="min-w-0">
                  <div className="font-medium">
                    {state.status.refresh.message || 'Refreshing FPBPack data'}
                  </div>
                  <div className="mt-0.5 truncate text-base-content/40">
                    {state.status.refresh.total
                      ? (state.status.refresh.current ?? 0) +
                        ' / ' +
                        state.status.refresh.total +
                        ' provider artifacts'
                      : state.status.refresh.phase || 'refresh'}
                  </div>
                </div>
                <span className="tabular-nums text-base-content/55">
                  {state.status.refresh.percent ?? 0}%
                </span>
              </div>
              <progress
                className="progress progress-primary mt-2 h-1.5 w-full"
                max={100}
                value={state.status.refresh.percent ?? 0}
              />
            </div>
          ) : null}
          {children}
        </div>
      </main>
    </div>
  );
}
