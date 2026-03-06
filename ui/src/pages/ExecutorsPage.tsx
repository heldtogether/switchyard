import React from "react";
import { PageHeader } from "../components/PageHeader";

export function ExecutorsPage() {
  return (
    <div className="space-y-6">
      <PageHeader
        title="Executors"
        subtitle="Execution backends registered for this workspace."
      />
      <div className="grid gap-4 md:grid-cols-2">
        <div className="card p-5">
          <p className="text-xs uppercase tracking-[0.2em] text-ink-400">Docker</p>
          <h3 className="mt-3 text-lg font-semibold text-ink-900">Docker Executor</h3>
          <p className="mt-2 text-sm text-ink-500">
            Primary executor for production runs. Handles network isolation and NFS outputs.
          </p>
          <div className="mt-4 text-xs text-ink-400">Status: Connected</div>
        </div>
        <div className="card p-5">
          <p className="text-xs uppercase tracking-[0.2em] text-ink-400">Docker</p>
          <h3 className="mt-3 text-lg font-semibold text-ink-900">Local Executor</h3>
          <p className="mt-2 text-sm text-ink-500">
            Development executor for quick iteration and debugging.
          </p>
          <div className="mt-4 text-xs text-ink-400">Status: Available</div>
        </div>
      </div>
    </div>
  );
}
