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
  kind?: string;
  phase?: string;
  message?: string;
  current?: number;
  total?: number;
  percent?: number;
  started_at?: string;
  last_success?: string;
  last_error?: string;
}

export interface CatalogProject {
  provider: 'modrinth' | 'curseforge' | string;
  project_id: string;
  name: string;
  slug?: string;
  summary?: string;
  icon_url?: string;
  project_url?: string;
  downloads?: number;
  environment?: string[];
  installed: boolean;
  installed_key?: string;
}

export interface CatalogVersion {
  id: string;
  number: string;
  name?: string;
  published_at?: string;
  channel?: string;
  filename?: string;
  environment?: string;
  changelog?: string;
  sha1?: string;
  sha512?: string;
  manual_download?: boolean;
  manual_url?: string;
}

export interface RuntimeLogEntry {
  id: number;
  time: string;
  level: 'info' | 'warn' | 'error' | string;
  area: string;
  message: string;
}

export interface CraftyStatus {
  configured: boolean;
  connected: boolean;
  state: string;
  detail?: string;
  url?: string;
  server_id?: string;
  credential_source?: 'saved' | 'environment';
  allow_insecure?: boolean;
}

export interface NeoForgeVersion {
  version: string;
  channel: 'release' | 'beta' | 'alpha' | 'rc' | string;
  current?: boolean;
}

export interface NeoForgeStatus {
  minecraft: string;
  current_version?: string;
  latest_version?: string;
  versions: NeoForgeVersion[];
  server_state: string;
  detail?: string;
}

export interface NeoForgeChangeResult {
  from_version: string;
  to_version: string;
  direction: 'upgrade' | 'downgrade' | 'reinstall' | string;
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
  crafty?: CraftyStatus;
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
  automodpack_group?: string;
  preferred_deployment: Location;
  preferred_automodpack_group?: string;
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
  automodpack_group?: string;
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
  automodpack_group?: string;
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
  automodpack_group?: string;
  environment?: string;
  manual_download?: boolean;
  manual_provided?: boolean;
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
  applied_at?: string;
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
  backup_id?: string;
  mods: number;
  summary: string;
}


export interface AutoModpackSettings {
  modpack_host: boolean;
  generate_modpack_on_start: boolean;
  auto_exclude_server_side_mods: boolean;
  require_modpack: boolean;
  accepted_loaders: string[];
  advertise_versions_to_sync: boolean;
  self_updater: boolean;
  connection_mode: string;
  bind_address: string;
  bind_port: number;
  advertised_endpoint_host: string;
  advertised_endpoint_port: number;
  bandwidth_limit: number;
  disable_internal_tls: boolean;
  accept_proxy_protocol: boolean;
  validate_secrets: boolean;
  secret_lifetime: number;
  export_http_directory: string;
  export_http_include_all: boolean;
  nag_unmodded_clients: boolean;
  nag_message: string;
  nag_clickable_message: string;
  nag_clickable_link: string;
}

export interface AutoModpackGroup {
  id: string;
  display_name: string;
  description: string;
  required: boolean;
  default_selected: boolean;
  requires: string[];
  breaks_with: string[];
  compatible_platforms: string[];
  from_server: string[];
  exclude: string[];
  editable: string[];
}

export interface AutoModpackCategory {
  name: string;
  groups: AutoModpackGroup[];
}

export interface AutoModpackConfig {
  name: string;
  settings: AutoModpackSettings;
  categories: AutoModpackCategory[];
}

export interface AutoModpackFinding {
  level: 'error' | 'warning' | 'info' | string;
  code: string;
  message: string;
  group?: string;
}

export interface AutoModpackPublishedFile {
  path: string;
  size?: string;
  type?: string;
  editable?: boolean;
  sha1?: string;
}

export interface AutoModpackGroupStatus {
  id: string;
  category: string;
  path: string;
  exists: boolean;
  files: number;
  mods: number;
  bytes: number;
  published_file_count: number;
}

export interface AutoModpackPublishedFilesPage {
  group: string;
  files: AutoModpackPublishedFile[];
  offset: number;
  limit: number;
  total: number;
  has_more: boolean;
  query?: string;
}

export interface AutoModpackGenerationDiffEntry {
  path: string;
  action: 'add' | 'change' | 'remove';
  current_sha1?: string;
  current_size?: number;
  target_sha1?: string;
  target_size?: number;
}

export interface AutoModpackGenerationDiff {
  target_sequence: number;
  head_sequence: number;
  added: number;
  changed: number;
  removed: number;
  entries: AutoModpackGenerationDiffEntry[];
}

export interface AutoModpackGeneration {
  sequence: number;
  content_token: string;
  created_at: string;
  notes?: string;
  restore_of?: number;
  summary: {
    added: number;
    changed: number;
    removed: number;
  };
}

export interface AutoModpackStatus {
  installed: boolean;
  version?: string;
  jar?: string;
  config_present: boolean;
  config_path: string;
  config_sha256?: string;
  raw_config?: string;
  config: AutoModpackConfig;
  findings: AutoModpackFinding[];
  groups: AutoModpackGroupStatus[];
  orphan_group_directories?: string[];
  pending_publish: boolean;
  last_changed_at?: string;
  last_publish_requested_at?: string;
  generations: AutoModpackGeneration[];
  published_content_token?: string;
  published_journal_head?: number;
}

export interface AutoModpackActionResult {
  action: string;
  command: string;
  status: string;
  requested_at: string;
  message: string;
  output: string[];
  output_error?: string;
}
