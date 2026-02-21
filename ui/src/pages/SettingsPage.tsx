import React from "react";
import { PageHeader } from "../components/PageHeader";

export function SettingsPage() {
  const apiBase = import.meta.env.VITE_API_BASE_URL ?? "http://localhost:8080";
  const workspace = import.meta.env.VITE_WORKSPACE_SLUG ?? "default";
  const mocks = import.meta.env.VITE_USE_MOCKS === "true";
  const aggregateLimit = import.meta.env.VITE_AGGREGATE_LIMIT ?? "5";

  return (
    <div className="space-y-6">
      <PageHeader
        title="Settings"
        subtitle="Environment-level configuration for this UI."
      />
      <div className="card p-6">
        <div className="space-y-4 text-sm text-ink-600">
          <div>
            <div className="text-xs uppercase tracking-[0.2em] text-ink-400">API Base URL</div>
            <div className="mt-1 font-mono text-ink-900">{apiBase}</div>
          </div>
          <div>
            <div className="text-xs uppercase tracking-[0.2em] text-ink-400">Workspace</div>
            <div className="mt-1 font-mono text-ink-900">{workspace}</div>
          </div>
          <div>
            <div className="text-xs uppercase tracking-[0.2em] text-ink-400">Mock Mode</div>
            <div className="mt-1 font-mono text-ink-900">{mocks ? "enabled" : "disabled"}</div>
          </div>
          <div>
            <div className="text-xs uppercase tracking-[0.2em] text-ink-400">Aggregate Limit</div>
            <div className="mt-1 font-mono text-ink-900">{aggregateLimit}</div>
          </div>
        </div>
      </div>
    </div>
  );
}
