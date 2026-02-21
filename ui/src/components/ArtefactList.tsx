import React from "react";
import { Artefact } from "../models/types";
import { formatBytes } from "../utils/format";

interface ArtefactListProps {
  artefacts: Artefact[];
}

export function ArtefactList({ artefacts }: ArtefactListProps) {
  if (artefacts.length === 0) {
    return <div className="card p-6 text-sm text-ink-500">No artefacts yet.</div>;
  }

  return (
    <div className="space-y-3">
      {artefacts.map((art) => (
        <div key={art.id} className="card p-4">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-semibold text-ink-900">{art.path}</p>
              <p className="text-xs text-ink-500">
                {formatBytes(art.size_bytes)} · {art.content_type}
              </p>
            </div>
            {art.download_url && (
              <a
                href={art.download_url}
                className="rounded-full border border-ink-200 px-3 py-1 text-xs font-semibold text-ink-600 hover:border-ink-400"
              >
                Download
              </a>
            )}
          </div>
          {art.content_type.includes("json") && (
            <pre className="mt-3 max-h-48 overflow-auto rounded-lg bg-ink-900 p-3 text-xs text-ink-100">
              {"{\n  \"preview\": \"JSON artefact\"\n}"}
            </pre>
          )}
          {art.content_type.startsWith("image") && art.download_url && (
            <img src={art.download_url} alt={art.path} className="mt-3 max-h-48 rounded-lg" />
          )}
        </div>
      ))}
    </div>
  );
}
