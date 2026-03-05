import { useEffect, useState } from "react";
import { Navigate, useLocation, useNavigate } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import { acceptProjectInvite, acceptWorkspaceInvite, listWorkspaces } from "../api";
import { useAuth } from "../auth/AuthProvider";

type AcceptState = "idle" | "submitting" | "error";

export function AcceptInvitePage() {
  const location = useLocation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { isLoading, isAuthenticated } = useAuth();
  const [state, setState] = useState<AcceptState>("idle");
  const [error, setError] = useState<string | null>(null);
  const [attempt, setAttempt] = useState(0);
  const params = new URLSearchParams(location.search);
  const token = params.get("token")?.trim() ?? "";
  useEffect(() => {
    if (!token || !isAuthenticated) {
      return;
    }

    let cancelled = false;
    const accept = async () => {
      setState("submitting");
      setError(null);
      try {
        try {
          await acceptWorkspaceInvite(token);
        } catch (workspaceErr) {
          try {
            await acceptProjectInvite(token);
          } catch {
            throw workspaceErr;
          }
        }

        await queryClient.invalidateQueries({ queryKey: ["auth", "me"] });
        await queryClient.invalidateQueries({ queryKey: ["workspaces"] });
        const workspaces = await listWorkspaces();
        const nextWorkspace = workspaces[0]?.slug;
        if (!nextWorkspace) {
          throw new Error("Invite accepted, but no workspace membership is visible yet. Retry in a moment.");
        }
        if (!cancelled) {
          navigate(`/${nextWorkspace}?invite=accepted`, { replace: true });
        }
      } catch (acceptErr) {
        if (!cancelled) {
          setState("error");
          setError((acceptErr as Error).message);
        }
      }
    };

    accept();
    return () => {
      cancelled = true;
    };
  }, [attempt, isAuthenticated, navigate, queryClient, token]);

  if (!token) {
    return (
      <div className="mx-auto flex min-h-[70vh] max-w-md items-center">
        <div className="card w-full p-8">
          <h1 className="text-xl font-semibold text-ink-900">Invalid invite link</h1>
          <p className="mt-2 text-sm text-ink-500">This link is missing an invite token.</p>
        </div>
      </div>
    );
  }

  if (isLoading) {
    return (
      <div className="mx-auto max-w-3xl py-16 text-sm text-ink-500">
        Checking your session...
      </div>
    );
  }

  if (!isAuthenticated) {
    const next = `${location.pathname}${location.search}`;
    const query = new URLSearchParams({ next }).toString();
    return <Navigate to={`/login?${query}`} replace />;
  }

  return (
    <div className="mx-auto flex min-h-[70vh] max-w-md items-center">
      <div className="card w-full p-8">
        <h1 className="text-xl font-semibold text-ink-900">Accepting invite</h1>
        <p className="mt-2 text-sm text-ink-500">
          {state === "submitting" ? "Finalizing access..." : "Processing invite..."}
        </p>
        {state === "error" && (
          <>
            <p className="mt-3 text-sm text-red-600">{error ?? "Invite acceptance failed."}</p>
            <button
              type="button"
              className="mt-4 rounded-full border border-ink-200 px-4 py-2 text-sm text-ink-700"
              onClick={() => {
                setState("idle");
                setAttempt((v) => v + 1);
              }}
            >
              Retry
            </button>
          </>
        )}
      </div>
    </div>
  );
}
