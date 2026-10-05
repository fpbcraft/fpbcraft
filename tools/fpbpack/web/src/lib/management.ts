export type DiagnosticLevel = 'info' | 'warning' | 'blocking';
export type Location = 'server' | 'client';
export type UpdateClassification = 'safe' | 'review' | 'blocked' | 'ignored' | 'up_to_date';

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

export interface UpdateReason {
  code: string;
  message: string;
}

export interface UpdateRelease {
  id: string;
  number: string;
  name?: string;
  published_at?: string;
  channel?: string;
  filename?: string;
  url?: string;
  sha512?: string;
}

export interface UpdateDependency {
  provider: string;
  project_id?: string;
  version_id?: string;
  type: string;
  action: string;
  installed_version?: string;
  target_version?: string;
}

export interface UpdateCandidate {
  key: string;
  provider: string;
  project_id: string;
  name: string;
  project_url?: string;
  icon_url?: string;
  side: string;
  deployment: Location;
  installed: UpdateRelease;
  target?: UpdateRelease;
  classification: UpdateClassification;
  reasons?: UpdateReason[];
  dependencies?: UpdateDependency[];
}

export interface UpdateSummary {
  safe: number;
  review: number;
  blocked: number;
  ignored: number;
  up_to_date: number;
}

export interface UpdateReport {
  generated_at: string;
  minecraft: string;
  loader: string;
  summary: UpdateSummary;
  candidates: UpdateCandidate[];
}

export interface ManagementState {
  status: ManagementStatus;
  diagnostics: DiagnosticReport;
  mods: ManagementMod[];
  updates: UpdateReport;
  source: 'api' | 'unavailable';
  errors: string[];
}

const emptySummary = (): DiagnosticSummary => ({
  blocking: 0,
  warnings: 0,
  info: 0,
  actionable: 0,
});

export function emptyUpdateReport(): UpdateReport {
  return {
    generated_at: '',
    minecraft: '',
    loader: '',
    summary: {safe: 0, review: 0, blocked: 0, ignored: 0, up_to_date: 0},
    candidates: [],
  };
}

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
    updates: emptyUpdateReport(),
    source: 'unavailable',
    errors: [],
  };
}
