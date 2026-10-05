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
  type DiagnosticReport,
  type ManagementMod,
  type ManagementState,
  type ManagementStatus,
} from '@/lib/management';

type ConnectionStatus = 'loading' | 'connected' | 'error';

interface ManagementContextValue {
  state: ManagementState;
  connectionStatus: ConnectionStatus;
  connectionError: string | null;
  refresh: () => Promise<void>;
}

const ManagementContext = createContext<ManagementContextValue | null>(null);

async function fetchApi<T>(path: string): Promise<T> {
  const response = await fetch(path, {
    cache: 'no-store',
    headers: {Accept: 'application/json'},
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

  return {
    status,
    mods: modsResponse.mods,
    diagnostics,
    source: 'api',
    errors: [],
  };
}

export function ManagementProvider({children}: {children: ReactNode}) {
  const [state, setState] = useState<ManagementState>(() => emptyManagementState());
  const [connectionStatus, setConnectionStatus] =
    useState<ConnectionStatus>('loading');
  const [connectionError, setConnectionError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    setConnectionStatus('loading');
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

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const value = useMemo(
    () => ({state, connectionStatus, connectionError, refresh}),
    [state, connectionStatus, connectionError, refresh],
  );

  return (
    <ManagementContext.Provider value={value}>
      {children}
    </ManagementContext.Provider>
  );
}

export function useManagement() {
  const value = useContext(ManagementContext);
  if (!value) {
    throw new Error('useManagement must be used inside ManagementProvider.');
  }
  return value;
}
