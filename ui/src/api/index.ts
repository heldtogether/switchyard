import { fetchJson, fetchText, shouldUseMocks } from "./client";
import {
  mockArtefacts,
  mockJobs,
  mockLogs,
  mockProjects,
  mockPromotions,
  mockRuns
} from "./mocks";
import {
  Artefact,
  CreatePromotionRequest,
  CurrentPromotion,
  CreateInviteResponse,
  CancelRunResponse,
  Job,
  Member,
  PromotionHistory,
  Project,
  ResolvedPromotedArtefact,
  Run,
  RunBillingBreakdown,
  Workspace,
  WorkspaceMonthToDateBilling
} from "../models/types";

const runtimeEnv = (window as any).__ENV ?? {};
const DEFAULT_WORKSPACE = runtimeEnv.WORKSPACE_SLUG ?? import.meta.env.VITE_WORKSPACE_SLUG ?? "default";
let activeWorkspaceSlug = DEFAULT_WORKSPACE;
const AGGREGATE_LIMIT = Number(runtimeEnv.AGGREGATE_LIMIT ?? import.meta.env.VITE_AGGREGATE_LIMIT ?? 5);

export function setWorkspaceSlug(slug?: string) {
  activeWorkspaceSlug = slug && slug.trim() ? slug : DEFAULT_WORKSPACE;
}

export function getWorkspaceSlug() {
  return activeWorkspaceSlug;
}

function mapWorkspace(workspace: any): Workspace {
  return {
    id: workspace.id,
    slug: workspace.slug,
    name: workspace.name,
    description: workspace.description,
    created_at: workspace.created_at,
    updated_at: workspace.updated_at
  };
}

function mapRun(run: any, index?: number): Run {
  const metadata = run.metadata ?? {};
  const computedNumber = metadata.run_number ?? (typeof index === "number" ? index + 1 : 0);
  return {
    id: run.id,
    project_id: run.project_id,
    slug: run.slug ?? run.id,
    run_number: computedNumber,
    name: run.name,
    status: run.status,
    created_by: run.created_by ?? "system",
    created_at: run.created_at,
    updated_at: run.updated_at,
    started_at: run.started_at ?? null,
    finished_at: run.finished_at ?? null,
    trigger: metadata.trigger ?? "API",
    tags_user: metadata.tags_user ?? [],
    tags_system: metadata.tags_system ?? [],
    jobs_count: metadata.jobs_count ?? 0,
    rerun_of_run_id: metadata.rerun_of_run_id,
    rerun_of_run_slug: metadata.rerun_of_run_slug,
    rerun_mode: metadata.rerun_mode
  } as Run;
}

function mapProject(project: any): Project {
  return {
    id: project.id,
    slug: project.slug,
    name: project.name,
    description: project.description,
    created_at: project.created_at,
    updated_at: project.updated_at
  };
}

function mapJob(job: any, runId?: string): Job {
  return {
    id: job.id,
    run_id: job.run_id ?? runId ?? "",
    name: job.name || job.image,
    image: job.image,
    image_digest: job.image_digest,
    command: job.command ?? [],
    env: job.env ?? {},
    status: job.status,
    executor_type: job.executor,
    started_at: job.started_at ?? null,
    finished_at: job.finished_at ?? null,
    duration:
      job.started_at && job.finished_at
        ? new Date(job.finished_at).getTime() - new Date(job.started_at).getTime()
        : undefined
  };
}

function mapPromotionEvent(event: any) {
  return {
    id: event.id,
    workspace_id: event.workspace_id,
    project_id: event.project_id,
    channel: event.channel,
    run_id: event.run_id,
    promoted_at: event.promoted_at ?? event.created_at,
    promoted_by: event.promoted_by,
    promoted_by_principal_id: event.promoted_by_principal_id,
    note: event.note,
    artefacts: Array.isArray(event.artefacts)
      ? event.artefacts.map((art: any) => ({
          logical_key: art.logical_key,
          job_id: art.job_id,
          path: art.path,
          object_key: art.object_key,
          size_bytes: art.size_bytes,
          content_type: art.content_type
        }))
      : []
  };
}

function mapCurrentPromotion(promo: any): CurrentPromotion {
  return {
    project_id: promo.project_id,
    channel: promo.channel,
    event: mapPromotionEvent(promo.event ?? promo)
  };
}

export async function listWorkspaces(): Promise<Workspace[]> {
  try {
    const res = await fetchJson<{ workspaces: any[] }>(`/v1/workspaces?limit=100&offset=0`);
    const workspaces = Array.isArray(res.workspaces) ? res.workspaces : [];
    return workspaces.map(mapWorkspace);
  } catch (error) {
    if (shouldUseMocks(error)) {
      return [
        {
          id: "default",
          slug: DEFAULT_WORKSPACE,
          name: "Default Workspace",
          description: "",
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString()
        }
      ];
    }
    throw error;
  }
}

export async function listProjects(workspaceSlug = activeWorkspaceSlug): Promise<Project[]> {
  try {
    const res = await fetchJson<{ projects: any[] }>(`/v1/workspaces/${workspaceSlug}/projects?limit=50&offset=0`);
    return res.projects.map(mapProject);
  } catch (error) {
    if (shouldUseMocks(error)) return mockProjects;
    throw error;
  }
}

export async function getProject(slug: string, workspaceSlug = activeWorkspaceSlug): Promise<Project> {
  try {
    const res = await fetchJson<any>(`/v1/workspaces/${workspaceSlug}/projects/${slug}`);
    return mapProject(res);
  } catch (error) {
    if (shouldUseMocks(error)) {
      const project = mockProjects.find((p) => p.slug === slug);
      if (!project) throw error;
      return project;
    }
    throw error;
  }
}

export async function listRuns(projectSlug: string, workspaceSlug = activeWorkspaceSlug): Promise<Run[]> {
  try {
    const res = await fetchJson<{ runs: any[] }>(
      `/v1/workspaces/${workspaceSlug}/projects/${projectSlug}/runs?limit=50&offset=0`
    );
    return res.runs.map((run, index) => mapRun(run, index));
  } catch (error) {
    if (shouldUseMocks(error)) {
      const project = mockProjects.find((p) => p.slug === projectSlug);
      return mockRuns.filter((run) => run.project_id === (project?.id ?? projectSlug));
    }
    throw error;
  }
}

export async function getRun(
  projectSlug: string,
  runSlug: string,
  workspaceSlug = activeWorkspaceSlug
): Promise<Run> {
  try {
    const res = await fetchJson<any>(
      `/v1/workspaces/${workspaceSlug}/projects/${projectSlug}/runs/${runSlug}`
    );
    return mapRun(res);
  } catch (error) {
    if (shouldUseMocks(error)) {
      const run = mockRuns.find((r) => r.slug === runSlug || r.id === runSlug);
      if (!run) throw error;
      return run;
    }
    throw error;
  }
}

export async function getWorkspaceMonthToDateBilling(
  workspaceSlug = activeWorkspaceSlug
): Promise<WorkspaceMonthToDateBilling> {
  try {
    return await fetchJson<WorkspaceMonthToDateBilling>(
      `/v1/workspaces/${workspaceSlug}/billing/month-to-date`
    );
  } catch (error) {
    if (shouldUseMocks(error)) {
      return {
        workspace_id: workspaceSlug,
        month_key: new Date().toISOString().slice(0, 7),
        cpu_seconds: 0,
        memory_gb_seconds: 0,
        gpu_seconds: 0,
        estimated_total_minor: 0,
        estimated_total_minor_exact: 0,
        currency: "USD"
      };
    }
    throw error;
  }
}

export async function getRunBillingBreakdown(
  projectSlug: string,
  runSlug: string,
  workspaceSlug = activeWorkspaceSlug
): Promise<RunBillingBreakdown> {
  try {
    return await fetchJson<RunBillingBreakdown>(
      `/v1/workspaces/${workspaceSlug}/projects/${projectSlug}/runs/${runSlug}/billing`
    );
  } catch (error) {
    if (shouldUseMocks(error)) {
      const run = mockRuns.find((r) => r.slug === runSlug || r.id === runSlug);
      const jobs = mockJobs.filter((j) => j.run_id === (run?.id ?? runSlug));
      return {
        workspace_id: workspaceSlug,
        project_id: run?.project_id ?? "",
        run_id: run?.id ?? runSlug,
        cpu_seconds: 0,
        memory_gb_seconds: 0,
        gpu_seconds: 0,
        estimated_total_minor: 0,
        estimated_total_minor_exact: 0,
        currency: "USD",
        items: jobs.map((job) => ({
          job_id: job.id,
          cpu_seconds: 0,
          memory_gb_seconds: 0,
          gpu_seconds: 0,
          estimated_cpu_minor: 0,
          estimated_memory_minor: 0,
          estimated_gpu_minor: 0,
          estimated_total_minor: 0,
          estimated_cpu_minor_exact: 0,
          estimated_memory_minor_exact: 0,
          estimated_gpu_minor_exact: 0,
          estimated_total_minor_exact: 0,
          pricing_version: "mock",
          currency: "USD",
          created_at: new Date().toISOString()
        }))
      };
    }
    throw error;
  }
}

export async function listJobs(
  projectSlug: string,
  runSlug: string,
  workspaceSlug = activeWorkspaceSlug
): Promise<Job[]> {
  try {
    const res = await fetchJson<{ jobs: any[] }>(
      `/v1/workspaces/${workspaceSlug}/projects/${projectSlug}/runs/${runSlug}/jobs?limit=100&offset=0`
    );
    return res.jobs.map((job) => mapJob(job, runSlug));
  } catch (error) {
    if (shouldUseMocks(error)) {
      const run = mockRuns.find((r) => r.slug === runSlug || r.id === runSlug);
      return mockJobs.filter((job) => job.run_id === (run?.id ?? runSlug));
    }
    throw error;
  }
}

export async function getJob(
  projectSlug: string,
  runSlug: string,
  jobId: string,
  workspaceSlug = activeWorkspaceSlug
): Promise<Job> {
  try {
    const res = await fetchJson<any>(
      `/v1/workspaces/${workspaceSlug}/projects/${projectSlug}/runs/${runSlug}/jobs/${jobId}`
    );
    return mapJob(res, runSlug);
  } catch (error) {
    if (shouldUseMocks(error)) {
      const job = mockJobs.find((j) => j.id === jobId);
      if (!job) throw error;
      return job;
    }
    throw error;
  }
}

export async function getJobLogs(
  projectSlug: string,
  runSlug: string,
  jobId: string,
  workspaceSlug = activeWorkspaceSlug
): Promise<string> {
  try {
    return await fetchText(
      `/v1/workspaces/${workspaceSlug}/projects/${projectSlug}/runs/${runSlug}/jobs/${jobId}/logs`
    );
  } catch (error) {
    if (shouldUseMocks(error)) return mockLogs(jobId);
    throw error;
  }
}

export async function listArtefacts(
  projectSlug: string,
  runSlug: string,
  jobId: string,
  workspaceSlug = activeWorkspaceSlug
): Promise<Artefact[]> {
  try {
    const res = await fetchJson<{ artefacts: any[] }>(
      `/v1/workspaces/${workspaceSlug}/projects/${projectSlug}/runs/${runSlug}/jobs/${jobId}/artefacts`
    );
    return res.artefacts.map((art) => ({
      id: `${jobId}-${art.path}`,
      job_id: jobId,
      path: art.path,
      object_key: art.path,
      size_bytes: art.size_bytes,
      content_type: art.content_type,
      created_at: new Date().toISOString(),
      download_url: art.download_url
    }));
  } catch (error) {
    if (shouldUseMocks(error)) return mockArtefacts.filter((a) => a.job_id === jobId);
    throw error;
  }
}

export async function listCurrentPromotions(
  projectSlug: string,
  workspaceSlug = activeWorkspaceSlug
): Promise<CurrentPromotion[]> {
  try {
    const res = await fetchJson<{ promotions: any[] }>(
      `/v1/workspaces/${workspaceSlug}/projects/${projectSlug}/promotions`
    );
    return (res.promotions ?? []).map(mapCurrentPromotion);
  } catch (error) {
    if (shouldUseMocks(error)) {
      const project = mockProjects.find((p) => p.slug === projectSlug);
      return mockPromotions.filter((promo) => promo.project_id === (project?.id ?? projectSlug));
    }
    throw error;
  }
}

export async function createPromotion(
  projectSlug: string,
  promotion: CreatePromotionRequest,
  workspaceSlug = activeWorkspaceSlug
) {
  try {
    const res = await fetchJson<any>(
      `/v1/workspaces/${workspaceSlug}/projects/${projectSlug}/promotions`,
      {
        method: "POST",
        body: JSON.stringify(promotion)
      }
    );
    return mapPromotionEvent(res);
  } catch (error) {
    if (shouldUseMocks(error)) {
      return {
        id: `promo-${Date.now()}`,
        workspace_id: workspaceSlug,
        project_id: projectSlug,
        channel: promotion.channel,
        run_id: promotion.run_id ?? "",
        promoted_at: new Date().toISOString(),
        promoted_by: "mock-user",
        note: promotion.note,
        artefacts: (promotion.artefacts ?? []).map((art) => ({
          logical_key: art.logical_key,
          job_id: art.job_id,
          path: art.path,
          object_key: art.path,
          size_bytes: 0,
          content_type: ""
        }))
      };
    }
    throw error;
  }
}

export async function listPromotionHistory(
  projectSlug: string,
  channel?: string,
  workspaceSlug = activeWorkspaceSlug
): Promise<PromotionHistory> {
  try {
    const params = new URLSearchParams();
    if (channel) params.set("channel", channel);
    const suffix = params.toString() ? `?${params.toString()}` : "";
    const res = await fetchJson<PromotionHistory>(
      `/v1/workspaces/${workspaceSlug}/projects/${projectSlug}/promotions/history${suffix}`
    );
    return {
      events: (res.events ?? []).map(mapPromotionEvent),
      total: res.total ?? 0,
      limit: res.limit ?? 50,
      offset: res.offset ?? 0
    };
  } catch (error) {
    if (shouldUseMocks(error)) {
      const current = await listCurrentPromotions(projectSlug, workspaceSlug);
      const events = current.map((promo) => promo.event);
      return { events, total: events.length, limit: 50, offset: 0 };
    }
    throw error;
  }
}

export async function resolvePromotedArtefact(
  projectSlug: string,
  channel: string,
  logicalKey: string,
  workspaceSlug = activeWorkspaceSlug
): Promise<ResolvedPromotedArtefact> {
  try {
    return await fetchJson<ResolvedPromotedArtefact>(
      `/v1/workspaces/${workspaceSlug}/projects/${projectSlug}/promotions/${channel}/artefacts/${encodeURIComponent(logicalKey)}`
    );
  } catch (error) {
    if (shouldUseMocks(error)) {
      return {
        channel: channel as any,
        logical_key: logicalKey,
        promotion_event_id: "promo-mock",
        run_id: "run-117",
        job_id: "job-1",
        path: "outputs/mock.bin",
        object_key: "outputs/mock.bin",
        size_bytes: 0,
        content_type: "application/octet-stream",
        promoted_at: new Date().toISOString(),
        promoted_by: "mock-user",
        download_url: "https://example.com/mock.bin",
        download_url_expires_at: new Date(Date.now() + 15 * 60_000).toISOString()
      };
    }
    throw error;
  }
}

export async function listAllRuns(workspaceSlug = activeWorkspaceSlug) {
  const projects = await listProjects(workspaceSlug);
  const limited = projects.slice(0, AGGREGATE_LIMIT);
  const entries = await Promise.all(
    limited.map(async (project) => ({
      project,
      runs: await listRuns(project.slug, workspaceSlug)
    }))
  );
  return entries.flatMap(({ project, runs }) =>
    runs.map((run) => ({ ...run, project_slug: project.slug, project_name: project.name }))
  );
}

export async function listAllJobs(workspaceSlug = activeWorkspaceSlug) {
  const projects = await listProjects(workspaceSlug);
  const limited = projects.slice(0, AGGREGATE_LIMIT);
  const entries = await Promise.all(
    limited.map(async (project) => {
      const runs = await listRuns(project.slug, workspaceSlug);
      const runEntries = await Promise.all(
        runs.slice(0, AGGREGATE_LIMIT).map(async (run) => ({
          run,
          jobs: await listJobs(project.slug, run.slug, workspaceSlug)
        }))
      );
      return runEntries.flatMap(({ run, jobs }) =>
        jobs.map((job) => ({
          ...job,
          project_slug: project.slug,
          project_name: project.name,
          run_slug: run.slug,
          run_number: run.run_number
        }))
      );
    })
  );
  return entries.flat();
}

export async function listAllArtefacts(workspaceSlug = activeWorkspaceSlug) {
  const projects = await listProjects(workspaceSlug);
  const limited = projects.slice(0, AGGREGATE_LIMIT);
  const entries = await Promise.all(
    limited.map(async (project) => {
      const runs = await listRuns(project.slug, workspaceSlug);
      const jobEntries = await Promise.all(
        runs.slice(0, AGGREGATE_LIMIT).map(async (run) => {
          const jobs = await listJobs(project.slug, run.slug, workspaceSlug);
          const artefactEntries = await Promise.all(
            jobs.slice(0, AGGREGATE_LIMIT).map(async (job) => ({
              job,
              run,
              artefacts: await listArtefacts(project.slug, run.slug, job.id, workspaceSlug)
            }))
          );
          return artefactEntries.flatMap(({ job, run, artefacts }) =>
            artefacts.map((art) => ({
              ...art,
              project_slug: project.slug,
              project_name: project.name,
              run_slug: run.slug,
              run_number: run.run_number,
              job_name: job.name,
              job_image: job.image
            }))
          );
        })
      );
      return jobEntries.flat();
    })
  );
  return entries.flat();
}

export type RegistrySecret = {
  id: string;
  created_at: string;
  created_by: string;
  host: string;
  username: string;
  active: boolean;
  deactivated_at?: string | null;
  deactivated_by?: string | null;
  rotated_from_secret_id?: string | null;
};

export async function listRegistrySecrets(): Promise<RegistrySecret[]> {
  const data = await fetchJson<{ registry_secrets: RegistrySecret[] }>(`/v1/workspaces/${activeWorkspaceSlug}/registry-secrets`);
  return data.registry_secrets ?? [];
}

export async function createRegistrySecret(payload: { host: string; username: string; password: string }): Promise<RegistrySecret> {
  return fetchJson<RegistrySecret>(`/v1/workspaces/${activeWorkspaceSlug}/registry-secrets`, {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

export async function deleteRegistrySecret(secretId: string): Promise<{ message: string }> {
  return fetchJson<{ message: string }>(`/v1/workspaces/${activeWorkspaceSlug}/registry-secrets/${secretId}`, {
    method: "DELETE"
  });
}

export async function rotateRegistrySecret(secretId: string, payload: { password: string }): Promise<RegistrySecret> {
  return fetchJson<RegistrySecret>(`/v1/workspaces/${activeWorkspaceSlug}/registry-secrets/${secretId}/rotate`, {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

export type ServiceAccountKey = {
  id: string;
  name?: string | null;
  token_prefix: string;
  expires_at: string;
  last_used_at?: string | null;
  revoked_at?: string | null;
  created_at: string;
  created_by: string;
};

export type ServiceAccount = {
  id: string;
  workspace_id: string;
  principal_id: string;
  subject: string;
  name: string;
  description?: string | null;
  disabled_at?: string | null;
  created_at: string;
  updated_at: string;
  created_by: string;
  project_slugs?: string[];
  keys?: ServiceAccountKey[];
};

export type CreateServiceAccountPayload = {
  name: string;
  description?: string;
  expires_at: string;
  project_slugs?: string[];
};

export type CreateServiceAccountResponse = {
  service_account: ServiceAccount;
  key: string;
};

export type CreateServiceAccountKeyResponse = {
  key_id: string;
  key: string;
  token_prefix: string;
  expires_at: string;
};

export async function listServiceAccounts(): Promise<ServiceAccount[]> {
  const data = await fetchJson<{ service_accounts: ServiceAccount[] }>(
    `/v1/workspaces/${activeWorkspaceSlug}/service-accounts`
  );
  return data.service_accounts ?? [];
}

export async function createServiceAccount(payload: CreateServiceAccountPayload): Promise<CreateServiceAccountResponse> {
  return fetchJson<CreateServiceAccountResponse>(`/v1/workspaces/${activeWorkspaceSlug}/service-accounts`, {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

export async function createServiceAccountKey(
  serviceAccountId: string,
  payload: { name?: string; expires_at: string }
): Promise<CreateServiceAccountKeyResponse> {
  return fetchJson<CreateServiceAccountKeyResponse>(
    `/v1/workspaces/${activeWorkspaceSlug}/service-accounts/${serviceAccountId}/keys`,
    {
      method: "POST",
      body: JSON.stringify(payload)
    }
  );
}

export async function revokeServiceAccountKey(serviceAccountId: string, keyId: string): Promise<void> {
  await fetchJson<void>(`/v1/workspaces/${activeWorkspaceSlug}/service-accounts/${serviceAccountId}/keys/${keyId}`, {
    method: "DELETE"
  });
}

export async function disableServiceAccount(serviceAccountId: string): Promise<void> {
  await fetchJson<void>(`/v1/workspaces/${activeWorkspaceSlug}/service-accounts/${serviceAccountId}`, {
    method: "DELETE"
  });
}

export async function listWorkspaceMembers(): Promise<Member[]> {
  const data = await fetchJson<{ members: Member[] }>(`/v1/workspaces/${activeWorkspaceSlug}/members`);
  return data.members ?? [];
}

export async function listProjectMembers(projectSlug: string): Promise<Member[]> {
  const data = await fetchJson<{ members: Member[] }>(
    `/v1/workspaces/${activeWorkspaceSlug}/projects/${projectSlug}/members`
  );
  return data.members ?? [];
}

export async function createWorkspaceInvite(email: string): Promise<CreateInviteResponse> {
  return fetchJson<CreateInviteResponse>(`/v1/workspaces/${activeWorkspaceSlug}/invites`, {
    method: "POST",
    body: JSON.stringify({ email, role: "member" })
  });
}

export async function createProjectInvite(projectSlug: string, email: string): Promise<CreateInviteResponse> {
  return fetchJson<CreateInviteResponse>(
    `/v1/workspaces/${activeWorkspaceSlug}/projects/${projectSlug}/invites`,
    {
      method: "POST",
      body: JSON.stringify({ email, role: "member" })
    }
  );
}

export async function acceptWorkspaceInvite(token: string): Promise<{ message: string }> {
  return fetchJson<{ message: string }>(`/v1/workspace-invites/accept`, {
    method: "POST",
    body: JSON.stringify({ token })
  });
}

export async function acceptProjectInvite(token: string): Promise<{ message: string }> {
  return fetchJson<{ message: string }>(`/v1/project-invites/accept`, {
    method: "POST",
    body: JSON.stringify({ token })
  });
}

export async function getAllocationCapacity(): Promise<{ max_gpu_per_node: number }> {
  try {
    return await fetchJson<{ max_gpu_per_node: number }>(`/v1/allocations/capacity`);
  } catch (error) {
    if (shouldUseMocks(error)) return { max_gpu_per_node: 2 };
    throw error;
  }
}

export async function createRun(
  projectSlug: string,
  payload: { slug: string; name: string; description?: string; metadata?: Record<string, any> },
  workspaceSlug = activeWorkspaceSlug
) {
  return await fetchJson<any>(
    `/v1/workspaces/${workspaceSlug}/projects/${projectSlug}/runs`,
    {
      method: "POST",
      body: JSON.stringify(payload)
    }
  );
}

export async function createProject(payload: { slug: string; name: string; description?: string }, workspaceSlug = activeWorkspaceSlug) {
  return await fetchJson<any>(
    `/v1/workspaces/${workspaceSlug}/projects`,
    {
      method: "POST",
      body: JSON.stringify(payload)
    }
  );
}

export async function createWorkspace(payload: { slug: string; name: string; description?: string }) {
  return await fetchJson<any>("/v1/workspaces", {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

export async function createJob(
  projectSlug: string,
  runSlug: string,
  payload: any,
  workspaceSlug = activeWorkspaceSlug
) {
  return await fetchJson<any>(
    `/v1/workspaces/${workspaceSlug}/projects/${projectSlug}/runs/${runSlug}/jobs`,
    {
      method: "POST",
      body: JSON.stringify(payload)
    }
  );
}

export async function rerunRun(
  projectSlug: string,
  runSlug: string,
  payload: { mode: "all" | "failed_only" },
  workspaceSlug = activeWorkspaceSlug
) {
  return fetchJson<{ run: any; jobs_created: number; source_run_id: string; mode: "all" | "failed_only" }>(
    `/v1/workspaces/${workspaceSlug}/projects/${projectSlug}/runs/${runSlug}/rerun`,
    {
      method: "POST",
      body: JSON.stringify(payload)
    }
  );
}

export async function cancelJob(
  projectSlug: string,
  runSlug: string,
  jobId: string,
  workspaceSlug = activeWorkspaceSlug
): Promise<Job> {
  const res = await fetchJson<any>(
    `/v1/workspaces/${workspaceSlug}/projects/${projectSlug}/runs/${runSlug}/jobs/${jobId}/cancel`,
    {
      method: "POST"
    }
  );
  return mapJob(res, runSlug);
}

export async function cancelRun(
  projectSlug: string,
  runSlug: string,
  workspaceSlug = activeWorkspaceSlug
): Promise<CancelRunResponse> {
  return fetchJson<CancelRunResponse>(
    `/v1/workspaces/${workspaceSlug}/projects/${projectSlug}/runs/${runSlug}/cancel`,
    {
      method: "POST"
    }
  );
}
