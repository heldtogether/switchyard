import React from "react";
import { PageHeader } from "../components/PageHeader";

export function PlaceholderPage({ title }: { title: string }) {
  return (
    <div className="space-y-6">
      <PageHeader title={title} subtitle="Coming soon" />
      <div className="card p-6 text-sm text-ink-500">
        This section is stubbed for the first release.
      </div>
    </div>
  );
}
