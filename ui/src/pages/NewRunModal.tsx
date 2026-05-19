import React, { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useParams } from "react-router-dom";
import { Modal } from "../components/Modal";
import { slugify } from "../utils/slug";
import { createJob, createRun, getAllocationCapacity, listRegistrySecrets } from "../api";

interface JobDraft {
  name: string;
  image: string;
  command: string;
  env: string;
  outputs: string;
  cpu: string;
  memory: string;
  timeout: string;
  gpu: number;
  registrySecretId?: string;
}

interface NewRunModalProps {
  open: boolean;
  projectSlug: string;
  onClose: () => void;
  onSuccess: (runSlug: string) => void;
}

export function NewRunModal({ open, projectSlug, onClose, onSuccess }: NewRunModalProps) {
  const { workspace = "" } = useParams();
  const [name, setName] = useState("");
  const [slug, setSlug] = useState("");
  const [description, setDescription] = useState("");
  const [tags, setTags] = useState("");
  const [jobs, setJobs] = useState<JobDraft[]>([
    {
      name: "",
      image: "",
      command: "",
      env: "",
      outputs: "/outputs",
      cpu: "",
      memory: "",
      timeout: "",
      gpu: 0
    }
  ]);
  const [slugTouched, setSlugTouched] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const registrySecretsQuery = useQuery({
    queryKey: ["registry-secrets", workspace],
    queryFn: listRegistrySecrets,
    enabled: open
  });

  const registrySecrets = registrySecretsQuery.data ?? [];

  const allocationCapacityQuery = useQuery({
    queryKey: ["allocation-capacity"],
    queryFn: getAllocationCapacity,
    enabled: open
  });

  const maxGPUPerNode = allocationCapacityQuery.data?.max_gpu_per_node ?? 0;

  const derivedSlug = useMemo(() => (slugTouched ? slug : slugify(name)), [slugTouched, slug, name]);

  function updateJob(index: number, patch: Partial<JobDraft>) {
    setJobs((prev) => prev.map((job, i) => (i === index ? { ...job, ...patch } : job)));
  }

  function parseEnv(input: string) {
    return input
      .split("\n")
      .map((line) => line.trim())
      .filter((line) => line && !line.startsWith("#"))
      .reduce<Record<string, string>>((acc, line) => {
        const [key, ...rest] = line.split("=");
        if (!key) return acc;
        acc[key.trim()] = rest.join("=").trim();
        return acc;
      }, {});
  }

  function parseCommand(input: string) {
    const trimmed = input.trim();
    if (!trimmed) return [] as string[];
    if (trimmed.startsWith("[")) {
      try {
        const parsed = JSON.parse(trimmed);
        if (Array.isArray(parsed)) return parsed.map(String);
      } catch {
        return trimmed.split(/\s+/);
      }
    }
    return trimmed.split(/\s+/);
  }

  async function handleSubmit() {
    setError(null);
    if (!name.trim()) {
      setError("Run name is required.");
      return;
    }
    if (!derivedSlug) {
      setError("Run slug is required.");
      return;
    }
    if (jobs.some((job) => !job.image)) {
      setError("Each job needs an image.");
      return;
    }

    setSubmitting(true);
    try {
      await createRun(
        projectSlug,
        {
          slug: derivedSlug,
          name: name.trim(),
          description: description.trim() || undefined,
          metadata: {
            tags_user: tags.split(",").map((tag) => tag.trim()).filter(Boolean)
          }
        },
        workspace
      );

      const failures: { job: JobDraft; error: string }[] = [];
      for (const job of jobs) {
        const resources = {
          ...(job.cpu ? { cpu: job.cpu } : {}),
          ...(job.memory ? { memory: job.memory } : {}),
          ...(job.gpu > 0 ? { gpu: job.gpu } : {})
        };
        const payload: any = {
          name: job.name || undefined,
          image: job.image,
          command: parseCommand(job.command),
          env: parseEnv(job.env),
          outputs: job.outputs.split(",").map((item) => item.trim()).filter(Boolean),
          resources: Object.keys(resources).length > 0 ? resources : undefined,
          timeout_seconds: job.timeout ? Number(job.timeout) : undefined
        };
        if (job.registrySecretId) {
          payload.registry_secret_id = job.registrySecretId;
        }
        try {
          await createJob(projectSlug, derivedSlug, payload, workspace);
        } catch (jobErr) {
          failures.push({ job, error: (jobErr as Error).message });
        }
      }

      if (failures.length > 0) {
        setError(`Run created, but ${failures.length} job(s) failed to submit.`);
        onSuccess(derivedSlug);
        return;
      }

      onSuccess(derivedSlug);
      onClose();
    } catch (err) {
      setError((err as Error).message ?? "Failed to create run.");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Modal
      open={open}
      title="New Run + Jobs"
      description="Create a run and queue its jobs in one action."
      onClose={onClose}
      footer={
        <div className="flex items-center justify-between">
          <div className="text-xs text-ink-400">Creates run, then submits jobs.</div>
          <div className="flex gap-2">
            <button type="button" onClick={onClose} className="text-sm text-ink-500">
              Cancel
            </button>
            <button
              type="button"
              disabled={submitting}
              onClick={handleSubmit}
              className="rounded-full bg-ink-900 px-4 py-2 text-sm font-semibold text-white disabled:opacity-60"
            >
              {submitting ? "Submitting..." : "Create Run"}
            </button>
          </div>
        </div>
      }
    >
      <div className="space-y-6 text-sm text-ink-600">
        {error && <div className="rounded-lg border border-danger/30 bg-danger/10 px-3 py-2 text-danger">{error}</div>}
        <div className="grid gap-4 md:grid-cols-2">
          <div>
            <label className="text-xs uppercase tracking-[0.2em] text-ink-400">Run Name</label>
            <input
              className="mt-2 w-full rounded-lg border border-ink-200 px-3 py-2"
              value={name}
              onChange={(event) => {
                setName(event.target.value);
                if (!slugTouched) setSlug(slugify(event.target.value));
              }}
              placeholder="threshold-sweep"
            />
          </div>
          <div>
            <label className="text-xs uppercase tracking-[0.2em] text-ink-400">Run Slug</label>
            <input
              className="mt-2 w-full rounded-lg border border-ink-200 px-3 py-2"
              value={derivedSlug}
              onChange={(event) => {
                setSlugTouched(true);
                setSlug(event.target.value);
              }}
              placeholder="threshold-sweep"
            />
          </div>
        </div>
        <div>
          <label className="text-xs uppercase tracking-[0.2em] text-ink-400">Description</label>
          <textarea
            className="mt-2 w-full rounded-lg border border-ink-200 px-3 py-2"
            rows={2}
            value={description}
            onChange={(event) => setDescription(event.target.value)}
            placeholder="Optional context"
          />
        </div>
        <div>
          <label className="text-xs uppercase tracking-[0.2em] text-ink-400">Tags</label>
          <input
            className="mt-2 w-full rounded-lg border border-ink-200 px-3 py-2"
            value={tags}
            onChange={(event) => setTags(event.target.value)}
            placeholder="nightly, metrics"
          />
        </div>

        <div className="space-y-4">
          {jobs.map((job, index) => (
            <div key={index} className="rounded-xl border border-ink-200 p-4">
              <div className="flex items-center justify-between">
                <p className="text-xs uppercase tracking-[0.2em] text-ink-400">Job {index + 1}</p>
                {jobs.length > 1 && (
                  <button
                    type="button"
                    className="text-xs text-ink-400"
                    onClick={() => setJobs((prev) => prev.filter((_, i) => i !== index))}
                  >
                    Remove
                  </button>
                )}
              </div>
              <div className="mt-3 space-y-3">
                <input
                  className="w-full rounded-lg border border-ink-200 px-3 py-2"
                  value={job.name}
                  onChange={(event) => updateJob(index, { name: event.target.value })}
                  placeholder="Job name"
                />
                <input
                  className="w-full rounded-lg border border-ink-200 px-3 py-2"
                  value={job.image}
                  onChange={(event) => updateJob(index, { image: event.target.value })}
                  placeholder="Image (required)"
                />
                <div>
                  <label className="text-xs uppercase tracking-[0.2em] text-ink-400">Registry Secret</label>
                  <select
                    className="mt-2 w-full rounded-lg border border-ink-200 px-3 py-2 text-sm"
                    value={job.registrySecretId ?? ""}
                    onChange={(event) => updateJob(index, { registrySecretId: event.target.value || undefined })}
                    disabled={registrySecretsQuery.isLoading || registrySecrets.length === 0}
                  >
                    <option value="">None</option>
                    {registrySecrets.map((secret) => (
                      <option key={secret.id} value={secret.id}>
                        {secret.host} / {secret.username}
                      </option>
                    ))}
                  </select>
                  {registrySecretsQuery.isError && (
                    <div className="mt-1 text-xs text-danger">Failed to load registry secrets.</div>
                  )}
                  {!registrySecretsQuery.isLoading && registrySecrets.length === 0 && (
                    <div className="mt-1 text-xs text-ink-400">
                      No registry secrets available. Add one via the API to pull private images.
                    </div>
                  )}
                </div>
              </div>
              <div className="mt-3">
                <input
                  className="w-full rounded-lg border border-ink-200 px-3 py-2"
                  value={job.command}
                  onChange={(event) => updateJob(index, { command: event.target.value })}
                  placeholder="Command (e.g. /app/run --fast)"
                />
              </div>
              <div className="mt-3">
                <textarea
                  className="w-full rounded-lg border border-ink-200 px-3 py-2 font-mono text-xs"
                  rows={3}
                  value={job.env}
                  onChange={(event) => updateJob(index, { env: event.target.value })}
                  placeholder="ENV_KEY=value\nANOTHER=123"
                />
              </div>
              <div className="mt-3 grid gap-3 md:grid-cols-3">
                <input
                  className="rounded-lg border border-ink-200 px-3 py-2"
                  value={job.outputs}
                  onChange={(event) => updateJob(index, { outputs: event.target.value })}
                  placeholder="Outputs (/outputs)"
                />
                <input
                  className="rounded-lg border border-ink-200 px-3 py-2"
                  value={job.cpu}
                  onChange={(event) => updateJob(index, { cpu: event.target.value })}
                  placeholder="CPU (0.5)"
                />
                <input
                  className="rounded-lg border border-ink-200 px-3 py-2"
                  value={job.memory}
                  onChange={(event) => updateJob(index, { memory: event.target.value })}
                  placeholder="Memory (512m)"
                />
              </div>
              <div className="mt-3">
                <label className="text-xs uppercase tracking-[0.2em] text-ink-400">GPUs</label>
                <select
                  className="mt-2 w-full rounded-lg border border-ink-200 px-3 py-2 text-sm"
                  value={job.gpu}
                  onChange={(event) => updateJob(index, { gpu: Number(event.target.value) })}
                  disabled={allocationCapacityQuery.isLoading || allocationCapacityQuery.isError || maxGPUPerNode === 0}
                >
                  {Array.from({ length: maxGPUPerNode + 1 }, (_, value) => (
                    <option key={value} value={value}>
                      {value}
                    </option>
                  ))}
                </select>
                {allocationCapacityQuery.isError && (
                  <div className="mt-1 text-xs text-danger">Failed to load GPU capacity.</div>
                )}
                {!allocationCapacityQuery.isLoading && !allocationCapacityQuery.isError && maxGPUPerNode === 0 && (
                  <div className="mt-1 text-xs text-ink-400">No GPU-capable nodes registered.</div>
                )}
              </div>
              <div className="mt-3">
                <input
                  className="w-full rounded-lg border border-ink-200 px-3 py-2"
                  value={job.timeout}
                  onChange={(event) => updateJob(index, { timeout: event.target.value })}
                  placeholder="Timeout seconds (optional)"
                />
              </div>
            </div>
          ))}
          <button
            type="button"
            onClick={() =>
              setJobs((prev) => [
                ...prev,
                {
                  name: "",
                  image: "",
                  command: "",
                  env: "",
                  outputs: "/outputs",
                  cpu: "",
                  memory: "",
                  timeout: "",
                  gpu: 0
                }
              ])
            }
            className="rounded-full border border-ink-200 px-3 py-1 text-xs text-ink-500"
          >
            Add job
          </button>
        </div>
      </div>
    </Modal>
  );
}
