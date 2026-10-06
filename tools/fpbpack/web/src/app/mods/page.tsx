'use client';

import {useEffect, useMemo, useState} from 'react';
import {useRouter} from 'next/navigation';
import {
  Check,
  ExternalLink,
  GitBranch,
  PackageMinus,
  PackagePlus,
  RefreshCw,
  Search,
  ShieldOff,
  Trash2,
  Wrench,
  X,
} from 'lucide-react';
import {CatalogManagerDialog} from '@/components/catalog-manager';
import {PageHeader, Pill, formatDate} from '@/components/ui';
import {useManagement} from '@/components/management-provider';
import {api} from '@/lib/api';
import type {
  AutoModpackStatus,
  DiagnosticFinding,
  ManagementMod,
  UpdateCandidate,
  UpdatePlan,
  UpdateRule,
} from '@/lib/management';

const PAGE_SIZE = 50;
const FILTER_STORAGE_KEY = 'fpbpack:mods-filters:v1';

interface ModManagementResult {
  action: string;
  path: string;
  management?: string;
  provider?: string;
  project_id?: string;
  message: string;
}

type ModTableRow =
  | {kind: 'mod'; mod: ManagementMod}
  | {
      kind: 'diagnostic';
      finding: DiagnosticFinding;
      deployment: 'server' | 'client';
      management: 'unresolved' | 'external';
    };

function normalizePath(value?: string) {
  return (value ?? '').replaceAll('\\', '/').replace(/^\.\//, '');
}

function normalizedGroup(mod: ManagementMod, preferred = false) {
  const deployment = preferred ? mod.preferred_deployment : mod.deployment;
  if (deployment !== 'client') return '';
  return (
    (preferred ? mod.preferred_automodpack_group : mod.automodpack_group)?.trim() ||
    'main'
  );
}

function placementLabel(mod: ManagementMod, preferred = false) {
  const deployment = preferred ? mod.preferred_deployment : mod.deployment;
  return deployment === 'client'
    ? 'AutoModpack/' + normalizedGroup(mod, preferred)
    : 'server/common';
}

function pathAutoModpackGroup(path?: string) {
  const normalized = normalizePath(path);
  const prefix = 'automodpack/host-modpack/';
  if (!normalized.startsWith(prefix)) return '';
  const rest = normalized.slice(prefix.length);
  return rest.split('/')[0] || '';
}

function managementTone(
  management: ManagementMod['management'],
): 'good' | 'warn' | 'bad' | 'neutral' {
  if (management === 'managed') return 'good';
  if (management === 'unmanaged') return 'warn';
  if (management === 'unresolved' || management === 'external') return 'bad';
  return 'neutral';
}

function updateTone(candidate?: UpdateCandidate): 'good' | 'warn' | 'bad' | 'neutral' {
  if (!candidate || candidate.classification === 'up_to_date') return 'good';
  if (candidate.classification === 'safe') return 'good';
  if (candidate.classification === 'review') return 'warn';
  if (candidate.classification === 'blocked') return 'bad';
  return 'neutral';
}

export default function ModsPage() {
  const {state, connectionStatus, reload} = useManagement();
  const router = useRouter();
  const [q, setQ] = useState('');
  const [deployment, setDeployment] = useState('all');
  const [autoModpackGroupFilter, setAutoModpackGroupFilter] = useState('all');
  const [management, setManagement] = useState('all');
  const [provider, setProvider] = useState('all');
  const [updateStatus, setUpdateStatus] = useState('all');
  const [attentionOnly, setAttentionOnly] = useState(false);
  const [page, setPage] = useState(1);
  const [filtersReady, setFiltersReady] = useState(false);
  const [selectedMod, setSelectedMod] = useState<ManagementMod | null>(null);
  const [rules, setRules] = useState<Record<string, UpdateRule>>({});
  const [ruleError, setRuleError] = useState<string | null>(null);
  const [managementError, setManagementError] = useState<string | null>(null);
  const [managementMessage, setManagementMessage] = useState<string | null>(null);
  const [managementBusy, setManagementBusy] = useState(false);
  const [sourceProvider, setSourceProvider] = useState<'modrinth' | 'curseforge' | 'github'>('github');
  const [modrinthProjectID, setModrinthProjectID] = useState('');
  const [modrinthVersionID, setModrinthVersionID] = useState('');
  const [curseForgeProjectID, setCurseForgeProjectID] = useState('');
  const [curseForgeFileID, setCurseForgeFileID] = useState('');
  const [githubRepository, setGithubRepository] = useState('');
  const [githubTag, setGithubTag] = useState('');
  const [githubAsset, setGithubAsset] = useState('');
  const [preferredPlacement, setPreferredPlacement] = useState<'server' | 'client'>('server');
  const [preferredAutoModpackGroup, setPreferredAutoModpackGroup] = useState('main');
  const [autoModpackStatus, setAutoModpackStatus] = useState<AutoModpackStatus | null>(null);
  const [replacementPath, setReplacementPath] = useState('');
  const [catalogDialog, setCatalogDialog] = useState<
    {mode: 'install' | 'version'; mod?: ManagementMod} | null
  >(null);

  const candidatesByKey = useMemo(
    () => new Map(state.updates.candidates.map((candidate) => [candidate.key, candidate])),
    [state.updates.candidates],
  );

  const blockers = useMemo(
    () => state.diagnostics.findings.filter((finding) => finding.level === 'blocking'),
    [state.diagnostics.findings],
  );

  const blockerPaths = useMemo(
    () => new Set(blockers.map((finding) => normalizePath(finding.path)).filter(Boolean)),
    [blockers],
  );

  const missingManagedChoices = useMemo(
    () =>
      blockers
        .filter(
          (finding) =>
            finding.code === 'managed_artifact_missing' && !!normalizePath(finding.path),
        )
        .map((finding) => ({
          path: normalizePath(finding.path),
          label:
            finding.mod ||
            normalizePath(finding.path).split('/').at(-1) ||
            normalizePath(finding.path),
        })),
    [blockers],
  );
  const blockerRank = useMemo(() => {
    const rank = new Map<string, number>();
    blockers.forEach((finding, index) => {
      const path = normalizePath(finding.path);
      if (path && !rank.has(path)) rank.set(path, index);
    });
    return rank;
  }, [blockers]);

  const livePaths = useMemo(
    () => new Set(state.mods.map((mod) => normalizePath(mod.path))),
    [state.mods],
  );

  const diagnosticRows = useMemo<ModTableRow[]>(() => {
    const needle = q.trim().toLowerCase();
    return blockers
      .filter((finding) => {
        const path = normalizePath(finding.path);
        if (!path || livePaths.has(path)) return false;

        const inferredGroup = pathAutoModpackGroup(path);
        const inferredDeployment: 'server' | 'client' = inferredGroup ? 'client' : 'server';
        const inferredManagement: 'unresolved' | 'external' =
          finding.code === 'unresolved_artifact' ? 'unresolved' : 'external';
        const text = [finding.mod ?? '', finding.message, finding.code, path]
          .join(' ')
          .toLowerCase();

        return (
          (!needle || text.includes(needle)) &&
          (deployment === 'all' || deployment === inferredDeployment) &&
          (autoModpackGroupFilter === 'all' ||
            (inferredDeployment === 'client' && inferredGroup === autoModpackGroupFilter)) &&
          (management === 'all' || management === inferredManagement) &&
          provider === 'all' &&
          (updateStatus === 'all' || updateStatus === 'blocked')
        );
      })
      .map((finding) => {
        const path = normalizePath(finding.path);
        return {
          kind: 'diagnostic' as const,
          finding,
          deployment: pathAutoModpackGroup(path) ? 'client' : 'server',
          management:
            finding.code === 'unresolved_artifact' ? ('unresolved' as const) : ('external' as const),
        };
      });
  }, [
    blockers,
    livePaths,
    q,
    deployment,
    autoModpackGroupFilter,
    management,
    provider,
    updateStatus,
  ]);

  useEffect(() => {
    api<{rules: Record<string, UpdateRule>}>('/api/update-rules')
      .then((response) => setRules(response.rules))
      .catch(() => undefined);
    api<AutoModpackStatus>('/api/automodpack')
      .then(setAutoModpackStatus)
      .catch(() => undefined);

    const params = new URLSearchParams(window.location.search);
    let persisted: Record<string, unknown> = {};
    try {
      persisted = JSON.parse(window.localStorage.getItem(FILTER_STORAGE_KEY) ?? '{}') as Record<
        string,
        unknown
      >;
    } catch {
      persisted = {};
    }

    const stringValue = (key: string, fallback: string) => {
      const fromURL = params.get(key);
      if (fromURL !== null) return fromURL;
      return typeof persisted[key] === 'string' ? String(persisted[key]) : fallback;
    };
    setQ(stringValue('q', ''));
    setDeployment(stringValue('deployment', 'all'));
    setAutoModpackGroupFilter(stringValue('group', 'all'));
    setManagement(stringValue('management', 'all'));
    setProvider(stringValue('provider', 'all'));
    setUpdateStatus(stringValue('update', 'all'));
    setAttentionOnly(
      params.has('attention')
        ? params.get('attention') === '1'
        : persisted.attentionOnly === true,
    );
    const restoredPage = Number(params.get('page') ?? persisted.page ?? 1);
    setPage(Number.isFinite(restoredPage) && restoredPage > 0 ? Math.floor(restoredPage) : 1);
    setFiltersReady(true);
  }, []);

  useEffect(() => {
    if (!filtersReady) return;
    const value = {
      q,
      deployment,
      group: autoModpackGroupFilter,
      management,
      provider,
      update: updateStatus,
      attentionOnly,
      page,
    };
    window.localStorage.setItem(FILTER_STORAGE_KEY, JSON.stringify(value));

    const params = new URLSearchParams(window.location.search);
    const sync = (key: string, value: string, defaultValue: string) => {
      if (value === defaultValue) params.delete(key);
      else params.set(key, value);
    };
    sync('q', q, '');
    sync('deployment', deployment, 'all');
    sync('group', autoModpackGroupFilter, 'all');
    sync('management', management, 'all');
    sync('provider', provider, 'all');
    sync('update', updateStatus, 'all');
    if (attentionOnly) params.set('attention', '1');
    else params.delete('attention');
    if (page > 1) params.set('page', String(page));
    else params.delete('page');
    const query = params.toString();
    window.history.replaceState(null, '', window.location.pathname + (query ? '?' + query : ''));
  }, [
    filtersReady,
    q,
    deployment,
    autoModpackGroupFilter,
    management,
    provider,
    updateStatus,
    attentionOnly,
    page,
  ]);

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
      const candidate = candidatesByKey.get(mod.id);
      const statusMatches =
        updateStatus === 'all' ||
        (updateStatus === 'available' &&
          !!candidate?.target &&
          candidate.classification !== 'ignored') ||
        (updateStatus === 'ignored' && candidate?.classification === 'ignored') ||
        (updateStatus === 'blocked' && candidate?.classification === 'blocked') ||
        (updateStatus === 'up_to_date' && candidate?.classification === 'up_to_date');
      const needsAttention =
        blockerPaths.has(normalizePath(mod.path)) ||
        mod.management === 'unresolved' ||
        mod.management === 'external';

      return (
        (!needle || text.includes(needle)) &&
        (deployment === 'all' || mod.deployment === deployment) &&
        (autoModpackGroupFilter === 'all' ||
          (mod.deployment === 'client' && normalizedGroup(mod) === autoModpackGroupFilter)) &&
        (management === 'all' || mod.management === management) &&
        (provider === 'all' || mod.provider === provider) &&
        statusMatches &&
        (!attentionOnly || needsAttention)
      );
    }).sort((a, b) => {
      if (attentionOnly) {
        const aRank = blockerRank.get(normalizePath(a.path)) ?? Number.MAX_SAFE_INTEGER;
        const bRank = blockerRank.get(normalizePath(b.path)) ?? Number.MAX_SAFE_INTEGER;
        if (aRank !== bRank) return aRank - bRank;
      }
      return a.name.localeCompare(b.name);
    });
  }, [
    state.mods,
    candidatesByKey,
    blockerPaths,
    blockerRank,
    q,
    deployment,
    autoModpackGroupFilter,
    management,
    provider,
    updateStatus,
    attentionOnly,
  ]);

  const tableRows = useMemo<ModTableRow[]>(() => {
    const combined: ModTableRow[] = [
      ...filtered.map((mod) => ({kind: 'mod' as const, mod})),
      ...diagnosticRows,
    ];
    return combined.sort((a, b) => {
      const aPath =
        a.kind === 'mod' ? normalizePath(a.mod.path) : normalizePath(a.finding.path);
      const bPath =
        b.kind === 'mod' ? normalizePath(b.mod.path) : normalizePath(b.finding.path);
      if (attentionOnly) {
        const aRank = blockerRank.get(aPath) ?? Number.MAX_SAFE_INTEGER;
        const bRank = blockerRank.get(bPath) ?? Number.MAX_SAFE_INTEGER;
        if (aRank !== bRank) return aRank - bRank;
      }
      const aName =
        a.kind === 'mod'
          ? a.mod.name
          : a.finding.mod || a.finding.path?.split('/').at(-1) || a.finding.code;
      const bName =
        b.kind === 'mod'
          ? b.mod.name
          : b.finding.mod || b.finding.path?.split('/').at(-1) || b.finding.code;
      return aName.localeCompare(bName);
    });
  }, [filtered, diagnosticRows, attentionOnly, blockerRank]);

  const totalPages = Math.max(1, Math.ceil(tableRows.length / PAGE_SIZE));
  const safePage = Math.min(page, totalPages);
  const rows = tableRows.slice((safePage - 1) * PAGE_SIZE, safePage * PAGE_SIZE);

  const updateFilter = (setter: (value: string) => void, value: string) => {
    setter(value);
    setPage(1);
  };

  const selectedCandidate = selectedMod ? candidatesByKey.get(selectedMod.id) : undefined;
  const selectedTargetGroup =
    preferredPlacement === 'client' ? preferredAutoModpackGroup || 'main' : '';
  const selectedPreferenceChanged = selectedMod
    ? preferredPlacement !== selectedMod.preferred_deployment ||
      (preferredPlacement === 'client' &&
        selectedTargetGroup !== normalizedGroup(selectedMod, true))
    : false;
  const selectedLiveMoveNeeded = selectedMod
    ? preferredPlacement !== selectedMod.deployment ||
      (preferredPlacement === 'client' &&
        selectedTargetGroup !== normalizedGroup(selectedMod))
    : false;
  const selectedRule = selectedCandidate ? rules[selectedCandidate.key] : undefined;
  const selectedBlockers = useMemo(
    () =>
      selectedMod
        ? blockers.filter(
            (finding) =>
              normalizePath(finding.path) === normalizePath(selectedMod.path) ||
              (!finding.path && finding.mod === selectedMod.name),
          )
        : [],
    [selectedMod, blockers],
  );

  const openMod = (mod: ManagementMod) => {
    const candidate = candidatesByKey.get(mod.id);
    setSelectedMod(mod);
    setManagementError(null);
    setManagementMessage(null);
    setSourceProvider(
      mod.provider === 'modrinth' || mod.provider === 'curseforge' || mod.provider === 'github'
        ? mod.provider
        : 'github',
    );
    setModrinthProjectID(mod.provider === 'modrinth' ? mod.project_id ?? '' : '');
    setModrinthVersionID(mod.provider === 'modrinth' ? candidate?.installed.id ?? '' : '');
    setCurseForgeProjectID(mod.provider === 'curseforge' ? mod.project_id ?? '' : '');
    setCurseForgeFileID(mod.provider === 'curseforge' ? candidate?.installed.id ?? '' : '');
    setGithubRepository(mod.provider === 'github' ? mod.project_id ?? '' : '');
    setGithubTag(mod.provider === 'github' ? candidate?.installed.id ?? '' : '');
    setGithubAsset(mod.filename);
    setPreferredPlacement(mod.preferred_deployment ?? mod.deployment);
    setPreferredAutoModpackGroup(
      mod.preferred_automodpack_group ?? mod.automodpack_group ?? 'main',
    );
    setReplacementPath('');
    const params = new URLSearchParams(window.location.search);
    params.set('mod', mod.id);
    const query = params.toString();
    window.history.replaceState(null, '', window.location.pathname + (query ? '?' + query : ''));
  };

  const closeMod = () => {
    setSelectedMod(null);
    const params = new URLSearchParams(window.location.search);
    params.delete('mod');
    const query = params.toString();
    window.history.replaceState(null, '', window.location.pathname + (query ? '?' + query : ''));
  };

  useEffect(() => {
    if (!filtersReady || selectedMod || state.mods.length === 0) return;
    const requested = new URLSearchParams(window.location.search).get('mod');
    if (!requested) return;
    const mod = state.mods.find(
      (item) => item.id === requested || normalizePath(item.path) === normalizePath(requested),
    );
    if (mod) openMod(mod);
  }, [filtersReady, state.mods, selectedMod]);

  const applyRule = async (candidate: UpdateCandidate, rule: UpdateRule) => {
    setRuleError(null);
    try {
      await api('/api/update-rules', {
        method: 'PUT',
        body: JSON.stringify({key: candidate.key, rule}),
      });
      setRules((current) => ({...current, [candidate.key]: rule}));
      await reload();
    } catch (error: unknown) {
      setRuleError(error instanceof Error ? error.message : String(error));
    }
  };

  const clearRule = async (candidate: UpdateCandidate) => {
    setRuleError(null);
    try {
      await api('/api/update-rules?key=' + encodeURIComponent(candidate.key), {
        method: 'DELETE',
      });
      setRules((current) => {
        const next = {...current};
        delete next[candidate.key];
        return next;
      });
      await reload();
    } catch (error: unknown) {
      setRuleError(error instanceof Error ? error.message : String(error));
    }
  };

  const manageMod = async (
    action:
      | 'mark_unmanaged'
      | 'assign_modrinth'
      | 'assign_curseforge'
      | 'assign_github'
      | 'forget_missing'
      | 'set_placement'
      | 'adopt_current',
    path: string,
    extra?: {
      project_id?: string;
      version_id?: string;
      file_id?: number;
      repository?: string;
      tag?: string;
      asset?: string;
      placement?: 'server' | 'client';
      automodpack_group?: string;
      replaces_path?: string;
    },
  ) => {
    setManagementBusy(true);
    setManagementError(null);
    setManagementMessage(null);
    try {
      const result = await api<ModManagementResult>('/api/mod-management', {
        method: 'POST',
        body: JSON.stringify({action, path, ...extra}),
      });
      setManagementMessage(result.message);
      closeMod();
      await reload({silent: true});
    } catch (error: unknown) {
      setManagementError(error instanceof Error ? error.message : String(error));
    } finally {
      setManagementBusy(false);
    }
  };

  const savePlacement = async (reviewMove: boolean) => {
    if (!selectedMod) return;
    setManagementBusy(true);
    setManagementError(null);
    setManagementMessage(null);
    try {
      const targetGroup = preferredPlacement === 'client' ? preferredAutoModpackGroup || 'main' : '';
      const preferredChanged =
        preferredPlacement !== selectedMod.preferred_deployment ||
        (preferredPlacement === 'client' &&
          targetGroup !== normalizedGroup(selectedMod, true));
      if (preferredChanged) {
        await api<ModManagementResult>('/api/mod-management', {
          method: 'POST',
          body: JSON.stringify({
            action: 'set_placement',
            path: selectedMod.path,
            placement: preferredPlacement,
            automodpack_group: preferredPlacement === 'client' ? targetGroup : undefined,
          }),
        });
      }

      const liveMoveNeeded =
        preferredPlacement !== selectedMod.deployment ||
        (preferredPlacement === 'client' && targetGroup !== normalizedGroup(selectedMod));
      if (reviewMove && liveMoveNeeded) {
        const plan = await api<UpdatePlan>('/api/placement-plans', {
          method: 'POST',
          body: JSON.stringify({path: selectedMod.path}),
        });
        await reload({silent: true});
        closeMod();
        router.push('/review?id=' + encodeURIComponent(plan.id));
        return;
      }

      setManagementMessage(
        liveMoveNeeded
          ? 'Preferred placement saved. Review a protected move when you are ready.'
          : 'Preferred placement now matches the live JAR.',
      );
      await reload({silent: true});
      closeMod();
    } catch (error: unknown) {
      setManagementError(error instanceof Error ? error.message : String(error));
    } finally {
      setManagementBusy(false);
    }
  };

  const openVersionManager = (mod: ManagementMod) => {
    closeMod();
    setCatalogDialog({mode: 'version', mod});
  };

  const reviewRemoval = async (mod: ManagementMod) => {
    setManagementBusy(true);
    setManagementError(null);
    setManagementMessage(null);
    try {
      const plan = await api<UpdatePlan>('/api/catalog/plans', {
        method: 'POST',
        body: JSON.stringify({action: 'remove', path: mod.path}),
      });
      closeMod();
      router.push('/review?id=' + encodeURIComponent(plan.id));
    } catch (error: unknown) {
      setManagementError(error instanceof Error ? error.message : String(error));
    } finally {
      setManagementBusy(false);
    }
  };

  const refreshModMetadata = async (path: string) => {
    setManagementBusy(true);
    setManagementError(null);
    setManagementMessage(null);
    try {
      await api<{status: string}>('/api/mod-metadata/refresh', {
        method: 'POST',
        body: JSON.stringify({path}),
      });
      setManagementMessage('Metadata refresh started. You can close or reload this page; it will continue on the server.');
      await reload({silent: true});
    } catch (error: unknown) {
      setManagementError(error instanceof Error ? error.message : String(error));
    } finally {
      setManagementBusy(false);
    }
  };

  const findLiveMod = (finding: DiagnosticFinding) =>
    finding.path
      ? state.mods.find(
          (mod) => normalizePath(mod.path) === normalizePath(finding.path),
        )
      : undefined;

  return (
    <>
      <PageHeader
        eyebrow="Inventory"
        title="Mods"
        description="Installed server/common and AutoModpack client-only artifacts."
        action={
          <div className="flex items-center gap-2">
            <button
              className="btn btn-sm btn-primary"
              type="button"
              onClick={() => setCatalogDialog({mode: 'install'})}
            >
              <PackagePlus size={14} /> Add mod
            </button>
            {blockers.length ? <Pill tone="bad">{blockers.length} blocking</Pill> : null}
            <Pill tone={connectionStatus === 'connected' ? 'good' : 'warn'}>
              {state.mods.length} JARs
            </Pill>
          </div>
        }
      />

      {managementError ? (
        <div className="alert alert-error mb-4 rounded-box py-3 text-sm">{managementError}</div>
      ) : null}
      {managementMessage ? (
        <div className="alert alert-info mb-4 rounded-box py-3 text-sm">{managementMessage}</div>
      ) : null}

      {blockers.length ? (
        <section className="panel mb-4 overflow-hidden">
          <div className="panel-header">
            <div>
              <div className="section-label">Needs attention</div>
              <h2 className="mt-0.5 text-sm font-semibold">
                Resolve blocking catalog issues
              </h2>
            </div>
            <Pill tone="bad">{blockers.length}</Pill>
          </div>
          <div className="divide-y divide-base-300">
            {blockers.map((finding, index) => {
              const liveMod = findLiveMod(finding);
              const canForget =
                !!finding.path &&
                !liveMod &&
                (finding.code === 'managed_artifact_missing' ||
                  finding.code === 'unresolved_artifact');
              return (
                <div
                  className="flex flex-col gap-3 px-4 py-3 sm:flex-row sm:items-center"
                  key={finding.code + ':' + (finding.path ?? finding.mod ?? index)}
                >
                  <Wrench size={14} className="shrink-0 text-error" />
                  <div className="min-w-0 flex-1">
                    <div className="text-sm font-medium">{finding.mod || finding.code}</div>
                    <div className="mt-0.5 text-xs text-base-content/50">
                      {finding.message}
                    </div>
                    {finding.path ? (
                      <div className="mono mt-1 text-[0.68rem] text-base-content/35">
                        {finding.path}
                      </div>
                    ) : null}
                  </div>
                  <div className="flex shrink-0 flex-wrap gap-2">
                    {liveMod ? (
                      <button
                        className="btn btn-xs btn-primary"
                        type="button"
                        onClick={() => openMod(liveMod)}
                      >
                        Fix mod
                      </button>
                    ) : null}
                    {canForget ? (
                      <button
                        className="btn btn-xs btn-ghost"
                        type="button"
                        disabled={managementBusy}
                        onClick={() =>
                          void manageMod('forget_missing', finding.path ?? '')
                        }
                      >
                        <Trash2 size={12} /> Forget absent entry
                      </button>
                    ) : null}
                    {!liveMod && !canForget && finding.mod ? (
                      <button
                        className="btn btn-xs btn-ghost"
                        type="button"
                        onClick={() => {
                          setQ(finding.mod ?? '');
                          setAttentionOnly(false);
                          setPage(1);
                        }}
                      >
                        Find related mods
                      </button>
                    ) : null}
                  </div>
                </div>
              );
            })}
          </div>
        </section>
      ) : null}

      <div className="mb-3 grid gap-2 md:grid-cols-[minmax(220px,1fr)_repeat(5,minmax(125px,auto))_auto_auto]">
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
        <select
          className="select select-sm"
          value={updateStatus}
          onChange={(event) => updateFilter(setUpdateStatus, event.target.value)}
        >
          <option value="all">All update states</option>
          <option value="available">Updates available</option>
          <option value="blocked">Blocked</option>
          <option value="ignored">Ignored</option>
          <option value="up_to_date">Up to date</option>
        </select>
        <select
          className="select select-sm"
          value={deployment}
          onChange={(event) => updateFilter(setDeployment, event.target.value)}
        >
          <option value="all">All placements</option>
          <option value="server">Server/common</option>
          <option value="client">Client-only</option>
        </select>
        <select
          className="select select-sm"
          value={autoModpackGroupFilter}
          onChange={(event) => updateFilter(setAutoModpackGroupFilter, event.target.value)}
        >
          <option value="all">All AutoModpack groups</option>
          {[...new Set([
            ...(autoModpackStatus?.config.categories.flatMap((category) =>
              category.groups.map((group) => group.id),
            ) ?? []),
            ...state.mods
              .filter((mod) => mod.deployment === 'client')
              .map((mod) => normalizedGroup(mod)),
          ])]
            .filter(Boolean)
            .sort((a, b) => a.localeCompare(b))
            .map((group) => (
              <option key={group} value={group}>{group}</option>
            ))}
        </select>
        <select
          className="select select-sm"
          value={management}
          onChange={(event) => updateFilter(setManagement, event.target.value)}
        >
          <option value="all">All management</option>
          <option value="managed">Managed</option>
          <option value="unmanaged">Unmanaged</option>
          <option value="unresolved">Unresolved</option>
          <option value="external">External change</option>
        </select>
        <select
          className="select select-sm"
          value={provider}
          onChange={(event) => updateFilter(setProvider, event.target.value)}
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
          className={'btn btn-sm ' + (attentionOnly ? 'btn-error' : 'btn-ghost')}
          onClick={() => {
            setAttentionOnly((value) => !value);
            setPage(1);
          }}
        >
          <Wrench size={14} /> Needs attention
        </button>
        <button
          type="button"
          className="btn btn-sm btn-ghost"
          onClick={() => {
            setQ('');
            setUpdateStatus('all');
            setDeployment('all');
            setAutoModpackGroupFilter('all');
            setManagement('all');
            setProvider('all');
            setAttentionOnly(false);
            setPage(1);
          }}
        >
          <X size={14} /> Reset
        </button>
      </div>

      <section className="panel overflow-hidden">
        <div className="flex items-center justify-between border-b border-base-300 px-4 py-2 text-xs text-base-content/45">
          <span>{tableRows.length} matching entries</span>
          <span>
            Page {safePage} / {totalPages}
          </span>
        </div>
        <div className="overflow-x-auto">
          <table className="table table-sm">
            <thead>
              <tr>
                <th>Mod</th>
                <th>Installed</th>
                <th>Latest</th>
                <th>Status</th>
                <th>Placement</th>
                <th>Group</th>
                <th>Source</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((row) => {
                if (row.kind === 'diagnostic') {
                  const path = normalizePath(row.finding.path);
                  const name =
                    row.finding.mod || row.finding.path?.split('/').at(-1) || row.finding.code;
                  return (
                    <tr
                      key={'diagnostic:' + row.finding.code + ':' + path}
                      className="border-base-300 bg-error/5"
                    >
                      <td>
                        <div className="flex items-center gap-2">
                          <Wrench size={13} className="shrink-0 text-error" />
                          <div>
                            <div className="font-medium">{name}</div>
                            <div className="mono mt-0.5 text-base-content/35">{path}</div>
                          </div>
                        </div>
                      </td>
                      <td className="whitespace-nowrap text-base-content/45">missing</td>
                      <td className="whitespace-nowrap">—</td>
                      <td>
                        <Pill tone="bad">{row.management}</Pill>
                      </td>
                      <td className="whitespace-nowrap">
                        <Pill tone={row.deployment === 'client' ? 'blue' : 'neutral'}>
                          {row.deployment === 'client' ? 'AutoModpack' : 'server/common'}
                        </Pill>
                      </td>
                      <td className="whitespace-nowrap text-base-content/45">
                        {row.deployment === 'client' ? pathAutoModpackGroup(path) || 'main' : '—'}
                      </td>
                      <td>
                        <div className="flex items-center gap-2">
                          <span className="text-base-content/45">unknown</span>
                          {(row.finding.code === 'managed_artifact_missing' ||
                            row.finding.code === 'unresolved_artifact') && path ? (
                            <button
                              className="btn btn-xs btn-ghost"
                              type="button"
                              disabled={managementBusy}
                              onClick={() => void manageMod('forget_missing', path)}
                            >
                              <Trash2 size={11} /> Forget
                            </button>
                          ) : null}
                        </div>
                      </td>
                    </tr>
                  );
                }

                const mod = row.mod;
                const candidate = candidatesByKey.get(mod.id);
                const needsAttention =
                  blockerPaths.has(normalizePath(mod.path)) ||
                  mod.management === 'unresolved' ||
                  mod.management === 'external';
                return (
                  <tr
                    key={mod.id + ':' + mod.path}
                    className="cursor-pointer border-base-300 hover:bg-base-300/25"
                    onClick={() => openMod(mod)}
                  >
                    <td>
                      <div className="flex items-center gap-2">
                        {candidate?.icon_url ? (
                          <img
                            src={candidate.icon_url}
                            alt=""
                            className="size-7 rounded-md border border-base-300 object-cover"
                          />
                        ) : null}
                        <div>
                          <div className="flex items-center gap-2 font-medium">
                            {mod.name}
                            {needsAttention ? <Wrench size={12} className="text-error" /> : null}
                          </div>
                          <div className="mono mt-0.5 text-base-content/35">{mod.filename}</div>
                        </div>
                      </div>
                    </td>
                    <td className="whitespace-nowrap">
                      {mod.installed_version || candidate?.installed.number || '—'}
                    </td>
                    <td className="whitespace-nowrap">
                      {candidate?.target?.number ?? candidate?.installed.number ?? '—'}
                    </td>
                    <td>
                      {candidate ? (
                        <div className="flex flex-wrap gap-1">
                          <Pill tone={updateTone(candidate)}>
                            {candidate.classification.replaceAll('_', ' ')}
                          </Pill>
                          {candidate.metadata_stale ? <Pill tone="warn">stale</Pill> : null}
                        </div>
                      ) : (
                        <Pill tone={managementTone(mod.management)}>{mod.management}</Pill>
                      )}
                    </td>
                    <td className="whitespace-nowrap">
                      <div className="flex items-center gap-1.5">
                        <Pill tone={mod.deployment === 'client' ? 'blue' : 'neutral'}>
                          {mod.deployment === 'client' ? 'AutoModpack' : 'server/common'}
                        </Pill>
                        {placementLabel(mod, true) !== placementLabel(mod) ? (
                          <>
                            <span className="text-base-content/30">→</span>
                            <Pill tone="warn">
                              {mod.preferred_deployment === 'client' ? 'AutoModpack' : 'server/common'}
                            </Pill>
                          </>
                        ) : null}
                      </div>
                    </td>
                    <td className="whitespace-nowrap">
                      {mod.deployment === 'client' ? (
                        <div className="flex items-center gap-1.5">
                          <span className="mono text-xs">{normalizedGroup(mod)}</span>
                          {placementLabel(mod, true) !== placementLabel(mod) &&
                          mod.preferred_deployment === 'client' ? (
                            <>
                              <span className="text-base-content/30">→</span>
                              <span className="mono text-xs text-warning">
                                {normalizedGroup(mod, true)}
                              </span>
                            </>
                          ) : null}
                        </div>
                      ) : '—'}
                    </td>
                    <td>
                      {candidate?.project_url || mod.project_url ? (
                        <a
                          className="link link-hover text-sm"
                          href={candidate?.project_url || mod.project_url}
                          target="_blank"
                          rel="noreferrer"
                          onClick={(event) => event.stopPropagation()}
                        >
                          {mod.provider ?? candidate?.provider ?? 'unknown'}
                        </a>
                      ) : (
                        <span className="text-base-content/45">
                          {mod.provider ?? 'unknown'}
                        </span>
                      )}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
        <div className="flex items-center justify-between border-t border-base-300 px-3 py-2">
          <button
            className="btn btn-xs btn-ghost"
            type="button"
            disabled={safePage <= 1}
            onClick={() => setPage(safePage - 1)}
          >
            Previous
          </button>
          <button
            className="btn btn-xs btn-ghost"
            type="button"
            disabled={safePage >= totalPages}
            onClick={() => setPage(safePage + 1)}
          >
            Next
          </button>
        </div>
      </section>

      {catalogDialog ? (
        <CatalogManagerDialog
          mode={catalogDialog.mode}
          mod={catalogDialog.mod}
          onClose={() => setCatalogDialog(null)}
        />
      ) : null}

      {selectedMod ? (
        <div
          className="modal modal-open"
          role="dialog"
          aria-modal="true"
          aria-label={'Details for ' + selectedMod.name}
        >
          <div className="modal-box max-h-[90vh] max-w-3xl overflow-y-auto max-sm:h-full max-sm:max-h-none max-sm:w-full max-sm:rounded-none">
            <div className="flex items-start gap-3 border-b border-base-300 pb-4">
              {selectedCandidate?.icon_url ? (
                <img
                  src={selectedCandidate.icon_url}
                  alt=""
                  className="size-12 rounded-lg border border-base-300 object-cover"
                />
              ) : (
                <div className="grid size-12 place-items-center rounded-lg border border-base-300 bg-base-200 text-lg font-semibold text-base-content/50">
                  {selectedMod.name.slice(0, 1).toUpperCase()}
                </div>
              )}
              <div className="min-w-0 flex-1">
                <h2 className="text-lg font-semibold">{selectedMod.name}</h2>
                <div className="mt-1 flex flex-wrap items-center gap-2">
                  <Pill tone={managementTone(selectedMod.management)}>
                    {selectedMod.management}
                  </Pill>
                  {selectedCandidate ? (
                    <Pill tone={updateTone(selectedCandidate)}>
                      {selectedCandidate.classification.replaceAll('_', ' ')}
                    </Pill>
                  ) : null}
                  {selectedCandidate?.metadata_stale ? (
                    <Pill tone="warn">stale metadata</Pill>
                  ) : null}
                </div>
              </div>
              <button
                className="btn btn-sm btn-ghost btn-square"
                type="button"
                onClick={closeMod}
                aria-label="Close"
              >
                <X size={16} />
              </button>
            </div>

            {ruleError ? (
              <div className="alert alert-error mt-4 py-2 text-xs">{ruleError}</div>
            ) : null}
            {managementError ? (
              <div className="alert alert-error mt-4 py-2 text-xs">{managementError}</div>
            ) : null}
            {managementMessage ? (
              <div className="alert alert-info mt-4 py-2 text-xs">{managementMessage}</div>
            ) : null}

            {selectedBlockers.length ? (
              <div className="alert alert-error mt-4 items-start py-3 text-xs">
                <Wrench size={15} className="mt-0.5 shrink-0" />
                <div>
                  <div className="font-semibold">
                    {selectedBlockers.length} blocking issue{selectedBlockers.length === 1 ? '' : 's'}
                  </div>
                  <ul className="mt-1 space-y-1 text-error-content/80">
                    {selectedBlockers.map((finding, index) => (
                      <li key={finding.code + ':' + index}>{finding.message}</li>
                    ))}
                  </ul>
                </div>
              </div>
            ) : null}

            <section className="panel mt-4">
              <div className="panel-header">
                <div>
                  <div className="section-label">Management</div>
                  <h3 className="mt-0.5 text-sm font-semibold">Source &amp; metadata</h3>
                </div>
              </div>
              <div className="space-y-4 p-4">
                {selectedMod.management === 'external' && missingManagedChoices.length ? (
                  <div className="rounded-box border border-base-300 bg-base-200/35 p-3">
                    <div className="text-xs font-semibold">Manual replacement</div>
                    <p className="mt-1 text-xs text-base-content/45">
                      If this JAR is a renamed manual update of a missing managed artifact,
                      select the old artifact. FPBPack will verify the new JAR against that
                      artifact's provider before adopting it.
                    </p>
                    <select
                      className="select select-sm select-bordered mt-2 w-full"
                      value={replacementPath}
                      onChange={(event) => setReplacementPath(event.target.value)}
                    >
                      <option value="">Auto-detect / new external mod</option>
                      {missingManagedChoices.map((choice) => (
                        <option value={choice.path} key={choice.path}>
                          {choice.label} · {choice.path}
                        </option>
                      ))}
                    </select>
                  </div>
                ) : null}

                <div className="flex flex-wrap gap-2">
                  <button
                    className="btn btn-sm btn-primary"
                    type="button"
                    disabled={managementBusy || selectedMod.management === 'unmanaged'}
                    onClick={() => void refreshModMetadata(selectedMod.path)}
                  >
                    <RefreshCw
                      size={14}
                      className={
                        state.status.refresh?.refreshing ? 'animate-spin' : ''
                      }
                    />
                    {selectedMod.management === 'managed'
                      ? 'Refresh metadata'
                      : 'Try auto-detect source'}
                  </button>
                  {selectedMod.management === 'managed' &&
                  (selectedMod.provider === 'modrinth' ||
                    selectedMod.provider === 'curseforge') ? (
                    <>
                      <button
                        className="btn btn-sm btn-outline"
                        type="button"
                        disabled={managementBusy}
                        onClick={() => openVersionManager(selectedMod)}
                      >
                        <GitBranch size={14} /> Change version
                      </button>
                      <button
                        className="btn btn-sm btn-outline btn-error"
                        type="button"
                        disabled={managementBusy}
                        onClick={() => void reviewRemoval(selectedMod)}
                      >
                        <PackageMinus size={14} /> Review removal
                      </button>
                    </>
                  ) : null}
                  {selectedMod.management !== 'unmanaged' ? (
                    <button
                      className="btn btn-sm btn-ghost"
                      type="button"
                      disabled={managementBusy}
                      onClick={() =>
                        void manageMod('mark_unmanaged', selectedMod.path)
                      }
                    >
                      <ShieldOff size={14} /> Keep unmanaged
                    </button>
                  ) : null}
                  {(selectedBlockers.some((finding) => finding.code === 'managed_artifact_replaced') ||
                    selectedMod.management === 'external') ? (
                    <button
                      className="btn btn-sm btn-outline"
                      type="button"
                      disabled={managementBusy}
                      onClick={() =>
                        void manageMod('adopt_current', selectedMod.path, {
                          replaces_path: replacementPath || undefined,
                        })
                      }
                    >
                      <Check size={14} /> Adopt current JAR
                    </button>
                  ) : null}
                </div>
                <p className="text-xs text-base-content/45">
                  Auto-detect retries exact provider identification/metadata for this mod only.
                  Adopt current JAR is for an intentional manual replacement: FPBPack accepts it
                  only after proving it belongs to the same provider project/repository. Keeping
                  it unmanaged excludes it from update planning without touching the file.
                </p>

                <div className="border-t border-base-300 pt-4">
                  <div className="mb-2 text-xs font-semibold">Placement</div>
                  <div className="grid gap-2 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto_auto] sm:items-end">
                    <label className="form-control">
                      <span className="mb-1 text-xs text-base-content/45">Preferred placement</span>
                      <select
                        className="select select-sm select-bordered"
                        value={preferredPlacement}
                        disabled={selectedMod.management !== 'managed' || managementBusy}
                        onChange={(event) =>
                          setPreferredPlacement(event.target.value as 'server' | 'client')
                        }
                      >
                        <option value="server">Server/common</option>
                        <option value="client">AutoModpack</option>
                      </select>
                    </label>
                    <label className="form-control">
                      <span className="mb-1 text-xs text-base-content/45">AutoModpack group</span>
                      <select
                        className="select select-sm select-bordered"
                        value={preferredAutoModpackGroup}
                        disabled={
                          preferredPlacement !== 'client' ||
                          selectedMod.management !== 'managed' ||
                          managementBusy
                        }
                        onChange={(event) => setPreferredAutoModpackGroup(event.target.value)}
                      >
                        {(autoModpackStatus?.config.categories.flatMap((category) =>
                          category.groups.map((group) => (
                            <option key={group.id} value={group.id}>
                              {group.display_name || group.id} · {category.name}
                            </option>
                          )),
                        ) ?? [
                          <option key="main" value="main">
                            main
                          </option>,
                        ])}
                      </select>
                    </label>
                    <button
                      className="btn btn-sm btn-ghost"
                      type="button"
                      disabled={
                        managementBusy ||
                        selectedMod.management !== 'managed' ||
                        !selectedPreferenceChanged
                      }
                      onClick={() => void savePlacement(false)}
                    >
                      Save preference
                    </button>
                    <button
                      className="btn btn-sm btn-primary"
                      type="button"
                      disabled={
                        managementBusy ||
                        selectedMod.management !== 'managed' ||
                        !selectedLiveMoveNeeded
                      }
                      onClick={() => void savePlacement(true)}
                    >
                      Review move
                    </button>
                  </div>
                  <p className="mt-2 text-xs text-base-content/40">
                    Current: {placementLabel(selectedMod)}. Preferred: {placementLabel(selectedMod, true)}.
                    Saving only updates the preferred target. Review move creates a verified,
                    backed-up plan; this includes moves between two AutoModpack groups.
                  </p>
                </div>

                <div className="border-t border-base-300 pt-4">
                  <div className="mb-3 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                    <div className="flex items-center gap-2">
                      <GitBranch size={14} />
                      <span className="text-xs font-semibold">Assign verified source</span>
                    </div>
                    <select
                      className="select select-sm select-bordered"
                      value={sourceProvider}
                      onChange={(event) =>
                        setSourceProvider(
                          event.target.value as 'modrinth' | 'curseforge' | 'github',
                        )
                      }
                    >
                      <option value="modrinth">Modrinth</option>
                      <option value="curseforge">CurseForge</option>
                      <option value="github">GitHub release</option>
                    </select>
                  </div>

                  {sourceProvider === 'modrinth' ? (
                    <div>
                      <div className="grid gap-2 sm:grid-cols-2">
                        <label className="form-control">
                          <span className="mb-1 text-xs text-base-content/45">Project ID</span>
                          <input
                            className="input input-sm input-bordered"
                            value={modrinthProjectID}
                            onChange={(event) => setModrinthProjectID(event.target.value)}
                            placeholder="Modrinth project ID"
                          />
                        </label>
                        <label className="form-control">
                          <span className="mb-1 text-xs text-base-content/45">Installed version ID</span>
                          <input
                            className="input input-sm input-bordered"
                            value={modrinthVersionID}
                            onChange={(event) => setModrinthVersionID(event.target.value)}
                            placeholder="Version ID"
                          />
                        </label>
                      </div>
                      <div className="mt-2 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                        <p className="text-xs text-base-content/40">
                          Accepted only when the selected Modrinth version contains this exact
                          installed JAR SHA-512.
                        </p>
                        <button
                          className="btn btn-sm btn-primary"
                          type="button"
                          disabled={
                            managementBusy ||
                            !modrinthProjectID.trim() ||
                            !modrinthVersionID.trim()
                          }
                          onClick={() =>
                            void manageMod('assign_modrinth', selectedMod.path, {
                              project_id: modrinthProjectID,
                              version_id: modrinthVersionID,
                            })
                          }
                        >
                          Verify &amp; assign
                        </button>
                      </div>
                    </div>
                  ) : null}

                  {sourceProvider === 'curseforge' ? (
                    <div>
                      <div className="grid gap-2 sm:grid-cols-2">
                        <label className="form-control">
                          <span className="mb-1 text-xs text-base-content/45">Project ID</span>
                          <input
                            className="input input-sm input-bordered"
                            value={curseForgeProjectID}
                            onChange={(event) => setCurseForgeProjectID(event.target.value)}
                            placeholder="CurseForge project ID"
                          />
                        </label>
                        <label className="form-control">
                          <span className="mb-1 text-xs text-base-content/45">Installed file ID</span>
                          <input
                            className="input input-sm input-bordered"
                            inputMode="numeric"
                            value={curseForgeFileID}
                            onChange={(event) => setCurseForgeFileID(event.target.value)}
                            placeholder="File ID"
                          />
                        </label>
                      </div>
                      <div className="mt-2 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                        <p className="text-xs text-base-content/40">
                          Requires the CurseForge key from Settings. Accepted only when the
                          provider SHA-1 matches this installed JAR.
                        </p>
                        <button
                          className="btn btn-sm btn-primary"
                          type="button"
                          disabled={
                            managementBusy ||
                            !curseForgeProjectID.trim() ||
                            !/^\d+$/.test(curseForgeFileID.trim())
                          }
                          onClick={() =>
                            void manageMod('assign_curseforge', selectedMod.path, {
                              project_id: curseForgeProjectID,
                              file_id: Number(curseForgeFileID),
                            })
                          }
                        >
                          Verify &amp; assign
                        </button>
                      </div>
                    </div>
                  ) : null}

                  {sourceProvider === 'github' ? (
                    <div>
                      <div className="grid gap-2 sm:grid-cols-2">
                        <label className="form-control">
                          <span className="mb-1 text-xs text-base-content/45">Repository</span>
                          <input
                            className="input input-sm input-bordered"
                            value={githubRepository}
                            onChange={(event) => setGithubRepository(event.target.value)}
                            placeholder="owner/repository or GitHub URL"
                          />
                        </label>
                        <label className="form-control">
                          <span className="mb-1 text-xs text-base-content/45">Installed release tag</span>
                          <input
                            className="input input-sm input-bordered"
                            value={githubTag}
                            onChange={(event) => setGithubTag(event.target.value)}
                            placeholder="v1.2.3"
                          />
                        </label>
                        <label className="form-control sm:col-span-2">
                          <span className="mb-1 text-xs text-base-content/45">Release asset</span>
                          <input
                            className="input input-sm input-bordered"
                            value={githubAsset}
                            onChange={(event) => setGithubAsset(event.target.value)}
                            placeholder={selectedMod.filename}
                          />
                        </label>
                      </div>
                      <div className="mt-2 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                        <p className="text-xs text-base-content/40">
                          Repository may be owner/repository or a GitHub repository URL. Accepted
                          only if the live JAR SHA-256 matches the selected release asset digest.
                        </p>
                        <button
                          className="btn btn-sm btn-primary"
                          type="button"
                          disabled={
                            managementBusy ||
                            !githubRepository.trim() ||
                            !githubTag.trim()
                          }
                          onClick={() =>
                            void manageMod('assign_github', selectedMod.path, {
                              repository: githubRepository,
                              tag: githubTag,
                              asset: githubAsset,
                            })
                          }
                        >
                          <GitBranch size={14} /> Verify &amp; assign
                        </button>
                      </div>
                    </div>
                  ) : null}
                </div>
              </div>
            </section>

            <div className="grid gap-4 py-4 sm:grid-cols-2">
              <div className="panel">
                <div className="panel-header">
                  <span className="section-label">Version</span>
                </div>
                <dl className="divide-y divide-base-300 text-sm">
                  <div className="flex justify-between gap-4 px-4 py-3">
                    <dt className="muted">Installed</dt>
                    <dd>
                      {selectedMod.installed_version ||
                        selectedCandidate?.installed.number ||
                        '—'}
                    </dd>
                  </div>
                  <div className="flex justify-between gap-4 px-4 py-3">
                    <dt className="muted">Latest valid</dt>
                    <dd>
                      {selectedCandidate?.target?.number ??
                        selectedCandidate?.installed.number ??
                        '—'}
                    </dd>
                  </div>
                  <div className="flex justify-between gap-4 px-4 py-3">
                    <dt className="muted">Side</dt>
                    <dd>{selectedMod.side}</dd>
                  </div>
                  <div className="flex justify-between gap-4 px-4 py-3">
                    <dt className="muted">Current placement</dt>
                    <dd>{selectedMod.deployment === 'client' ? 'client-only' : 'server/common'}</dd>
                  </div>
                  <div className="flex justify-between gap-4 px-4 py-3">
                    <dt className="muted">Preferred placement</dt>
                    <dd>
                      {selectedMod.preferred_deployment === 'client'
                        ? 'client-only'
                        : 'server/common'}
                    </dd>
                  </div>
                </dl>
              </div>

              <div className="panel">
                <div className="panel-header">
                  <span className="section-label">Provider</span>
                </div>
                <div className="p-4 text-sm">
                  <div>
                    {selectedMod.provider ?? 'unknown'}
                    {selectedMod.project_id ? ' · ' + selectedMod.project_id : ''}
                  </div>
                  {selectedCandidate?.project_url || selectedMod.project_url ? (
                    <a
                      className="btn btn-sm btn-ghost mt-3"
                      href={selectedCandidate?.project_url || selectedMod.project_url}
                      target="_blank"
                      rel="noreferrer"
                    >
                      Open project page <ExternalLink size={13} />
                    </a>
                  ) : null}
                  {selectedCandidate?.target?.manual_download &&
                  selectedCandidate.target.manual_url ? (
                    <a
                      className="btn btn-sm btn-warning btn-outline mt-2"
                      href={selectedCandidate.target.manual_url}
                      target="_blank"
                      rel="noreferrer"
                    >
                      Manual download <ExternalLink size={13} />
                    </a>
                  ) : null}
                </div>
              </div>
            </div>

            {selectedCandidate?.refresh_error ? (
              <div className="alert alert-warning mb-4 py-2 text-xs">
                Latest metadata refresh failed. The information below is the last successful
                provider metadata and was intentionally preserved.
              </div>
            ) : null}

            {selectedCandidate?.reasons?.length ? (
              <section className="panel mb-4">
                <div className="panel-header">
                  <span className="section-label">Decision</span>
                </div>
                <div className="space-y-2 p-4 text-xs text-base-content/65">
                  {selectedCandidate.reasons.map((reason) => (
                    <p key={reason.code}>{reason.message}</p>
                  ))}
                </div>
              </section>
            ) : null}

            {selectedCandidate?.dependencies?.length ||
            selectedCandidate?.required_by?.length ? (
              <section className="panel mb-4">
                <div className="panel-header">
                  <span className="section-label">Dependencies</span>
                </div>
                <div className="grid gap-4 p-4 sm:grid-cols-2">
                  <div>
                    <div className="text-xs font-medium">Requires</div>
                    <div className="mt-2 space-y-1 text-xs text-base-content/55">
                      {selectedCandidate.dependencies?.filter(
                        (dependency) => dependency.type === 'required',
                      ).length ? (
                        selectedCandidate.dependencies
                          ?.filter((dependency) => dependency.type === 'required')
                          .map((dependency) => (
                            <div
                              key={
                                (dependency.project_id ?? '') +
                                ':' +
                                (dependency.version_id ?? '')
                              }
                            >
                              {dependency.name || dependency.project_id || 'Unknown'} ·{' '}
                              {dependency.action}
                            </div>
                          ))
                      ) : (
                        <span>None reported</span>
                      )}
                    </div>
                  </div>
                  <div>
                    <div className="text-xs font-medium">Required by</div>
                    <div className="mt-2 space-y-1 text-xs text-base-content/55">
                      {selectedCandidate.required_by?.length ? (
                        selectedCandidate.required_by.map((name) => (
                          <div key={name}>{name}</div>
                        ))
                      ) : (
                        <span>None reported</span>
                      )}
                    </div>
                  </div>
                </div>
              </section>
            ) : null}

            {selectedCandidate?.changelogs?.length ? (
              <section className="panel mb-4">
                <div className="panel-header">
                  <div>
                    <div className="section-label">What changed</div>
                    <h3 className="mt-0.5 text-sm font-semibold">
                      {selectedCandidate.changelogs.length} release
                      {selectedCandidate.changelogs.length === 1 ? '' : 's'}
                    </h3>
                  </div>
                </div>
                <div className="divide-y divide-base-300">
                  {selectedCandidate.changelogs.map((entry) => (
                    <div className="p-4" key={entry.id}>
                      <div className="flex flex-wrap items-baseline gap-2">
                        <span className="text-xs font-semibold">
                          {entry.number || entry.name || entry.id}
                        </span>
                        {entry.published_at ? (
                          <span className="text-[0.68rem] text-base-content/35">
                            {formatDate(entry.published_at)}
                          </span>
                        ) : null}
                      </div>
                      <div className="mt-1 whitespace-pre-wrap text-xs leading-5 text-base-content/65">
                        {entry.body || 'No changelog was provided for this release.'}
                      </div>
                    </div>
                  ))}
                </div>
              </section>
            ) : null}

            {selectedCandidate ? (
              <section className="panel mb-4">
                <div className="panel-header">
                  <span className="section-label">Update preference</span>
                </div>
                <div className="flex flex-wrap gap-2 p-4">
                  <button
                    className="btn btn-xs btn-ghost"
                    type="button"
                    disabled={!selectedCandidate.installed.id}
                    onClick={() =>
                      void applyRule(selectedCandidate, {
                        ...selectedRule,
                        pin_version: selectedCandidate.installed.id,
                        ignore_mod: false,
                        review_after: undefined,
                      })
                    }
                  >
                    Pin current
                  </button>
                  {selectedCandidate.target?.id ? (
                    <button
                      className="btn btn-xs btn-ghost"
                      type="button"
                      onClick={() =>
                        void applyRule(selectedCandidate, {
                          ...selectedRule,
                          ignored_versions: Array.from(
                            new Set([
                              ...(selectedRule?.ignored_versions ?? []),
                              selectedCandidate.target?.id ?? '',
                            ]),
                          ).filter(Boolean),
                        })
                      }
                    >
                      Ignore this version
                    </button>
                  ) : null}
                  <button
                    className="btn btn-xs btn-ghost"
                    type="button"
                    onClick={() =>
                      void applyRule(selectedCandidate, {
                        ...selectedRule,
                        ignore_mod: true,
                      })
                    }
                  >
                    Ignore mod
                  </button>
                  <button
                    className="btn btn-xs btn-ghost"
                    type="button"
                    onClick={() =>
                      void applyRule(selectedCandidate, {
                        ...selectedRule,
                        review_after: new Date(
                          Date.now() + 7 * 24 * 60 * 60 * 1000,
                        ).toISOString(),
                      })
                    }
                  >
                    Review in 7 days
                  </button>
                  {selectedRule ? (
                    <button
                      className="btn btn-xs btn-ghost"
                      type="button"
                      onClick={() => void clearRule(selectedCandidate)}
                    >
                      Clear rule
                    </button>
                  ) : null}
                </div>
              </section>
            ) : null}

            <section className="panel">
              <div className="panel-header">
                <span className="section-label">Advanced</span>
              </div>
              <dl className="grid gap-2 p-4 text-xs sm:grid-cols-[120px_minmax(0,1fr)]">
                <dt className="muted">Filename</dt>
                <dd className="mono break-all">{selectedMod.filename}</dd>
                <dt className="muted">Path</dt>
                <dd className="mono break-all">{selectedMod.path}</dd>
                <dt className="muted">Project ID</dt>
                <dd className="mono break-all">{selectedMod.project_id ?? '—'}</dd>
                <dt className="muted">SHA-512</dt>
                <dd className="mono break-all">{selectedMod.sha512 ?? '—'}</dd>
              </dl>
            </section>
          </div>
          <button
            className="modal-backdrop"
            type="button"
            onClick={() => setSelectedMod(null)}
            aria-label="Close details"
          >
            close
          </button>
        </div>
      ) : null}
    </>
  );
}
