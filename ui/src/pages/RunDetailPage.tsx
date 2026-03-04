import React, { useMemo, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { getProject, getRun, listJobs, listArtefacts, savePromotion, listPromotions, rerunRun } from "../api";
import { PageHeader } from "../components/PageHeader";
import { StatusPill } from "../components/StatusPill";
import { Tabs } from "../components/Tabs";
import { DataTable, DataTableBody, DataTableCell, DataTableHeader, DataTableHeaderCell } from "../components/DataTable";
import { RelativeTime } from "../components/RelativeTime";
import { formatDurationMs } from "../utils/format";
import { JSONViewer } from "../components/JSONViewer";
import { Modal } from "../components/Modal";
import { ArtefactList } from "../components/ArtefactList";
import { ErrorBanner } from "../components/ErrorBanner";
import { EmptyState } from "../components/EmptyState";
import { Breadcrumbs } from "../components/Breadcrumbs";

export function RunDetailPage() {
  const { projectSlug = "", runSlug = "" } = useParams();
  const navigate = useNavigate();
  const [tab, setTab] = useState("overview");
  const [promoOpen, setPromoOpen] = useState(false);
  const [promoChannel, setPromoChannel] = useState("dev");
  const [promoNote, setPromoNote] = useState("");
  const [promoMode, setPromoMode] = useState<"run" | "artefacts">("run");
  const [selectedArtefacts, setSelectedArtefacts] = useState<string[]>([]);
  const [rerunOpen, setRerunOpen] = useState(false);
  const [rerunSubmitting, setRerunSubmitting] = useState(false);
  const [rerunError, setRerunError] = useState<string | null>(null);

  const runQuery = useQuery({
    queryKey: ["run", projectSlug, runSlug],
    queryFn: () => getRun(projectSlug, runSlug)
  });

  const projectQuery = useQuery({
    queryKey: ["project", projectSlug],
    queryFn: () => getProject(projectSlug)
  });

  const jobsQuery = useQuery({
    queryKey: ["jobs", projectSlug, runSlug],
    queryFn: () => listJobs(projectSlug, runSlug)
  });

  const artefactsQuery = useQuery({
    queryKey: ["artefacts", projectSlug, runSlug],
    queryFn: async () => {
      if (!jobsQuery.data) return {};
      const entries = await Promise.all(
        jobsQuery.data.map(async (job) => [job.id, await listArtefacts(projectSlug, runSlug, job.id)] as const)
      );
      return Object.fromEntries(entries);
    },
    enabled: !!jobsQuery.data
  });

  const specPayload = useMemo(() => {
    return {
      run: runQuery.data,
      jobs: jobsQuery.data
    };
  }, [runQuery.data, jobsQuery.data]);

  const allArtefacts = useMemo(() => {
    if (!artefactsQuery.data) return [];
    return Object.values(artefactsQuery.data as Record<string, any[]>).flat();
  }, [artefactsQuery.data]);

  const promotedArtefacts = useMemo(() => {
    if (!runQuery.data) return [];
    const promos = listPromotions(runQuery.data.project_id);
    const fromRun = promos.filter((promo) => promo.run_id === runQuery.data.id);
    const keys = new Set(fromRun.flatMap((promo) => promo.artefact_keys ?? []));
    return allArtefacts.filter((art: any) => keys.has(art.object_key));
  }, [runQuery.data, allArtefacts]);

  if (runQuery.error) {
    return <ErrorBanner message={(runQuery.error as Error).message} onRetry={() => runQuery.refetch()} />;
  }

  async function handleRerun(mode: "all" | "failed_only") {
    if (!runQuery.data) return;
    setRerunSubmitting(true);
    setRerunError(null);
    setRerunOpen(false);
    try {
      const res = await rerunRun(projectSlug, runSlug, { mode });
      const newRunSlug = res.run?.slug;
      if (!newRunSlug) throw new Error("Rerun created but missing run slug");
      navigate(`/${projectSlug}/${newRunSlug}`);
    } catch (error) {
      setRerunError((error as Error).message ?? "Failed to create rerun");
    } finally {
      setRerunSubmitting(false);
    }
  }

  return (
    <div className="space-y-6">
      {rerunError && <ErrorBanner message={rerunError} onRetry={() => setRerunError(null)} />}
      <PageHeader
        breadcrumbs={
          <Breadcrumbs
            items={[
              { label: "Projects", to: "/" },
              { label: projectQuery.data?.name ?? projectSlug, to: `/${projectSlug}` },
              { label: runQuery.data?.name ?? runSlug }
            ]}
          />
        }
        title={`Run · ${runQuery.data?.name ?? runQuery.data?.slug ?? runSlug}`}
        subtitle={`Created by ${runQuery.data?.created_by ?? "system"}`}
        meta={
          <div className="flex flex-wrap gap-4 text-xs text-ink-500">
            <span>Started: <RelativeTime value={runQuery.data?.started_at ?? runQuery.data?.created_at} /></span>
            <span>Updated: <RelativeTime value={runQuery.data?.updated_at} /></span>
            {runQuery.data?.rerun_of_run_slug && (
              <span>
                Re-run of:{" "}
                <Link to={`/${projectSlug}/${runQuery.data.rerun_of_run_slug}`} className="text-ink-700 underline">
                  {runQuery.data.rerun_of_run_slug}
                </Link>
              </span>
            )}
            {runQuery.data?.rerun_mode && (
              <span>Mode: {runQuery.data.rerun_mode === "failed_only" ? "failed-only" : "all jobs"}</span>
            )}
          </div>
        }
        actions={
          <div className="flex flex-wrap gap-2">
            <StatusPill status={runQuery.data?.status ?? "PENDING"} />
            <div className="relative">
              <button
                type="button"
                onClick={() => setRerunOpen((open) => !open)}
                disabled={rerunSubmitting}
                className="rounded-full border border-ink-200 px-3 py-1 text-sm text-ink-500 disabled:opacity-60"
              >
                {rerunSubmitting ? "Re-running..." : "Re-run"}
              </button>
              {rerunOpen && (
                <div className="absolute right-0 z-20 mt-2 w-56 rounded-xl border border-ink-200 bg-white p-1 shadow-lg">
                  <button
                    type="button"
                    onClick={() => handleRerun("all")}
                    className="block w-full rounded-lg px-3 py-2 text-left text-sm text-ink-700 hover:bg-ink-50"
                  >
                    Re-run all jobs
                  </button>
                  <button
                    type="button"
                    onClick={() => handleRerun("failed_only")}
                    className="block w-full rounded-lg px-3 py-2 text-left text-sm text-ink-700 hover:bg-ink-50"
                  >
                    Re-run failed jobs only
                  </button>
                </div>
              )}
            </div>
            <button className="rounded-full border border-ink-200 px-3 py-1 text-sm text-ink-500">Cancel</button>
            <button
              className="rounded-full bg-ink-900 px-3 py-1 text-sm font-semibold text-white"
              onClick={() => setPromoOpen(true)}
            >
              Promote
            </button>
          </div>
        }
      />

      <Tabs
        tabs={[
          { id: "overview", label: "Overview" },
          { id: "jobs", label: "Jobs" },
          { id: "artefacts", label: "Artefacts" },
          { id: "spec", label: "Spec" }
        ]}
        active={tab}
        onChange={setTab}
      />

      {tab === "overview" && (
        <div className="space-y-6">
          <div className="grid gap-4 md:grid-cols-3">
            <div className="card p-4">
              <p className="text-xs uppercase tracking-[0.2em] text-ink-400">Status</p>
              <div className="mt-3">
                <StatusPill status={runQuery.data?.status ?? "PENDING"} />
              </div>
            </div>
            <div className="card p-4">
              <p className="text-xs uppercase tracking-[0.2em] text-ink-400">Jobs</p>
              <p className="mt-3 text-2xl font-semibold text-ink-900">
                {jobsQuery.data?.length ?? 0}
              </p>
            </div>
          </div>

          {jobsQuery.data && jobsQuery.data.length > 0 ? (
            <DataTable>
              <DataTableHeader>
                <DataTableHeaderCell>Job</DataTableHeaderCell>
                <DataTableHeaderCell>Status</DataTableHeaderCell>
                <DataTableHeaderCell>Duration</DataTableHeaderCell>
                <DataTableHeaderCell>Executor</DataTableHeaderCell>
              </DataTableHeader>
              <DataTableBody>
                {jobsQuery.data.map((job) => (
                  <tr
                    key={job.id}
                    className="cursor-pointer hover:bg-ink-50"
                    onClick={() => navigate(`/${projectSlug}/${runSlug}/${job.id}`)}
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
                    <DataTableCell>{formatDurationMs(job.duration)}</DataTableCell>
                    <DataTableCell>{job.executor_type}</DataTableCell>
                  </tr>
                ))}
              </DataTableBody>
            </DataTable>
          ) : (
            <EmptyState title="No jobs yet" description="Jobs will appear once the run starts." />
          )}
        </div>
      )}

      {tab === "jobs" && (
        <div className="space-y-4">
          {jobsQuery.data && jobsQuery.data.length > 0 ? (
            <DataTable>
              <DataTableHeader>
                <DataTableHeaderCell>Job</DataTableHeaderCell>
                <DataTableHeaderCell>Status</DataTableHeaderCell>
                <DataTableHeaderCell>Duration</DataTableHeaderCell>
                <DataTableHeaderCell>Executor</DataTableHeaderCell>
              </DataTableHeader>
              <DataTableBody>
                {jobsQuery.data.map((job) => (
                  <tr
                    key={job.id}
                    className="cursor-pointer hover:bg-ink-50"
                    onClick={() => navigate(`/${projectSlug}/${runSlug}/${job.id}`)}
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
                    <DataTableCell>{formatDurationMs(job.duration)}</DataTableCell>
                    <DataTableCell>{job.executor_type}</DataTableCell>
                  </tr>
                ))}
              </DataTableBody>
            </DataTable>
          ) : (
            <EmptyState title="No jobs yet" description="Jobs will appear once the run starts." />
          )}
        </div>
      )}

      {tab === "artefacts" && (
        <div className="space-y-4">
          {promotedArtefacts.length > 0 && (
            <div className="card p-4">
              <p className="text-xs uppercase tracking-[0.2em] text-ink-400">Pinned Artefacts</p>
              <div className="mt-3">
                <ArtefactList artefacts={promotedArtefacts} />
              </div>
            </div>
          )}
          {jobsQuery.data?.map((job) => (
            <div key={job.id} className="space-y-2">
              <div className="flex items-center justify-between">
                <h3 className="text-sm font-semibold text-ink-900">{job.name}</h3>
                <span className="text-xs text-ink-400">{job.image}</span>
              </div>
              <ArtefactList artefacts={(artefactsQuery.data as any)?.[job.id] ?? []} />
            </div>
          ))}
        </div>
      )}

      {tab === "spec" && (
        <JSONViewer
          data={specPayload}
          onCopy={() => navigator.clipboard.writeText(JSON.stringify(specPayload, null, 2))}
        />
      )}

      <Modal
        open={promoOpen}
        title="Promote Run"
        description="Promotions are stored locally until a backend endpoint is available."
        onClose={() => setPromoOpen(false)}
        footer={
          <div className="flex justify-end gap-2">
            <button type="button" onClick={() => setPromoOpen(false)} className="text-sm text-ink-500">
              Cancel
            </button>
            <button
              type="button"
              onClick={() => {
                if (!runQuery.data) return;
                savePromotion({
                  id: `promo-${Date.now()}`,
                  project_id: runQuery.data.project_id,
                  channel: promoChannel as any,
                  run_id: runQuery.data.id,
                  promoted_at: new Date().toISOString(),
                  promoted_by: "you",
                  note: promoNote,
                  artefact_keys: promoMode === "artefacts" ? selectedArtefacts : undefined
                });
                setPromoOpen(false);
              }}
              className="rounded-full bg-ink-900 px-4 py-2 text-sm font-semibold text-white"
            >
              Confirm Promotion
            </button>
          </div>
        }
      >
        <div className="space-y-4 text-sm text-ink-600">
          <div>
            <label className="text-xs uppercase tracking-[0.2em] text-ink-400">Channel</label>
            <select
              className="mt-2 w-full rounded-lg border border-ink-200 px-3 py-2"
              value={promoChannel}
              onChange={(event) => setPromoChannel(event.target.value)}
            >
              <option value="dev">dev</option>
              <option value="staging">staging</option>
              <option value="prod">prod</option>
              <option value="validated">validated</option>
            </select>
          </div>
          <div>
            <label className="text-xs uppercase tracking-[0.2em] text-ink-400">Scope</label>
            <div className="mt-2 space-y-2 text-sm text-ink-600">
              <label className="flex items-center gap-2">
                <input
                  type="radio"
                  checked={promoMode === "run"}
                  onChange={() => setPromoMode("run")}
                />
                Entire run
              </label>
              <label className="flex items-center gap-2">
                <input
                  type="radio"
                  checked={promoMode === "artefacts"}
                  onChange={() => setPromoMode("artefacts")}
                />
                Specific artefacts
              </label>
            </div>
            {promoMode === "artefacts" && (
              <div className="mt-3 max-h-40 space-y-2 overflow-auto rounded-lg border border-ink-200 p-3 text-xs">
                {allArtefacts.length === 0 && <div className="text-ink-400">No artefacts to select.</div>}
                {allArtefacts.map((art: any) => (
                  <label key={art.id} className="flex items-center gap-2">
                    <input
                      type="checkbox"
                      checked={selectedArtefacts.includes(art.object_key)}
                      onChange={(event) => {
                        setSelectedArtefacts((prev) =>
                          event.target.checked
                            ? [...prev, art.object_key]
                            : prev.filter((key) => key !== art.object_key)
                        );
                      }}
                    />
                    <span className="text-ink-700">{art.path}</span>
                  </label>
                ))}
              </div>
            )}
          </div>
          <div>
            <label className="text-xs uppercase tracking-[0.2em] text-ink-400">Note</label>
            <textarea
              className="mt-2 w-full rounded-lg border border-ink-200 px-3 py-2"
              rows={3}
              value={promoNote}
              onChange={(event) => setPromoNote(event.target.value)}
              placeholder="Optional promotion context"
            />
          </div>
        </div>
      </Modal>
    </div>
  );
}
