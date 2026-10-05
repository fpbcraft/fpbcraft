'use client';

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from 'react';
import {
  emptyManagementState,
  emptyUpdateReport,
  type DiagnosticReport,
  type ManagementMod,
  type ManagementState,
  type ManagementStatus,
  type UpdateReport,
} from '@/lib/management';

type ConnectionStatus = 'loading' | 'connected' | 'error';

interface ManagementContextValue {
  state: ManagementState;
  connectionStatus: ConnectionStatus;
  connectionError: string | null;
  reload: (options?: {silent?: boolean}) => Promise<void>;
  refresh: () => Promise<void>;
  checkUpdates: () => Promise<void>;
}

const ManagementContext = createContext<ManagementContextValue | null>(null);

async function fetchApi<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, {
    cache: 'no-store',
    headers: {Accept: 'application/json', ...(init?.headers ?? {})},
    ...init,
  });
  if (!response.ok) {
    throw new Error(path + ' returned HTTP ' + response.status);
  }
  return (await response.json()) as T;
}

async function loadManagementState(): Promise<ManagementState> {
  const [status, modsResponse, diagnostics] = await Promise.all([
    fetchApi<ManagementStatus>('/api/status'),
    fetchApi<{mods: ManagementMod[]}>('/api/mods'),
    fetchApi<DiagnosticReport>('/api/diagnostics'),
  ]);

  let updates = emptyUpdateReport();
  const errors: string[] = [];
  try {
    updates = await fetchApi<UpdateReport>('/api/updates');
  } catch {
    if (status.refresh?.last_error) {
      errors.push('Refresh failed: ' + status.refresh.last_error);
    } else if (status.refresh?.refreshing) {
      errors.push('Update discovery is still refreshing; cached update data is not available yet.');
    } else {
      errors.push('No cached update data is available yet. Run Check updates when you want to query providers.');
    }
  }

  return {
    status,
    mods: Array.isArray(modsResponse.mods) ? modsResponse.mods : [],
    diagnostics: {
      ...diagnostics,
      findings: Array.isArray(diagnostics.findings) ? diagnostics.findings : [],
    },
    updates: {
      ...updates,
      candidates: Array.isArray(updates.candidates) ? updates.candidates : [],
    },
    source: 'api',
    errors,
  };
}

export function ManagementProvider({children}: {children: ReactNode}) {
  const [state, setState] = useState<ManagementState>(() => emptyManagementState());
  const [connectionStatus, setConnectionStatus] = useState<ConnectionStatus>('loading');
  const [connectionError, setConnectionError] = useState<string | null>(null);

  const reload = useCallback(async (options?: {silent?: boolean}) => {
    if (!options?.silent) {
      setConnectionStatus('loading');
    }
    setConnectionError(null);
    try {
      setState(await loadManagementState());
      setConnectionStatus('connected');
    } catch (error: unknown) {
      const message =
        'Could not load FPBPack management state: ' +
        (error instanceof Error ? error.message : String(error));
      setState({...emptyManagementState(), errors: [message]});
      setConnectionStatus('error');
      setConnectionError(message);
    }
  }, []);

  const refresh = useCallback(async () => {
    await fetchApi<{status: string}>('/api/refresh', {method: 'POST'});
    await reload({silent: true});
  }, [reload]);

  const checkUpdates = useCallback(async () => {
    await fetchApi<{status: string}>('/api/updates/check', {method: 'POST'});
    await reload({silent: true});
  }, [reload]);

  useEffect(() => {
    void reload();
  }, [reload]);

  useEffect(() => {
    const shouldPoll =
      state.status.refresh?.refreshing ||
      state.errors.some((message) => message.includes('still refreshing'));
    if (!shouldPoll) {
      return;
    }
    const timer = window.setTimeout(() => {
      void reload({silent: true});
    }, 2500);
    return () => window.clearTimeout(timer);
  }, [state.status.refresh?.refreshing, state.errors, reload]);

  const value = useMemo(
    () => ({state, connectionStatus, connectionError, reload, refresh, checkUpdates}),
    [state, connectionStatus, connectionError, reload, refresh, checkUpdates],
  );

  return <ManagementContext.Provider value={value}>{children}</ManagementContext.Provider>;
}

export function useManagement() {
  const value = useContext(ManagementContext);
  if (!value) {
    throw new Error('useManagement must be used inside ManagementProvider.');
  }
  return value;
}
