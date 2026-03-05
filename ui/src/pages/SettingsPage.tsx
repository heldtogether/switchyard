import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { PageHeader } from "../components/PageHeader";
import { useParams } from "react-router-dom";
import {
  createProjectInvite,
  // createWorkspaceInvite,
  listProjectMembers,
  listProjects,
  listWorkspaceMembers
} from "../api";
// import { ErrorBanner } from "../components/ErrorBanner";
import { RelativeTime } from "../components/RelativeTime";
import { useAuth } from "../auth/AuthProvider";

export function SettingsPage() {
  const apiBase = import.meta.env.VITE_API_BASE_URL ?? "http://localhost:8080";
  const { workspace = import.meta.env.VITE_WORKSPACE_SLUG ?? "default" } = useParams();
  const { isWorkspaceOwner } = useAuth();
  const owner = isWorkspaceOwner(workspace);
  const mocks = import.meta.env.VITE_USE_MOCKS === "true";
  const aggregateLimit = import.meta.env.VITE_AGGREGATE_LIMIT ?? "5";
  // const [workspaceInviteEmail, setWorkspaceInviteEmail] = useState("");
  // const [workspaceInviteResult, setWorkspaceInviteResult] = useState<string | null>(null);
  // const [workspaceInviteError, setWorkspaceInviteError] = useState<string | null>(null);
  // const [workspaceInviting, setWorkspaceInviting] = useState(false);

  const [projectInviteEmail, setProjectInviteEmail] = useState("");
  const [projectInviteProjectSlug, setProjectInviteProjectSlug] = useState("");
  const [projectInviteResult, setProjectInviteResult] = useState<string | null>(null);
  const [projectInviteError, setProjectInviteError] = useState<string | null>(null);
  const [projectInviting, setProjectInviting] = useState(false);

  const workspaceMembersQuery = useQuery({
    queryKey: ["workspace-members", workspace],
    queryFn: listWorkspaceMembers
  });

  const projectsQuery = useQuery({
    queryKey: ["projects", workspace],
    queryFn: listProjects
  });

  const projectMembersQuery = useQuery({
    queryKey: ["project-members-matrix", workspace, projectsQuery.data?.map((p) => p.slug).join(",")],
    queryFn: async () => {
      const projects = projectsQuery.data ?? [];
      const settled = await Promise.all(
        projects.map(async (project) => ({
          project,
          members: await listProjectMembers(project.slug)
        }))
      );
      const bySubject: Record<string, { projectSlug: string; projectName: string; role: string }[]> = {};
      for (const entry of settled) {
        for (const member of entry.members) {
          if (!bySubject[member.subject]) {
            bySubject[member.subject] = [];
          }
          bySubject[member.subject].push({
            projectSlug: entry.project.slug,
            projectName: entry.project.name,
            role: member.role
          });
        }
      }
      return bySubject;
    },
    enabled: !!projectsQuery.data && projectsQuery.data.length > 0
  });

  const memberRows = useMemo(() => {
    const matrix = projectMembersQuery.data ?? {};
    return (workspaceMembersQuery.data ?? []).map((member) => ({
      member,
      projectAccess: matrix[member.subject] ?? []
    }));
  }, [workspaceMembersQuery.data, projectMembersQuery.data]);

  // async function onInviteWorkspace() {
  //   const email = workspaceInviteEmail.trim();
  //   if (!email) {
  //     setWorkspaceInviteError("Email is required.");
  //     return;
  //   }
  //   setWorkspaceInviting(true);
  //   setWorkspaceInviteError(null);
  //   setWorkspaceInviteResult(null);
  //   try {
  //     const res = await createWorkspaceInvite(email);
  //     const link = `${window.location.origin}/accept-invite?token=${encodeURIComponent(res.invite_token)}`;
  //     setWorkspaceInviteResult(link);
  //     setWorkspaceInviteEmail("");
  //   } catch (error) {
  //     setWorkspaceInviteError((error as Error).message);
  //   } finally {
  //     setWorkspaceInviting(false);
  //   }
  // }

  async function onInviteProject() {
    const email = projectInviteEmail.trim();
    const projectSlug = projectInviteProjectSlug || projectsQuery.data?.[0]?.slug;
    if (!projectSlug) {
      setProjectInviteError("Select a project.");
      return;
    }
    if (!email) {
      setProjectInviteError("Email is required.");
      return;
    }
    setProjectInviting(true);
    setProjectInviteError(null);
    setProjectInviteResult(null);
    try {
      const res = await createProjectInvite(projectSlug, email);
      const link = `${window.location.origin}/accept-invite?token=${encodeURIComponent(res.invite_token)}`;
      setProjectInviteResult(link);
      setProjectInviteEmail("");
    } catch (error) {
      setProjectInviteError((error as Error).message);
    } finally {
      setProjectInviting(false);
    }
  }

  async function copyInviteLink(link: string) {
    try {
      await navigator.clipboard.writeText(link);
    } catch {
      // no-op
    }
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title="Settings"
        subtitle="Environment configuration and access management for this workspace."
      />

      <div className="card p-6">
        <div className="mb-3 text-xs uppercase tracking-[0.2em] text-ink-400">Access Management</div>
        {!owner ? (
          <p className="text-sm text-ink-600">
            You need workspace owner access to manage invites and members.
          </p>
        ) : (
          <div className="space-y-6">

            <div>
              <div className="mb-3 text-sm font-semibold text-ink-900">Workspace Members</div>
              <div className="overflow-x-auto rounded-lg border border-ink-100">
                <table className="min-w-full text-sm">
                  <thead className="bg-ink-50 text-left text-xs uppercase tracking-[0.15em] text-ink-500">
                    <tr>
                      <th className="px-4 py-3">Member</th>
                      <th className="px-4 py-3">Workspace Role</th>
                      <th className="px-4 py-3">Project Access</th>
                      <th className="px-4 py-3">Added</th>
                    </tr>
                  </thead>
                  <tbody>
                    {memberRows.map(({ member, projectAccess }) => (
                      <tr key={member.subject} className="border-t border-ink-100">
                        <td className="px-4 py-3">
                          <div className="font-medium text-ink-900">{member.display_name ?? member.email ?? member.subject}</div>
                          <div className="text-xs text-ink-500">{member.email ?? member.subject}</div>
                        </td>
                        <td className="px-4 py-3 capitalize text-ink-700">{member.role}</td>
                        <td className="px-4 py-3">
                          {projectAccess.length === 0 ? (
                            <span className="text-xs text-ink-400">No project access</span>
                          ) : (
                            <div className="flex flex-wrap gap-1">
                              {projectAccess.map((entry) => (
                                <span key={`${member.subject}-${entry.projectSlug}`} className="rounded-full bg-ink-100 px-2 py-0.5 text-xs text-ink-600">
                                  {entry.projectName} ({entry.role})
                                </span>
                              ))}
                            </div>
                          )}
                        </td>
                        <td className="px-4 py-3 text-ink-600">
                          <RelativeTime value={member.added_at} />
                        </td>
                      </tr>
                    ))}
                    {memberRows.length === 0 && (
                      <tr>
                        <td className="px-4 py-4 text-ink-500" colSpan={4}>
                          No workspace members found.
                        </td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </div>
            </div>

            <div className="grid gap-4 md:grid-cols-2">
              {/* <div className="rounded-lg border border-ink-100 p-4">
                <div className="text-sm font-semibold text-ink-900">Invite To Workspace</div>
                <p className="mt-1 text-xs text-ink-500">Creates a member-level workspace invite.</p>
                {workspaceInviteError && <p className="mt-3 text-sm text-red-600">{workspaceInviteError}</p>}
                <div className="mt-3 flex gap-2">
                  <input
                    className="flex-1 rounded-lg border border-ink-200 px-3 py-2 text-sm"
                    placeholder="person@example.com"
                    value={workspaceInviteEmail}
                    onChange={(event) => setWorkspaceInviteEmail(event.target.value)}
                  />
                  <button
                    type="button"
                    disabled={workspaceInviting}
                    onClick={onInviteWorkspace}
                    className="rounded-full bg-ink-900 px-4 py-2 text-sm font-semibold text-white disabled:opacity-60"
                  >
                    Invite
                  </button>
                </div>
                {workspaceInviteResult && (
                  <div className="mt-3 space-y-2">
                    <div className="text-xs text-ink-500">Invite link</div>
                    <div className="flex gap-2">
                      <input className="flex-1 rounded-lg border border-ink-200 px-3 py-2 text-xs" readOnly value={workspaceInviteResult} />
                      <button
                        type="button"
                        onClick={() => copyInviteLink(workspaceInviteResult)}
                        className="rounded-full border border-ink-200 px-3 py-2 text-xs text-ink-700"
                      >
                        Copy
                      </button>
                    </div>
                  </div>
                )}
              </div> */}

              <div className="rounded-lg border border-ink-100 p-4 col-span-2">
                <div className="text-sm font-semibold text-ink-900">Invite To Project</div>
                <p className="mt-1 text-xs text-ink-500">Creates a member-level project invite.</p>
                {projectInviteError && <p className="mt-3 text-sm text-red-600">{projectInviteError}</p>}
                <div className="mt-3 space-y-2">
                  <select
                    className="w-full rounded-lg border border-ink-200 px-3 py-2 text-sm"
                    value={projectInviteProjectSlug || projectsQuery.data?.[0]?.slug || ""}
                    onChange={(event) => setProjectInviteProjectSlug(event.target.value)}
                  >
                    {(projectsQuery.data ?? []).map((project) => (
                      <option key={project.slug} value={project.slug}>
                        {project.name}
                      </option>
                    ))}
                  </select>
                  <div className="flex gap-2">
                    <input
                      className="flex-1 rounded-lg border border-ink-200 px-3 py-2 text-sm"
                      placeholder="person@example.com"
                      value={projectInviteEmail}
                      onChange={(event) => setProjectInviteEmail(event.target.value)}
                    />
                    <button
                      type="button"
                      disabled={projectInviting || (projectsQuery.data?.length ?? 0) === 0}
                      onClick={onInviteProject}
                      className="rounded-full bg-ink-900 px-4 py-2 text-sm font-semibold text-white disabled:opacity-60"
                    >
                      Invite
                    </button>
                  </div>
                </div>
                {projectInviteResult && (
                  <div className="mt-3 space-y-2">
                    <div className="text-xs text-ink-500">Invite link</div>
                    <div className="flex gap-2">
                      <input className="flex-1 rounded-lg border border-ink-200 px-3 py-2 text-xs" readOnly value={projectInviteResult} />
                      <button
                        type="button"
                        onClick={() => copyInviteLink(projectInviteResult)}
                        className="rounded-full border border-ink-200 px-3 py-2 text-xs text-ink-700"
                      >
                        Copy
                      </button>
                    </div>
                  </div>
                )}
              </div>
            </div>

          </div>
        )}
      </div>

      <div className="card p-6">
        <div className="space-y-4 text-sm text-ink-600">
          <div className="text-xs uppercase tracking-[0.2em] text-ink-400">Environment</div>
          <div>
            <div className="text-xs uppercase tracking-[0.2em] text-ink-400">API Base URL</div>
            <div className="mt-1 font-mono text-ink-900">{apiBase}</div>
          </div>
          <div>
            <div className="text-xs uppercase tracking-[0.2em] text-ink-400">Workspace</div>
            <div className="mt-1 font-mono text-ink-900">{workspace}</div>
          </div>
          <div>
            <div className="text-xs uppercase tracking-[0.2em] text-ink-400">Mock Mode</div>
            <div className="mt-1 font-mono text-ink-900">{mocks ? "enabled" : "disabled"}</div>
          </div>
          <div>
            <div className="text-xs uppercase tracking-[0.2em] text-ink-400">Aggregate Limit</div>
            <div className="mt-1 font-mono text-ink-900">{aggregateLimit}</div>
          </div>
        </div>
      </div>
    </div>
  );
}
