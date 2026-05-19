import { useMemo } from "react";
import { Link, useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { getWorkspaceMonthToDateBilling, listProjects, listRuns, getRunBillingBreakdown } from "../api";
import { PageHeader } from "../components/PageHeader";
import { Breadcrumbs } from "../components/Breadcrumbs";
import { DataTable, DataTableBody, DataTableCell, DataTableHeader, DataTableHeaderCell } from "../components/DataTable";
import { ErrorBanner } from "../components/ErrorBanner";
import { EmptyState } from "../components/EmptyState";
import { StatusPill } from "../components/StatusPill";
import { formatCurrencyFromMinorExact } from "../utils/format";

type BilledRunRow = {
  project_slug: string;
  project_name: string;
  run_slug: string;
  run_name: string;
  run_status: string;
  estimated_total_minor_exact: number;
  currency: string;
};

export function BillingPage() {
  const { workspace = "" } = useParams();

  const mtdQuery = useQuery({
    queryKey: ["billing", workspace, "month-to-date"],
    queryFn: () => getWorkspaceMonthToDateBilling(workspace)
  });

  const runsQuery = useQuery({
    queryKey: ["billing", workspace, "recent-runs"],
    queryFn: async (): Promise<BilledRunRow[]> => {
      const projects = await listProjects(workspace);
      const runCollections = await Promise.all(
        projects.map(async (project) => {
          const runs = await listRuns(project.slug, workspace);
          return runs.map((run) => ({ project, run }));
        })
      );
      const flattened = runCollections.flat();
      const recent = flattened
        .sort((a, b) => new Date(b.run.created_at).getTime() - new Date(a.run.created_at).getTime())
        .slice(0, 12);

      const results = await Promise.allSettled(
        recent.map(async ({ project, run }) => {
          const billing = await getRunBillingBreakdown(project.slug, run.slug, workspace);
          return {
            project_slug: project.slug,
            project_name: project.name,
            run_slug: run.slug,
            run_name: run.name ?? run.slug,
            run_status: run.status,
            estimated_total_minor_exact: billing.estimated_total_minor_exact ?? 0,
            currency: billing.currency ?? "USD"
          } satisfies BilledRunRow;
        })
      );

      return results
        .filter((r): r is PromiseFulfilledResult<BilledRunRow> => r.status === "fulfilled")
        .map((r) => r.value)
        .filter((row) => row.estimated_total_minor_exact !== 0);
    }
  });

  const totalEstimatedDisplay = useMemo(() => {
    return formatCurrencyFromMinorExact(
      mtdQuery.data?.estimated_total_minor_exact ?? 0,
      mtdQuery.data?.currency ?? "USD"
    );
  }, [mtdQuery.data]);

  return (
    <div className="space-y-6">
      <PageHeader
        breadcrumbs={<Breadcrumbs items={[{ label: "Billing" }]} />}
        title="Billing"
        subtitle="Month-to-date usage and estimated spend."
      />

      {mtdQuery.error && (
        <ErrorBanner message={(mtdQuery.error as Error).message} onRetry={() => mtdQuery.refetch()} />
      )}
      {runsQuery.error && (
        <ErrorBanner message={(runsQuery.error as Error).message} onRetry={() => runsQuery.refetch()} />
      )}

      <div className="grid gap-4 md:grid-cols-5">
        <div className="card p-4 md:col-span-2">
          <p className="text-xs uppercase tracking-[0.2em] text-ink-400">Estimated month-to-date</p>
          <p className="mt-3 text-2xl font-semibold text-ink-900">{totalEstimatedDisplay}</p>
          <p className="mt-1 text-xs text-ink-500">Month: {mtdQuery.data?.month_key ?? "—"}</p>
        </div>
        <div className="card p-4">
          <p className="text-xs uppercase tracking-[0.2em] text-ink-400">CPU Seconds</p>
          <p className="mt-3 text-2xl font-semibold text-ink-900">
            {(mtdQuery.data?.cpu_seconds ?? 0).toLocaleString(undefined, { maximumFractionDigits: 3 })}
          </p>
        </div>
        <div className="card p-4">
          <p className="text-xs uppercase tracking-[0.2em] text-ink-400">Memory GB-seconds</p>
          <p className="mt-3 text-2xl font-semibold text-ink-900">
            {(mtdQuery.data?.memory_gb_seconds ?? 0).toLocaleString(undefined, { maximumFractionDigits: 3 })}
          </p>
        </div>
        <div className="card p-4">
          <p className="text-xs uppercase tracking-[0.2em] text-ink-400">GPU Seconds</p>
          <p className="mt-3 text-2xl font-semibold text-ink-900">
            {(mtdQuery.data?.gpu_seconds ?? 0).toLocaleString(undefined, { maximumFractionDigits: 3 })}
          </p>
        </div>
      </div>

      {runsQuery.data && runsQuery.data.length > 0 ? (
        <DataTable>
          <DataTableHeader>
            <DataTableHeaderCell>Run</DataTableHeaderCell>
            <DataTableHeaderCell>Status</DataTableHeaderCell>
            <DataTableHeaderCell>Estimated Price</DataTableHeaderCell>
          </DataTableHeader>
          <DataTableBody>
            {runsQuery.data.map((row) => (
              <tr key={`${row.project_slug}:${row.run_slug}`} className="hover:bg-ink-50">
                <DataTableCell>
                  <div className="font-semibold text-ink-900">
                    <Link to={`/${workspace}/${row.project_slug}/${row.run_slug}`} className="underline">
                      {row.run_name}
                    </Link>
                  </div>
                  <div className="text-xs text-ink-500">{row.project_name}</div>
                </DataTableCell>
                <DataTableCell>
                  <StatusPill status={row.run_status as any} />
                </DataTableCell>
                <DataTableCell>
                  {formatCurrencyFromMinorExact(row.estimated_total_minor_exact, row.currency)}
                </DataTableCell>
              </tr>
            ))}
          </DataTableBody>
        </DataTable>
      ) : (
        <EmptyState
          title="No billed runs yet"
          description="Once runs complete, estimated prices will appear here."
        />
      )}
    </div>
  );
}
