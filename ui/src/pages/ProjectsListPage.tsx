import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { createProject, listProjects, listRuns } from "../api";
import { PageHeader } from "../components/PageHeader";
import { DataTable, DataTableBody, DataTableCell, DataTableHeader, DataTableHeaderCell } from "../components/DataTable";
import { StatusPill } from "../components/StatusPill";
import { EmptyState } from "../components/EmptyState";
import { ErrorBanner } from "../components/ErrorBanner";
import { Skeleton } from "../components/Skeleton";
import { RelativeTime } from "../components/RelativeTime";
import { Modal } from "../components/Modal";
import { useNavigate, useParams } from "react-router-dom";

export function ProjectsListPage() {
  const navigate = useNavigate();
  const { workspace = "" } = useParams();
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [slug, setSlug] = useState("");
  const [slugManuallyEdited, setSlugManuallyEdited] = useState(false);
  const [description, setDescription] = useState("");
  const [createError, setCreateError] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ["projects", workspace],
    queryFn: listProjects
  });

  const { data: runsData } = useQuery({
    queryKey: ["projects", workspace, "runs"],
    queryFn: async () => {
      if (!data) return {} as Record<string, Awaited<ReturnType<typeof listRuns>>>;
      const entries = await Promise.all(
        data.map(async (project) => [project.slug, await listRuns(project.slug)] as const)
      );
      return Object.fromEntries(entries);
    },
    enabled: !!data
  });

  const rows = useMemo(() => {
    if (!data) return [];
    return data.map((project) => {
      const runs = runsData?.[project.slug] ?? [];
      const lastRun = runs[0];
      const successRate = runs.length
        ? Math.round(
            (runs.filter((run) => run.status === "SUCCEEDED").length / runs.length) * 100
          )
        : 0;
      return { project, lastRun, successRate };
    });
  }, [data, runsData]);

  async function onCreateProject() {
    if (!name.trim() || !slug.trim()) {
      setCreateError("Name and slug are required.");
      return;
    }
    setCreating(true);
    setCreateError(null);
    try {
      await createProject({
        name: name.trim(),
        slug: slug.trim(),
        description: description.trim() || undefined
      });
      setOpen(false);
      setName("");
      setSlug("");
      setSlugManuallyEdited(false);
      setDescription("");
      await refetch();
    } catch (error) {
      setCreateError((error as Error).message);
    } finally {
      setCreating(false);
    }
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title="Projects"
        subtitle="Workspaces you ship from. Keep runs, jobs, and artefacts organized." 
        actions={
          <button
            type="button"
            onClick={() => setOpen(true)}
            className="rounded-full bg-ink-900 px-4 py-2 text-sm font-semibold text-white"
          >
            New Project
          </button>
        }
      />

      {error && (
        <ErrorBanner message={(error as Error).message} onRetry={() => refetch()} />
      )}

      {isLoading && (
        <div className="grid gap-4 md:grid-cols-2">
          <Skeleton className="h-32" />
          <Skeleton className="h-32" />
        </div>
      )}

      {!isLoading && rows.length === 0 && (
        <EmptyState
          title="No projects yet"
          description="Create your first project to start tracking runs and artefacts."
          action={
            <button
              type="button"
              onClick={() => setOpen(true)}
              className="rounded-full bg-ink-900 px-4 py-2 text-sm font-semibold text-white"
            >
              Create Project
            </button>
          }
        />
      )}

      {rows.length > 0 && (
        <DataTable>
          <DataTableHeader>
            <DataTableHeaderCell>Project</DataTableHeaderCell>
            <DataTableHeaderCell>Last Run</DataTableHeaderCell>
            <DataTableHeaderCell>Activity</DataTableHeaderCell>
            <DataTableHeaderCell>7-day Success</DataTableHeaderCell>
          </DataTableHeader>
          <DataTableBody>
            {rows.map(({ project, lastRun, successRate }) => (
              <tr
                key={project.id}
                className="cursor-pointer hover:bg-ink-50"
                onClick={() => navigate(`/${workspace}/${project.slug}`)}
              >
                <DataTableCell>
                  <div className="font-semibold text-ink-900">{project.name}</div>
                  <div className="text-xs text-ink-500">{project.description}</div>
                </DataTableCell>
                <DataTableCell>
                  {lastRun ? (
                    <div className="flex items-center gap-2">
                      <span className="text-sm font-semibold">#{lastRun.run_number}</span>
                      <StatusPill status={lastRun.status} />
                    </div>
                  ) : (
                    <span className="text-sm text-ink-400">No runs yet</span>
                  )}
                </DataTableCell>
                <DataTableCell>
                  <RelativeTime value={project.updated_at} />
                </DataTableCell>
                <DataTableCell>
                  <span className="text-sm font-semibold text-ink-900">{successRate}%</span>
                </DataTableCell>
              </tr>
            ))}
          </DataTableBody>
        </DataTable>
      )}

      <Modal
        open={open}
        title="New Project"
        description="Create a project in the current workspace."
        onClose={() => setOpen(false)}
        footer={
          <div className="flex justify-end gap-2">
            <button type="button" onClick={() => setOpen(false)} className="text-sm text-ink-500">
              Close
            </button>
            <button
              type="button"
              onClick={onCreateProject}
              disabled={creating}
              className="rounded-full bg-ink-900 px-4 py-2 text-sm font-semibold text-white disabled:opacity-60"
            >
              Create
            </button>
          </div>
        }
      >
        <div className="space-y-4 text-sm text-ink-600">
          {createError && <p className="text-sm text-red-600">{createError}</p>}
          <div>
            <label className="text-xs uppercase tracking-[0.2em] text-ink-400">Name</label>
            <input
              className="mt-2 w-full rounded-lg border border-ink-200 px-3 py-2"
              placeholder="Vision Core"
              value={name}
              onChange={(e) => {
                const nextName = e.target.value;
                setName(nextName);
                if (!slugManuallyEdited) {
                  setSlug(nextName.toLowerCase().trim().replace(/[^a-z0-9]+/g, "-").replace(/(^-|-$)/g, ""));
                }
              }}
            />
          </div>
          <div>
            <label className="text-xs uppercase tracking-[0.2em] text-ink-400">Slug</label>
            <input
              className="mt-2 w-full rounded-lg border border-ink-200 px-3 py-2"
              placeholder="vision-core"
              value={slug}
              onChange={(e) => {
                const nextSlug = e.target.value;
                setSlug(nextSlug);
                setSlugManuallyEdited(nextSlug.trim().length > 0);
              }}
            />
          </div>
          <div>
            <label className="text-xs uppercase tracking-[0.2em] text-ink-400">Description</label>
            <textarea
              className="mt-2 w-full rounded-lg border border-ink-200 px-3 py-2"
              placeholder="Optional description"
              rows={3}
              value={description}
              onChange={(e) => setDescription(e.target.value)}
            />
          </div>
        </div>
      </Modal>
    </div>
  );
}
