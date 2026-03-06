import { Artefact, Job, Project, Promotion, Run } from "../models/types";

const now = Date.now();

export const mockProjects: Project[] = [
  {
    id: "proj-1",
    slug: "vision-core",
    name: "Vision Core",
    description: "Model training and validation pipeline.",
    created_at: new Date(now - 1000 * 60 * 60 * 24 * 30).toISOString(),
    updated_at: new Date(now - 1000 * 60 * 10).toISOString()
  },
  {
    id: "proj-2",
    slug: "edge-ops",
    name: "Edge Ops",
    description: "Device fleet inference pipeline.",
    created_at: new Date(now - 1000 * 60 * 60 * 24 * 60).toISOString(),
    updated_at: new Date(now - 1000 * 60 * 45).toISOString()
  }
];

export const mockRuns: Run[] = [
  {
    id: "run-118",
    project_id: "proj-1",
    run_number: 118,
    name: "threshold-sweep",
    status: "RUNNING",
    created_by: "alex",
    created_at: new Date(now - 1000 * 60 * 70).toISOString(),
    updated_at: new Date(now - 1000 * 60 * 5).toISOString(),
    started_at: new Date(now - 1000 * 60 * 68).toISOString(),
    finished_at: null,
    trigger: "API",
    tags_user: ["nightly", "metrics"],
    tags_system: ["ci"],
    jobs_count: 6,
    slug: "threshold-sweep"
  },
  {
    id: "run-117",
    project_id: "proj-1",
    run_number: 117,
    name: "baseline-eval",
    status: "SUCCEEDED",
    created_by: "ci-bot",
    created_at: new Date(now - 1000 * 60 * 60 * 20).toISOString(),
    updated_at: new Date(now - 1000 * 60 * 60 * 18).toISOString(),
    started_at: new Date(now - 1000 * 60 * 60 * 20).toISOString(),
    finished_at: new Date(now - 1000 * 60 * 60 * 18).toISOString(),
    trigger: "Schedule",
    tags_user: ["nightly"],
    tags_system: ["ci"],
    jobs_count: 4,
    slug: "baseline-eval"
  },
  {
    id: "run-44",
    project_id: "proj-2",
    run_number: 44,
    name: "device-rollout",
    status: "FAILED",
    created_by: "morgan",
    created_at: new Date(now - 1000 * 60 * 60 * 6).toISOString(),
    updated_at: new Date(now - 1000 * 60 * 60 * 5.5).toISOString(),
    started_at: new Date(now - 1000 * 60 * 60 * 6).toISOString(),
    finished_at: new Date(now - 1000 * 60 * 60 * 5.5).toISOString(),
    trigger: "Manual",
    tags_user: ["urgent"],
    tags_system: [],
    jobs_count: 3,
    slug: "device-rollout"
  }
];

export const mockJobs: Job[] = [
  {
    id: "job-1",
    run_id: "run-118",
    name: "preprocess",
    image: "ghcr.io/heldtogether/switchyard-preprocess:latest",
    image_digest: null,
    command: ["/app/run"],
    env: { DATASET: "s3://datasets/v7" },
    status: "SUCCEEDED",
    executor_type: "docker",
    started_at: new Date(now - 1000 * 60 * 65).toISOString(),
    finished_at: new Date(now - 1000 * 60 * 60).toISOString(),
    duration: 1000 * 60 * 5
  },
  {
    id: "job-2",
    run_id: "run-118",
    name: "train",
    image: "ghcr.io/heldtogether/switchyard-train:latest",
    image_digest: null,
    command: ["/app/train"],
    env: { SEED: "42" },
    status: "RUNNING",
    executor_type: "docker",
    started_at: new Date(now - 1000 * 60 * 50).toISOString(),
    finished_at: null,
    duration: 1000 * 60 * 50
  },
  {
    id: "job-3",
    run_id: "run-118",
    name: "validate",
    image: "ghcr.io/heldtogether/switchyard-validate:latest",
    image_digest: null,
    command: ["/app/validate"],
    env: { METRIC: "f1" },
    status: "PENDING",
    executor_type: "docker",
    started_at: null,
    finished_at: null,
    duration: null
  }
];

export const mockArtefacts: Artefact[] = [
  {
    id: "art-1",
    job_id: "job-1",
    path: "outputs/metrics.json",
    object_key: "runs/run-118/outputs/metrics.json",
    size_bytes: 24576,
    content_type: "application/json",
    created_at: new Date(now - 1000 * 60 * 60).toISOString(),
    download_url: "https://example.com/metrics.json"
  },
  {
    id: "art-2",
    job_id: "job-2",
    path: "outputs/curve.png",
    object_key: "runs/run-118/outputs/curve.png",
    size_bytes: 102400,
    content_type: "image/png",
    created_at: new Date(now - 1000 * 60 * 30).toISOString(),
    download_url: "https://placehold.co/600x400/png"
  }
];

export const mockPromotions: Promotion[] = [
  {
    id: "promo-1",
    project_id: "proj-1",
    channel: "dev",
    run_id: "run-117",
    promoted_at: new Date(now - 1000 * 60 * 60 * 2).toISOString(),
    promoted_by: "alex",
    note: "Stable baseline"
  }
];

export function mockLogs(jobId: string) {
  return `Switchyard job ${jobId}\n[12:01:10] Booting container\n[12:01:12] Pulling image\n[12:01:13] Running\n[12:01:15] Processing batch 1/4\n[12:01:20] Processing batch 2/4\n[12:01:25] Processing batch 3/4\n[12:01:30] Processing batch 4/4\n[12:01:32] Done.`;
}
