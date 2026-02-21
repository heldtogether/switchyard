import React, { useMemo, useState } from "react";
import { useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { getJob, getJobLogs, listArtefacts } from "../api";
import { PageHeader } from "../components/PageHeader";
import { StatusPill } from "../components/StatusPill";
import { Tabs } from "../components/Tabs";
import { LogViewer } from "../components/LogViewer";
import { ArtefactList } from "../components/ArtefactList";
import { JSONViewer } from "../components/JSONViewer";
import { RelativeTime } from "../components/RelativeTime";
import { formatDurationMs } from "../utils/format";
import { ErrorBanner } from "../components/ErrorBanner";

export function JobDetailPage() {
  const { projectSlug = "", runSlug = "", jobId = "" } = useParams();
  const [tab, setTab] = useState("logs");
  const [autoScroll, setAutoScroll] = useState(true);

  const jobQuery = useQuery({
    queryKey: ["job", projectSlug, runSlug, jobId],
    queryFn: () => getJob(projectSlug, runSlug, jobId)
  });

  const logsQuery = useQuery({
    queryKey: ["job-logs", projectSlug, runSlug, jobId],
    queryFn: () => getJobLogs(projectSlug, runSlug, jobId),
    refetchInterval: 1500
  });

  const artefactsQuery = useQuery({
    queryKey: ["job-artefacts", projectSlug, runSlug, jobId],
    queryFn: () => listArtefacts(projectSlug, runSlug, jobId)
  });

  const specPayload = useMemo(() => ({ job: jobQuery.data }), [jobQuery.data]);

  if (jobQuery.error) {
    return <ErrorBanner message={(jobQuery.error as Error).message} onRetry={() => jobQuery.refetch()} />;
  }

  const jobName = jobQuery.data?.name ?? "Job";
  const jobSubtitle = jobQuery.data?.image && jobQuery.data?.image !== jobName ? jobQuery.data?.image : "";

  return (
    <div className="space-y-6">
      <PageHeader
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
            <button className="rounded-full border border-ink-200 px-3 py-1 text-sm text-ink-500">Cancel</button>
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
