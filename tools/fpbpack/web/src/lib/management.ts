export type DiagnosticLevel = 'info' | 'warning' | 'blocking';
export type Location = 'server' | 'client';

export interface DiagnosticFinding {
  code: string;
  level: DiagnosticLevel;
  actionable: boolean;
  message: string;
  mod?: string;
  path?: string;
}

export interface DiagnosticSummary {
  blocking: number;
  warnings: number;
  info: number;
  actionable: number;
}

export interface DiagnosticReport {
  summary: DiagnosticSummary;
  findings: DiagnosticFinding[];
}

export interface ManagementStatus {
  mode: string;
  read_only: boolean;
  server_state: string;
  inventory_generated_at: string;
  mods: number;
  managed: number;
  unmanaged: number;
  diagnostics: DiagnosticSummary;
  version?: string;
}

export interface ManagementMod {
  id: string;
  name: string;
  filename: string;
  installed_version?: string;
  provider?: string;
  project_id?: string;
  project_url?: string;
  side: string;
  deployment: Location;
  management: 'managed' | 'unmanaged' | 'unresolved' | 'external' | string;
  path: string;
  sha512?: string;
}

export interface ManagementState {
  status: ManagementStatus;
  diagnostics: DiagnosticReport;
  mods: ManagementMod[];
  source: 'api' | 'unavailable';
  errors: string[];
}

const emptySummary = (): DiagnosticSummary => ({
  blocking: 0,
  warnings: 0,
  info: 0,
  actionable: 0,
});

export function emptyManagementState(): ManagementState {
  const diagnostics = {summary: emptySummary(), findings: []};
  return {
    status: {
      mode: 'read-only',
      read_only: true,
      server_state: 'unknown',
      inventory_generated_at: '',
      mods: 0,
      managed: 0,
      unmanaged: 0,
      diagnostics: diagnostics.summary,
    },
    diagnostics,
    mods: [],
    source: 'unavailable',
    errors: [],
  };
}
