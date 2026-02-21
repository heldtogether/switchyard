import React from "react";

export function ErrorBanner({ message, onRetry }: { message: string; onRetry?: () => void }) {
  return (
    <div className="rounded-xl border border-danger/30 bg-danger/10 px-4 py-3 text-sm text-danger">
      <div className="flex items-center justify-between">
        <span>{message}</span>
        {onRetry && (
          <button type="button" onClick={onRetry} className="text-xs font-semibold underline">
            Retry
          </button>
        )}
      </div>
    </div>
  );
}
