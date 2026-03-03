import { useEffect, useRef } from "react";

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
    <div
      ref={ref}
      className="card h-[420px] w-full min-w-0 overflow-x-auto overflow-y-auto bg-ink-900 p-4 font-mono text-xs text-ink-100"
    >
      {content ? (
        <pre className="min-w-full whitespace-pre w-max">{content}</pre>
      ) : (
        <div className="text-ink-400">No logs yet.</div>
      )}
    </div>
  );
}
