import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { listAllRuns, listProjects } from "../api";
import { PageHeader } from "../components/PageHeader";
import { DataTable, DataTableBody, DataTableCell, DataTableHeader, DataTableHeaderCell } from "../components/DataTable";
import { StatusPill } from "../components/StatusPill";
import { EmptyState } from "../components/EmptyState";
import { ErrorBanner } from "../components/ErrorBanner";
import { RelativeTime } from "../components/RelativeTime";
import { useNavigate } from "react-router-dom";

export function RunsListPage() {
  const navigate = useNavigate();
  const [statusFilter, setStatusFilter] = useState("all");
  const [projectFilter, setProjectFilter] = useState("all");
  const [search, setSearch] = useState("");

  const projectsQuery = useQuery({
    queryKey: ["projects"],
    queryFn: listProjects
  });

  const runsQuery = useQuery({
    queryKey: ["runs", "all"],
    queryFn: listAllRuns,
    enabled: !!projectsQuery.data
  });

  const filteredRuns = useMemo(() => {
    return (runsQuery.data ?? [])
      .filter((run) => (statusFilter === "all" ? true : run.status === statusFilter))
      .filter((run) => (projectFilter === "all" ? true : run.project_slug === projectFilter))
      .filter((run) =>
        run.name?.toLowerCase().includes(search.toLowerCase()) ||
        run.slug?.toLowerCase().includes(search.toLowerCase())
      );
  }, [runsQuery.data, statusFilter, projectFilter, search]);

  return (
    <div className="space-y-6">
      <PageHeader
        title="Runs"
        subtitle="Aggregate run history across projects."
      />

      {projectsQuery.error && (
        <ErrorBanner message={(projectsQuery.error as Error).message} onRetry={() => projectsQuery.refetch()} />
      )}
      {runsQuery.error && (
        <ErrorBanner message={(runsQuery.error as Error).message} onRetry={() => runsQuery.refetch()} />
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
            placeholder="Search runs"
            value={search}
            onChange={(event) => setSearch(event.target.value)}
          />
        </div>
      </div>

      {filteredRuns.length === 0 ? (
        <EmptyState title="No runs found" description="Try adjusting your filters." />
      ) : (
        <DataTable>
          <DataTableHeader>
            <DataTableHeaderCell>Run</DataTableHeaderCell>
            <DataTableHeaderCell>Status</DataTableHeaderCell>
            <DataTableHeaderCell>Project</DataTableHeaderCell>
            <DataTableHeaderCell>Started</DataTableHeaderCell>
            <DataTableHeaderCell>Trigger</DataTableHeaderCell>
          </DataTableHeader>
          <DataTableBody>
            {filteredRuns.map((run) => (
              <tr
                key={run.id}
                className="cursor-pointer hover:bg-ink-50"
                onClick={() => navigate(`/${run.project_slug}/${run.slug}`)}
              >
                <DataTableCell>
                  <div className="flex items-center gap-2">
                    <div className="font-semibold text-ink-900">{run.name ?? run.slug}</div>
                    {run.rerun_of_run_id && (
                      <span className="rounded-full bg-ink-100 px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-ink-500">
                        Re-run
                      </span>
                    )}
                  </div>
                  <div className="text-xs text-ink-500">{run.slug}</div>
                </DataTableCell>
                <DataTableCell>
                  <StatusPill status={run.status} />
                </DataTableCell>
                <DataTableCell>
                  <div className="text-sm font-semibold text-ink-900">{run.project_name}</div>
                </DataTableCell>
                <DataTableCell>
                  <RelativeTime value={run.started_at ?? run.created_at} />
                </DataTableCell>
                <DataTableCell>{run.trigger}</DataTableCell>
              </tr>
            ))}
          </DataTableBody>
        </DataTable>
      )}
    </div>
  );
}
