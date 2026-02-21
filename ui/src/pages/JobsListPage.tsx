import React, { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { listProjects, listAllJobs } from "../api";
import { PageHeader } from "../components/PageHeader";
import { DataTable, DataTableBody, DataTableCell, DataTableHeader, DataTableHeaderCell } from "../components/DataTable";
import { StatusPill } from "../components/StatusPill";
import { EmptyState } from "../components/EmptyState";
import { ErrorBanner } from "../components/ErrorBanner";
import { RelativeTime } from "../components/RelativeTime";
import { formatDurationMs } from "../utils/format";
import { useNavigate } from "react-router-dom";

export function JobsListPage() {
  const navigate = useNavigate();
  const [statusFilter, setStatusFilter] = useState("all");
  const [projectFilter, setProjectFilter] = useState("all");
  const [search, setSearch] = useState("");

  const projectsQuery = useQuery({
    queryKey: ["projects"],
    queryFn: listProjects
  });

  const jobsQuery = useQuery({
    queryKey: ["jobs", "all"],
    queryFn: listAllJobs,
    enabled: !!projectsQuery.data
  });

  const filteredJobs = useMemo(() => {
    return (jobsQuery.data ?? [])
      .filter((job) => (statusFilter === "all" ? true : job.status === statusFilter))
      .filter((job) => (projectFilter === "all" ? true : job.project_slug === projectFilter))
      .filter((job) => job.name.toLowerCase().includes(search.toLowerCase()));
  }, [jobsQuery.data, statusFilter, projectFilter, search]);

  return (
    <div className="space-y-6">
      <PageHeader title="Jobs" subtitle="Track every job across your workspace." />

      {projectsQuery.error && (
        <ErrorBanner message={(projectsQuery.error as Error).message} onRetry={() => projectsQuery.refetch()} />
      )}
      {jobsQuery.error && (
        <ErrorBanner message={(jobsQuery.error as Error).message} onRetry={() => jobsQuery.refetch()} />
      )}

      <div className="card p-4">
        <div className="grid gap-3 md:grid-cols-3">
          <select
            className="rounded-lg border border-ink-200 px-3 py-2 text-sm"
            value={statusFilter}
            onChange={(event) => setStatusFilter(event.target.value)}
          >
            <option value="all">All statuses</option>
            <option value="SUCCEEDED">Succeeded</option>
            <option value="FAILED">Failed</option>
            <option value="RUNNING">Running</option>
            <option value="PENDING">Pending</option>
          </select>
          <select
            className="rounded-lg border border-ink-200 px-3 py-2 text-sm"
            value={projectFilter}
            onChange={(event) => setProjectFilter(event.target.value)}
          >
            <option value="all">All projects</option>
            {projectsQuery.data?.map((project) => (
              <option key={project.id} value={project.slug}>
                {project.name}
              </option>
            ))}
          </select>
          <input
            className="rounded-lg border border-ink-200 px-3 py-2 text-sm"
            placeholder="Search jobs"
            value={search}
            onChange={(event) => setSearch(event.target.value)}
          />
        </div>
      </div>

      {filteredJobs.length === 0 ? (
        <EmptyState title="No jobs found" description="Try adjusting your filters." />
      ) : (
        <DataTable>
          <DataTableHeader>
            <DataTableHeaderCell>Job</DataTableHeaderCell>
            <DataTableHeaderCell>Status</DataTableHeaderCell>
            <DataTableHeaderCell>Run</DataTableHeaderCell>
            <DataTableHeaderCell>Duration</DataTableHeaderCell>
            <DataTableHeaderCell>Executor</DataTableHeaderCell>
          </DataTableHeader>
          <DataTableBody>
            {filteredJobs.map((job) => (
              <tr
                key={job.id}
                className="cursor-pointer hover:bg-ink-50"
                onClick={() => navigate(`/${job.project_slug}/${job.run_slug}/${job.id}`)}
              >
                <DataTableCell>
                  <div className="font-semibold text-ink-900">{job.name}</div>
                  {job.image && job.image !== job.name && (
                    <div className="text-xs text-ink-500">{job.image}</div>
                  )}
                </DataTableCell>
                <DataTableCell>
                  <StatusPill status={job.status} />
                </DataTableCell>
                <DataTableCell>
                  <div className="text-sm font-semibold text-ink-900">{job.run_slug}</div>
                </DataTableCell>
                <DataTableCell>{formatDurationMs(job.duration)}</DataTableCell>
                <DataTableCell>{job.executor_type}</DataTableCell>
              </tr>
            ))}
          </DataTableBody>
        </DataTable>
      )}
    </div>
  );
}
