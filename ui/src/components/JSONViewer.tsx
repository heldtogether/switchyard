import React from "react";

interface JSONViewerProps {
  data: unknown;
  onCopy?: () => void;
}

export function JSONViewer({ data, onCopy }: JSONViewerProps) {
  const formatted = JSON.stringify(data, null, 2);
  return (
    <div className="card p-4">
      <div className="flex items-center justify-between">
        <p className="text-xs uppercase tracking-[0.2em] text-ink-400">Spec</p>
        {onCopy && (
          <button
            type="button"
            onClick={onCopy}
            className="rounded-full border border-ink-200 px-3 py-1 text-xs font-semibold text-ink-600 hover:border-ink-400"
          >
            Copy
          </button>
        )}
      </div>
      <pre className="mt-3 max-h-[420px] overflow-auto rounded-lg bg-ink-900 p-4 text-xs text-ink-100">
        {formatted}
      </pre>
    </div>
  );
}
