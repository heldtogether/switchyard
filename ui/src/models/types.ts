export type Status = "SUCCEEDED" | "FAILED" | "RUNNING" | "PENDING" | "CANCELLED" | "TIMEOUT" | "PARTIAL";

export interface Project {
  id: string;
  name: string;
  description?: string | null;
  created_at: string;
  updated_at: string;
  slug: string;
}

export interface Workspace {
  id: string;
  slug: string;
  name: string;
  description?: string | null;
  created_at: string;
  updated_at: string;
}

export interface Member {
  subject: string;
  email?: string | null;
  display_name?: string | null;
  role: "owner" | "member";
  added_at: string;
}

export interface CreateInviteResponse {
  invite_id: string;
  invite_url: string;
  invite_token: string;
  expires_at: string;
}

export interface Run {
  id: string;
  project_id: string;
  run_number: number;
  name?: string | null;
  status: Status;
  created_by: string;
  created_at: string;
  updated_at: string;
  started_at?: string | null;
  finished_at?: string | null;
  trigger: "API" | "Schedule" | "Manual";
  tags_user: string[];
  tags_system: string[];
  jobs_count: number;
  slug: string;
  rerun_of_run_id?: string;
  rerun_of_run_slug?: string;
  rerun_mode?: "all" | "failed_only";
}

export interface Job {
  id: string;
  run_id: string;
  name: string;
  image: string;
  image_digest?: string | null;
  command: string[];
  env: Record<string, string>;
  status: Status;
  executor_type: string;
  started_at?: string | null;
  finished_at?: string | null;
  duration?: number;
}

export interface Artefact {
  id: string;
  job_id: string;
  path: string;
  object_key: string;
  size_bytes: number;
  content_type: string;
  created_at: string;
  download_url?: string;
}

export interface WorkspaceMonthToDateBilling {
  workspace_id: string;
  month_key: string;
  cpu_seconds: number;
  memory_gb_seconds: number;
  estimated_total_minor: number;
  estimated_total_minor_exact: number;
  currency: string;
}

export interface RunBillingLineItem {
  job_id: string;
  cpu_seconds: number;
  memory_gb_seconds: number;
  estimated_cpu_minor: number;
  estimated_memory_minor: number;
  estimated_total_minor: number;
  estimated_cpu_minor_exact: number;
  estimated_memory_minor_exact: number;
  estimated_total_minor_exact: number;
  pricing_version: string;
  currency: string;
  created_at: string;
}

export interface RunBillingBreakdown {
  workspace_id: string;
  project_id: string;
  run_id: string;
  cpu_seconds: number;
  memory_gb_seconds: number;
  estimated_total_minor: number;
  estimated_total_minor_exact: number;
  currency: string;
  items: RunBillingLineItem[];
}

export type PromotionChannel = "dev" | "staging" | "prod" | "validated";

export interface Promotion {
  id: string;
  project_id: string;
  channel: PromotionChannel;
  run_id: string;
  promoted_at: string;
  promoted_by: string;
  note?: string;
  artefact_keys?: string[];
}
