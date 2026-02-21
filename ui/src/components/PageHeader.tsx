import React from "react";

interface PageHeaderProps {
  title: string;
  subtitle?: string;
  meta?: React.ReactNode;
  actions?: React.ReactNode;
}

export function PageHeader({ title, subtitle, meta, actions }: PageHeaderProps) {
  return (
    <div className="flex flex-col gap-6 md:flex-row md:items-center md:justify-between">
      <div>
        <p className="text-xs uppercase tracking-[0.2em] text-ink-400">Switchyard</p>
        <h1 className="mt-2 text-3xl font-semibold text-ink-900">{title}</h1>
        {subtitle && <p className="mt-2 text-sm text-ink-500">{subtitle}</p>}
        {meta && <div className="mt-3 text-sm text-ink-500">{meta}</div>}
      </div>
      {actions && <div className="flex items-center gap-3">{actions}</div>}
    </div>
  );
}
