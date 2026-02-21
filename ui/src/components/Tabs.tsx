import React from "react";
import clsx from "clsx";

interface TabsProps {
  tabs: { id: string; label: string }[];
  active: string;
  onChange: (id: string) => void;
}

export function Tabs({ tabs, active, onChange }: TabsProps) {
  return (
    <div className="flex flex-wrap gap-2 border-b border-ink-100">
      {tabs.map((tab) => (
        <button
          key={tab.id}
          type="button"
          onClick={() => onChange(tab.id)}
          className={clsx(
            "rounded-t-lg px-4 py-2 text-sm font-medium",
            active === tab.id
              ? "bg-white text-ink-900 shadow-card"
              : "text-ink-500 hover:text-ink-900"
          )}
        >
          {tab.label}
        </button>
      ))}
    </div>
  );
}
