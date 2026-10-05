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

export interface RefreshStatus {
  refreshing: boolean;
  last_success?: string;
  last_error?: string;
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
  refresh?: RefreshStatus;
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
  sha1?: string;
  sha256?: string;
  sha512?: string;
  manual_download?: boolean;
  manual_url?: string;
}

export interface UpdateDependency {
  provider: string;
  project_id?: string;
  version_id?: string;
  name?: string;
  type: string;
  action: string;
  installed_version?: string;
  target_version?: string;
  deployment?: Location;
  target?: UpdateRelease;
  dependencies?: UpdateDependency[];
}

export interface UpdateRule {
  pin_version?: string;
  ignore_mod?: boolean;
  ignored_versions?: string[];
  review_after?: string;
}

export interface UpdateChangelogEntry {
  id: string;
  number: string;
  name?: string;
  published_at?: string;
  channel?: string;
  body?: string;
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
  changelogs?: UpdateChangelogEntry[];
  required_by?: string[];
  metadata_stale?: boolean;
  refresh_error?: string;
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
      refresh: {refreshing: false},
    },
    diagnostics,
    mods: [],
    updates: emptyUpdateReport(),
    source: 'unavailable',
    errors: [],
  };
}

export interface PlanFinding {
  code: string;
  message: string;
  candidate_key?: string;
}

export interface PlanArtifact {
  provider: string;
  project_id: string;
  version_id: string;
  filename: string;
  url: string;
  sha1?: string;
  sha256?: string;
  sha512: string;
  deployment: string;
  manual_download?: boolean;
  manual_url?: string;
}

export interface PlanFileOperation {
  action: string;
  current_path?: string;
  target_path: string;
  current_sha512?: string;
  target_sha512: string;
}

export interface PlanChange {
  candidate_key: string;
  name: string;
  requested: boolean;
  dependency_driven: boolean;
  classification: UpdateClassification;
  installed: UpdateRelease;
  target: UpdateRelease;
  artifact: PlanArtifact;
  operations: PlanFileOperation[];
}

export interface UpdatePlan {
  schema_version: number;
  id: string;
  created_at: string;
  status: 'ready' | 'blocked';
  inventory_generated_at: string;
  updates_generated_at: string;
  selected: string[];
  changes: PlanChange[];
  warnings?: PlanFinding[];
  blockers?: PlanFinding[];
  prefetched?: Array<{
    filename: string;
    sha512: string;
    cache_path: string;
    bytes: number;
  }>;
  verified: boolean;
  verified_at?: string;
  backup_id?: string;
  requires_server_stop: boolean;
  requires_backup: boolean;
}

export interface PlanSummary {
  id: string;
  created_at: string;
  status: 'ready' | 'blocked';
  changes: number;
  blockers: number;
  warnings: number;
  verified: boolean;
}

export interface HistoryEvent {
  id: string;
  created_at: string;
  type: string;
  status: string;
  plan_id?: string;
  mods: number;
  summary: string;
}
