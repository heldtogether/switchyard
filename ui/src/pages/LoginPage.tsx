import React from "react";
import { useLocation } from "react-router-dom";
import { useAuth } from "../auth/AuthProvider";

export function LoginPage() {
  const { loginUrl } = useAuth();
  const location = useLocation();
  const params = new URLSearchParams(location.search);
  const next = params.get("next");
  const href = (() => {
    if (!next) {
      return loginUrl;
    }
    const query = new URLSearchParams({ next }).toString();
    const separator = loginUrl.includes("?") ? "&" : "?";
    return `${loginUrl}${separator}${query}`;
  })();

  return (
    <div className="mx-auto flex min-h-[70vh] max-w-md items-center">
      <div className="card w-full p-8">
        <p className="text-xs uppercase tracking-[0.2em] text-ink-400">Authentication</p>
        <h1 className="mt-3 text-2xl font-semibold text-ink-900">Sign in to Switchyard</h1>
        <p className="mt-2 text-sm text-ink-500">
          Use your organization SSO account to access projects, runs, and jobs.
        </p>
        <a
          href={href}
          className="mt-6 inline-flex w-full items-center justify-center rounded-full bg-ink-900 px-4 py-2 text-sm font-semibold text-white"
        >
          Continue with SSO
        </a>
      </div>
    </div>
  );
}
