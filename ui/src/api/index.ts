import { fetchJson, shouldUseMocks } from "./client";
import {
  mockArtefacts,
  mockJobs,
  mockLogs,
  mockProjects,
  mockPromotions,
  mockRuns
} from "./mocks";
import { Artefact, Job, Project, Promotion, Run } from "../models/types";

const WORKSPACE = import.meta.env.VITE_WORKSPACE_SLUG ?? "default";

function mapRun(run: any, index: number): Run {
  const metadata = run.metadata ?? {};
  return {
    id: run.id,
    project_id: run.project_id,
    slug: run.slug ?? run.id,
    run_number: metadata.run_number ?? index + 1,
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
    jobs_count: metadata.jobs_count ?? 0
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
    name: job.name ?? job.image,
    image: job.image,
    image_digest: job.image_digest,
    command: job.command ?? [],
    env: job.env ?? {},
    status: job.status,
    executor_type: job.executor,
    started_at: job.started_at ?? null,
    finished_at: job.finished_at ?? null,
    duration: job.started_at && job.finished_at ? new Date(job.finished_at).getTime() - new Date(job.started_at).getTime() : undefined
  };
}

export async function listProjects(): Promise<Project[]> {
  try {
    const res = await fetchJson<{ projects: any[] }>(`/v1/workspaces/${WORKSPACE}/projects?limit=50&offset=0`);
    return res.projects.map(mapProject);
  } catch (error) {
    if (shouldUseMocks(error)) return mockProjects;
    throw error;
  }
}

export async function getProject(slug: string): Promise<Project> {
  try {
    const res = await fetchJson<any>(`/v1/workspaces/${WORKSPACE}/projects/${slug}`);
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

export async function listRuns(projectSlug: string): Promise<Run[]> {
  try {
    const res = await fetchJson<{ runs: any[] }>(
      `/v1/workspaces/${WORKSPACE}/projects/${projectSlug}/runs?limit=50&offset=0`
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

export async function getRun(projectSlug: string, runSlug: string): Promise<Run> {
  try {
    const res = await fetchJson<any>(
      `/v1/workspaces/${WORKSPACE}/projects/${projectSlug}/runs/${runSlug}`
    );
    return mapRun(res, 0);
  } catch (error) {
    if (shouldUseMocks(error)) {
      const run = mockRuns.find((r) => r.slug === runSlug || r.id === runSlug);
      if (!run) throw error;
      return run;
    }
    throw error;
  }
}

export async function listJobs(projectSlug: string, runSlug: string): Promise<Job[]> {
  try {
    const res = await fetchJson<{ jobs: any[] }>(
      `/v1/workspaces/${WORKSPACE}/projects/${projectSlug}/runs/${runSlug}/jobs?limit=100&offset=0`
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

export async function getJob(projectSlug: string, runSlug: string, jobId: string): Promise<Job> {
  try {
    const res = await fetchJson<any>(
      `/v1/workspaces/${WORKSPACE}/projects/${projectSlug}/runs/${runSlug}/jobs/${jobId}`
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

export async function getJobLogs(projectSlug: string, runSlug: string, jobId: string): Promise<string> {
  try {
    const res = await fetch(`${import.meta.env.VITE_API_BASE_URL ?? "http://localhost:8080"}/v1/workspaces/${WORKSPACE}/projects/${projectSlug}/runs/${runSlug}/jobs/${jobId}/logs`, {
      headers: {
        ...(import.meta.env.VITE_API_KEY ? { "X-API-Key": import.meta.env.VITE_API_KEY } : {})
      }
    });
    if (!res.ok) throw new Error(res.statusText);
    return res.text();
  } catch (error) {
    if (shouldUseMocks(error)) return mockLogs(jobId);
    throw error;
  }
}

export async function listArtefacts(projectSlug: string, runSlug: string, jobId: string): Promise<Artefact[]> {
  try {
    const res = await fetchJson<{ artefacts: any[] }>(
      `/v1/workspaces/${WORKSPACE}/projects/${projectSlug}/runs/${runSlug}/jobs/${jobId}/artefacts`
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

const PROMO_KEY = "switchyard.promotions";

export function listPromotions(projectId: string): Promotion[] {
  const raw = localStorage.getItem(PROMO_KEY);
  const list = raw ? (JSON.parse(raw) as Promotion[]) : mockPromotions;
  return list.filter((promo) => promo.project_id === projectId);
}

export function savePromotion(promo: Promotion) {
  const raw = localStorage.getItem(PROMO_KEY);
  const list = raw ? (JSON.parse(raw) as Promotion[]) : mockPromotions;
  const next = [promo, ...list.filter((p) => !(p.project_id === promo.project_id && p.channel === promo.channel))];
  localStorage.setItem(PROMO_KEY, JSON.stringify(next));
}
