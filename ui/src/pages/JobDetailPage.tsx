import { useMemo, useState } from "react";
import { useParams } from "react-router-dom";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { cancelJob, getJob, getJobLogs, getProject, getRun, listArtefacts } from "../api";
import { PageHeader } from "../components/PageHeader";
import { StatusPill } from "../components/StatusPill";
import { Tabs } from "../components/Tabs";
import { LogViewer } from "../components/LogViewer";
import { ArtefactList } from "../components/ArtefactList";
import { JSONViewer } from "../components/JSONViewer";
import { RelativeTime } from "../components/RelativeTime";
import { formatDurationMs } from "../utils/format";
import { ErrorBanner } from "../components/ErrorBanner";
import { Breadcrumbs } from "../components/Breadcrumbs";

export function JobDetailPage() {
  const { workspace = "", projectSlug = "", runSlug = "", jobId = "" } = useParams();
  const queryClient = useQueryClient();
  const [tab, setTab] = useState("logs");
  const [autoScroll, setAutoScroll] = useState(true);
  const [cancelError, setCancelError] = useState<string | null>(null);

  const jobQuery = useQuery({
    queryKey: ["job", workspace, projectSlug, runSlug, jobId],
    queryFn: () => getJob(projectSlug, runSlug, jobId, workspace)
  });

  const projectQuery = useQuery({
    queryKey: ["project", workspace, projectSlug],
    queryFn: () => getProject(projectSlug, workspace)
  });

  const runQuery = useQuery({
    queryKey: ["run", workspace, projectSlug, runSlug],
    queryFn: () => getRun(projectSlug, runSlug, workspace)
  });

  const logsQuery = useQuery({
    queryKey: ["job-logs", workspace, projectSlug, runSlug, jobId],
    queryFn: () => getJobLogs(projectSlug, runSlug, jobId, workspace),
    refetchInterval: 1500
  });

  const artefactsQuery = useQuery({
    queryKey: ["job-artefacts", workspace, projectSlug, runSlug, jobId],
    queryFn: () => listArtefacts(projectSlug, runSlug, jobId, workspace)
  });

  const cancelMutation = useMutation({
    mutationFn: () => cancelJob(projectSlug, runSlug, jobId, workspace),
    onSuccess: async () => {
      setCancelError(null);
      await queryClient.invalidateQueries({ queryKey: ["job", workspace, projectSlug, runSlug, jobId] });
      await queryClient.invalidateQueries({ queryKey: ["jobs", workspace, projectSlug, runSlug] });
      await queryClient.invalidateQueries({ queryKey: ["run", workspace, projectSlug, runSlug] });
      await queryClient.invalidateQueries({ queryKey: ["runs", workspace, projectSlug] });
    },
    onError: (error) => {
      setCancelError((error as Error).message ?? "Failed to cancel job");
    }
  });

  const specPayload = useMemo(() => ({ job: jobQuery.data }), [jobQuery.data]);

  if (jobQuery.error) {
    return <ErrorBanner message={(jobQuery.error as Error).message} onRetry={() => jobQuery.refetch()} />;
  }

  const jobName = jobQuery.data?.name ?? "Job";
  const jobSubtitle = jobQuery.data?.image && jobQuery.data?.image !== jobName ? jobQuery.data?.image : "";

  return (
    <div className="space-y-6">
      {cancelError && <ErrorBanner message={cancelError} onRetry={() => setCancelError(null)} />}
      <PageHeader
        breadcrumbs={
          <Breadcrumbs
            items={[
              { label: "Projects", to: `/${workspace}` },
              { label: projectQuery.data?.name ?? projectSlug, to: `/${workspace}/${projectSlug}` },
              { label: runQuery.data?.name ?? runSlug, to: `/${workspace}/${projectSlug}/${runSlug}` },
              { label: jobQuery.data?.name ?? jobId }
            ]}
          />
        }
        title={jobName}
        subtitle={jobSubtitle}
        meta={
          <div className="flex flex-wrap gap-4 text-xs text-ink-500">
            <span>Started: <RelativeTime value={jobQuery.data?.started_at} /></span>
            <span>Finished: <RelativeTime value={jobQuery.data?.finished_at} /></span>
            <span>Duration: {formatDurationMs(jobQuery.data?.duration)}</span>
          </div>
        }
        actions={
          <div className="flex flex-wrap gap-2">
            <StatusPill status={jobQuery.data?.status ?? "PENDING"} />
            <button className="rounded-full border border-ink-200 px-3 py-1 text-sm text-ink-500">Retry</button>
            <button
              type="button"
              onClick={() => cancelMutation.mutate()}
              disabled={cancelMutation.isPending || jobQuery.data?.status === "CANCELLED" || jobQuery.data?.status === "CANCELLING" || jobQuery.data?.status === "SUCCEEDED" || jobQuery.data?.status === "FAILED" || jobQuery.data?.status === "TIMEOUT"}
              className="rounded-full border border-ink-200 px-3 py-1 text-sm text-ink-500 disabled:opacity-60"
            >
              {cancelMutation.isPending ? "Cancelling..." : "Cancel"}
            </button>
          </div>
        }
      />

      <Tabs
        tabs={[
          { id: "logs", label: "Logs" },
          { id: "artefacts", label: "Artefacts" },
          { id: "spec", label: "Spec" }
        ]}
        active={tab}
        onChange={setTab}
      />

      {tab === "logs" && (
        <div className="space-y-4">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <label className="flex items-center gap-2 text-sm text-ink-500">
              <input
                type="checkbox"
                checked={autoScroll}
                onChange={(event) => setAutoScroll(event.target.checked)}
              />
              Auto-scroll
            </label>
            <div className="flex gap-2">
              <button
                type="button"
                onClick={() => logsQuery.refetch()}
                className="rounded-full border border-ink-200 px-3 py-1 text-xs text-ink-500"
              >
                Refresh
              </button>
              <button
                type="button"
                onClick={() => navigator.clipboard.writeText(logsQuery.data ?? "")}
                className="rounded-full border border-ink-200 px-3 py-1 text-xs text-ink-500"
              >
                Copy logs
              </button>
              <button
                type="button"
                onClick={() => {
                  const blob = new Blob([logsQuery.data ?? ""], { type: "text/plain" });
                  const url = URL.createObjectURL(blob);
                  const a = document.createElement("a");
                  a.href = url;
                  a.download = `${jobId}-logs.txt`;
                  a.click();
                  URL.revokeObjectURL(url);
                }}
                className="rounded-full border border-ink-200 px-3 py-1 text-xs text-ink-500"
              >
                Download
              </button>
            </div>
          </div>
          <LogViewer content={logsQuery.data ?? ""} autoScroll={autoScroll} />
        </div>
      )}

      {tab === "artefacts" && (
        <ArtefactList artefacts={artefactsQuery.data ?? []} />
      )}

      {tab === "spec" && (
        <JSONViewer
          data={specPayload}
          onCopy={() => navigator.clipboard.writeText(JSON.stringify(specPayload, null, 2))}
        />
      )}
    </div>
  );
}
