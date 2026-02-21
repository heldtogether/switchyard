import React from "react";

interface EmptyStateProps {
  title: string;
  description?: string;
  action?: React.ReactNode;
}

export function EmptyState({ title, description, action }: EmptyStateProps) {
  return (
    <div className="card flex flex-col items-start gap-3 p-6">
      <h3 className="text-lg font-semibold text-ink-900">{title}</h3>
      {description && <p className="text-sm text-ink-500">{description}</p>}
      {action}
    </div>
  );
}
