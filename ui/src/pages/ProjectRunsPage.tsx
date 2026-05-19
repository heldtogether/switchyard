import { useMemo, useState } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { getProject, listCurrentPromotions, listRuns } from "../api";
import { PageHeader } from "../components/PageHeader";
import { DataTable, DataTableBody, DataTableCell, DataTableHeader, DataTableHeaderCell } from "../components/DataTable";
import { StatusPill } from "../components/StatusPill";
import { EmptyState } from "../components/EmptyState";
import { ErrorBanner } from "../components/ErrorBanner";
import { RelativeTime } from "../components/RelativeTime";
import { Breadcrumbs } from "../components/Breadcrumbs";
import { NewRunModal } from "./NewRunModal";

export function ProjectRunsPage() {
  const { workspace = "", projectSlug = "" } = useParams();
  const navigate = useNavigate();
  const [statusFilter, setStatusFilter] = useState("all");
  const [search, setSearch] = useState("");
  const [newRunOpen, setNewRunOpen] = useState(false);

  const projectQuery = useQuery({
    queryKey: ["project", workspace, projectSlug],
    queryFn: () => getProject(projectSlug, workspace)
  });

  const runsQuery = useQuery({
    queryKey: ["runs", workspace, projectSlug],
    queryFn: () => listRuns(projectSlug, workspace)
  });

  const promotionsQuery = useQuery({
    queryKey: ["promotions", workspace, projectSlug],
    queryFn: () => listCurrentPromotions(projectSlug, workspace)
  });

  const filteredRuns = useMemo(() => {
    if (!runsQuery.data) return [];
    return runsQuery.data.filter((run) => {
      const matchesStatus = statusFilter === "all" || run.status === statusFilter;
      const matchesSearch =
        run.name?.toLowerCase().includes(search.toLowerCase()) ||
        run.slug?.toLowerCase().includes(search.toLowerCase());
      return matchesStatus && matchesSearch;
    });
  }, [runsQuery.data, statusFilter, search]);

  return (
    <div className="space-y-6">
      <PageHeader
        breadcrumbs={
          <Breadcrumbs
            items={[
              { label: "Projects", to: `/${workspace}` },
              { label: projectQuery.data?.name ?? projectSlug }
            ]}
          />
        }
        title={projectQuery.data?.name ?? "Project"}
        subtitle={projectQuery.data?.description ?? "Runs and promotions for this project."}
        meta={
          <div className="flex flex-wrap gap-4 text-xs text-ink-500">
            <span>Runs: {runsQuery.data?.length ?? 0}</span>
            <span>Updated: <RelativeTime value={projectQuery.data?.updated_at} /></span>
          </div>
        }
        actions={
          <button
            type="button"
            onClick={() => setNewRunOpen(true)}
            className="rounded-full bg-ink-900 px-4 py-2 text-sm font-semibold text-white"
          >
            New Run
          </button>
        }
      />

      {projectQuery.error && (
        <ErrorBanner
          message={(projectQuery.error as Error).message}
          onRetry={() => projectQuery.refetch()}
        />
      )}

      <div className="grid gap-4 md:grid-cols-3">
        <div className="card p-4">
          <p className="text-xs uppercase tracking-[0.2em] text-ink-400">Current Promotions</p>
          <div className="mt-4 space-y-3">
            {(promotionsQuery.data ?? []).length === 0 && <p className="text-sm text-ink-500">No promotions yet.</p>}
            {(promotionsQuery.data ?? []).map((promo) => (
              <div key={promo.event.id} className="flex items-center justify-between">
                <span className="text-sm font-semibold text-ink-900 capitalize">{promo.channel}</span>
                <span className="text-xs text-ink-500">Run {promo.event.run_id}</span>
              </div>
            ))}
          </div>
        </div>
        <div className="card p-4 md:col-span-2">
          <p className="text-xs uppercase tracking-[0.2em] text-ink-400">Filters</p>
          <div className="mt-4 grid gap-3 md:grid-cols-3">
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
            <input
              className="rounded-lg border border-ink-200 px-3 py-2 text-sm"
              placeholder="Search runs"
              value={search}
              onChange={(event) => setSearch(event.target.value)}
            />
            <button className="rounded-lg border border-ink-200 px-3 py-2 text-sm text-ink-500">
              Date range (stub)
            </button>
          </div>
        </div>
      </div>

      {runsQuery.error && (
        <ErrorBanner message={(runsQuery.error as Error).message} onRetry={() => runsQuery.refetch()} />
      )}

      {filteredRuns.length === 0 ? (
        <EmptyState
          title="No runs yet"
          description="Runs appear when jobs are submitted or scheduled."
        />
      ) : (
        <DataTable>
          <DataTableHeader>
            <DataTableHeaderCell>Run</DataTableHeaderCell>
            <DataTableHeaderCell>Status</DataTableHeaderCell>
            <DataTableHeaderCell>Jobs</DataTableHeaderCell>
            <DataTableHeaderCell>Started</DataTableHeaderCell>
            <DataTableHeaderCell>Trigger</DataTableHeaderCell>
            <DataTableHeaderCell>Tags</DataTableHeaderCell>
          </DataTableHeader>
          <DataTableBody>
            {filteredRuns.map((run) => (
              <tr
                key={run.id}
                className="cursor-pointer hover:bg-ink-50"
                onClick={() => navigate(`/${workspace}/${projectSlug}/${run.slug}`)}
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
                <DataTableCell>{run.jobs_count}</DataTableCell>
                <DataTableCell>
                  <RelativeTime value={run.started_at ?? run.created_at} />
                </DataTableCell>
                <DataTableCell>{run.trigger}</DataTableCell>
                <DataTableCell>
                  <div className="flex flex-wrap gap-1">
                    {[...run.tags_user, ...run.tags_system].map((tag) => (
                      <span key={tag} className="rounded-full bg-ink-100 px-2 py-0.5 text-xs text-ink-500">
                        {tag}
                      </span>
                    ))}
                  </div>
                </DataTableCell>
              </tr>
            ))}
          </DataTableBody>
        </DataTable>
      )}

      <NewRunModal
        open={newRunOpen}
        projectSlug={projectSlug}
        onClose={() => setNewRunOpen(false)}
        onSuccess={(runSlug) => {
          runsQuery.refetch();
          navigate(`/${workspace}/${projectSlug}/${runSlug}`);
        }}
      />
    </div>
  );
}
