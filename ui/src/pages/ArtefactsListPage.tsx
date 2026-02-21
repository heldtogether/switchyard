import React, { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { listProjects, listAllArtefacts } from "../api";
import { PageHeader } from "../components/PageHeader";
import { DataTable, DataTableBody, DataTableCell, DataTableHeader, DataTableHeaderCell } from "../components/DataTable";
import { EmptyState } from "../components/EmptyState";
import { ErrorBanner } from "../components/ErrorBanner";
import { formatBytes } from "../utils/format";

export function ArtefactsListPage() {
  const [projectFilter, setProjectFilter] = useState("all");
  const [search, setSearch] = useState("");

  const projectsQuery = useQuery({
    queryKey: ["projects"],
    queryFn: listProjects
  });

  const artefactsQuery = useQuery({
    queryKey: ["artefacts", "all"],
    queryFn: listAllArtefacts,
    enabled: !!projectsQuery.data
  });

  const filtered = useMemo(() => {
    return (artefactsQuery.data ?? [])
      .filter((art) => (projectFilter === "all" ? true : art.project_slug === projectFilter))
      .filter((art) => art.path.toLowerCase().includes(search.toLowerCase()));
  }, [artefactsQuery.data, projectFilter, search]);

  return (
    <div className="space-y-6">
      <PageHeader title="Artefacts" subtitle="Browse outputs across all runs." />

      {projectsQuery.error && (
        <ErrorBanner message={(projectsQuery.error as Error).message} onRetry={() => projectsQuery.refetch()} />
      )}
      {artefactsQuery.error && (
        <ErrorBanner message={(artefactsQuery.error as Error).message} onRetry={() => artefactsQuery.refetch()} />
      )}

      <div className="card p-4">
        <div className="grid gap-3 md:grid-cols-3">
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
            placeholder="Search artefacts"
            value={search}
            onChange={(event) => setSearch(event.target.value)}
          />
          <div className="rounded-lg border border-ink-200 px-3 py-2 text-sm text-ink-400">Content type (stub)</div>
        </div>
      </div>

      {filtered.length === 0 ? (
        <EmptyState title="No artefacts found" description="Try adjusting your filters." />
      ) : (
        <DataTable>
          <DataTableHeader>
            <DataTableHeaderCell>Artefact</DataTableHeaderCell>
            <DataTableHeaderCell>Job</DataTableHeaderCell>
            <DataTableHeaderCell>Run</DataTableHeaderCell>
            <DataTableHeaderCell>Size</DataTableHeaderCell>
            <DataTableHeaderCell>Download</DataTableHeaderCell>
          </DataTableHeader>
          <DataTableBody>
            {filtered.map((art) => (
              <tr key={`${art.id}-${art.path}`} className="hover:bg-ink-50">
                <DataTableCell>
                  <div className="font-semibold text-ink-900">{art.path}</div>
                  <div className="text-xs text-ink-500">{art.project_name}</div>
                </DataTableCell>
                <DataTableCell>
                  <div className="font-semibold text-ink-900">{art.job_name}</div>
                  {art.job_image && art.job_image !== art.job_name && (
                    <div className="text-xs text-ink-500">{art.job_image}</div>
                  )}
                </DataTableCell>
                <DataTableCell>#{art.run_number}</DataTableCell>
                <DataTableCell>{formatBytes(art.size_bytes)}</DataTableCell>
                <DataTableCell>
                  {art.download_url ? (
                    <a
                      className="rounded-full border border-ink-200 px-3 py-1 text-xs text-ink-500"
                      href={art.download_url}
                    >
                      Download
                    </a>
                  ) : (
                    <span className="text-xs text-ink-400">Unavailable</span>
                  )}
                </DataTableCell>
              </tr>
            ))}
          </DataTableBody>
        </DataTable>
      )}
    </div>
  );
}
