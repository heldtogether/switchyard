import React, { useEffect, useRef } from "react";

interface LogViewerProps {
  content: string;
  autoScroll: boolean;
}

export function LogViewer({ content, autoScroll }: LogViewerProps) {
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!autoScroll || !ref.current) return;
    ref.current.scrollTop = ref.current.scrollHeight;
  }, [content, autoScroll]);

  return (
    <div ref={ref} className="card h-[420px] overflow-auto bg-ink-900 p-4 font-mono text-xs text-ink-100">
      {content ? (
        <pre className="whitespace-pre-wrap">{content}</pre>
      ) : (
        <div className="text-ink-400">No logs yet.</div>
      )}
    </div>
  );
}
