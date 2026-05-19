import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { PageHeader } from "../components/PageHeader";
import { useParams } from "react-router-dom";
import {
  createServiceAccount,
  createServiceAccountKey,
  createRegistrySecret,
  createProjectInvite,
  createWorkspaceInvite,
  deleteRegistrySecret,
  disableServiceAccount,
  listProjectMembers,
  listProjects,
  listRegistrySecrets,
  listServiceAccounts,
  listWorkspaceMembers,
  revokeServiceAccountKey,
  rotateRegistrySecret
} from "../api";
// import { ErrorBanner } from "../components/ErrorBanner";
import { RelativeTime } from "../components/RelativeTime";
import { useAuth } from "../auth/AuthProvider";
import { Modal } from "../components/Modal";
import type { ServiceAccount } from "../api";
import type { Member, Project } from "../models/types";

type OneTimeKeyState = {
  title: string;
  key: string;
  prefix?: string;
  expiresAt?: string;
} | null;

function toISOFromDateTimeLocal(value: string) {
  if (!value) return "";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "" : date.toISOString();
}

function defaultExpiryInput(days: number) {
  const date = new Date(Date.now() + days * 24 * 60 * 60 * 1000);
  date.setSeconds(0, 0);
  return date.toISOString().slice(0, 16);
}

function isServiceAccountMember(member: Member) {
  return member.subject.startsWith("service_account:");
}

export function SettingsPage() {
  const queryClient = useQueryClient();
  const apiBase = import.meta.env.VITE_API_BASE_URL ?? "http://localhost:8080";
  const { workspace = import.meta.env.VITE_WORKSPACE_SLUG ?? "default" } = useParams();
  const { isProjectOwner, isWorkspaceOwner, memberships, workspaceRole } = useAuth();
  const owner = isWorkspaceOwner(workspace);
  const currentWorkspaceRole = workspaceRole(workspace);
  const hasDirectWorkspaceAccess = currentWorkspaceRole === "owner" || currentWorkspaceRole === "member";
  const mocks = import.meta.env.VITE_USE_MOCKS === "true";
  const aggregateLimit = import.meta.env.VITE_AGGREGATE_LIMIT ?? "5";
  const [workspaceInviteEmail, setWorkspaceInviteEmail] = useState("");
  const [workspaceInviteResult, setWorkspaceInviteResult] = useState<string | null>(null);
  const [workspaceInviteError, setWorkspaceInviteError] = useState<string | null>(null);
  const [workspaceInviting, setWorkspaceInviting] = useState(false);

  const [projectInviteEmail, setProjectInviteEmail] = useState("");
  const [projectInviteProjectSlug, setProjectInviteProjectSlug] = useState("");
  const [projectInviteResult, setProjectInviteResult] = useState<string | null>(null);
  const [projectInviteError, setProjectInviteError] = useState<string | null>(null);
  const [projectInviting, setProjectInviting] = useState(false);
  const [secretHost, setSecretHost] = useState("");
  const [secretUsername, setSecretUsername] = useState("");
  const [secretPassword, setSecretPassword] = useState("");
  const [secretCreateError, setSecretCreateError] = useState<string | null>(null);
  const [rotatingSecretID, setRotatingSecretID] = useState<string | null>(null);
  const [rotatePassword, setRotatePassword] = useState("");
  const [rotateError, setRotateError] = useState<string | null>(null);
  const [serviceAccountModalOpen, setServiceAccountModalOpen] = useState(false);
  const [serviceAccountName, setServiceAccountName] = useState("");
  const [serviceAccountDescription, setServiceAccountDescription] = useState("");
  const [serviceAccountExpiry, setServiceAccountExpiry] = useState(defaultExpiryInput(365));
  const [serviceAccountProjects, setServiceAccountProjects] = useState<string[]>([]);
  const [serviceAccountError, setServiceAccountError] = useState<string | null>(null);
  const [keyModalAccount, setKeyModalAccount] = useState<ServiceAccount | null>(null);
  const [keyName, setKeyName] = useState("");
  const [keyExpiry, setKeyExpiry] = useState(defaultExpiryInput(90));
  const [keyError, setKeyError] = useState<string | null>(null);
  const [oneTimeKey, setOneTimeKey] = useState<OneTimeKeyState>(null);

  const workspaceMembersQuery = useQuery({
    queryKey: ["workspace-members", workspace],
    queryFn: listWorkspaceMembers,
    enabled: owner
  });

  const projectsQuery = useQuery({
    queryKey: ["projects", workspace],
    queryFn: () => listProjects(workspace)
  });

  const registrySecretsQuery = useQuery({
    queryKey: ["registry-secrets", workspace],
    queryFn: listRegistrySecrets,
    enabled: owner
  });

  const serviceAccountsQuery = useQuery({
    queryKey: ["service-accounts", workspace],
    queryFn: listServiceAccounts,
    enabled: owner
  });

  const createRegistrySecretMutation = useMutation({
    mutationFn: createRegistrySecret,
    onSuccess: async () => {
      setSecretHost("");
      setSecretUsername("");
      setSecretPassword("");
      setSecretCreateError(null);
      await queryClient.invalidateQueries({ queryKey: ["registry-secrets", workspace] });
    },
    onError: (error) => setSecretCreateError((error as Error).message)
  });

  const rotateRegistrySecretMutation = useMutation({
    mutationFn: ({ secretID, password }: { secretID: string; password: string }) =>
      rotateRegistrySecret(secretID, { password }),
    onSuccess: async () => {
      setRotatingSecretID(null);
      setRotatePassword("");
      setRotateError(null);
      await queryClient.invalidateQueries({ queryKey: ["registry-secrets", workspace] });
    },
    onError: (error) => setRotateError((error as Error).message)
  });

  const deleteRegistrySecretMutation = useMutation({
    mutationFn: deleteRegistrySecret,
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["registry-secrets", workspace] });
    }
  });

  const createServiceAccountMutation = useMutation({
    mutationFn: createServiceAccount,
    onSuccess: async (response) => {
      setServiceAccountModalOpen(false);
      setServiceAccountName("");
      setServiceAccountDescription("");
      setServiceAccountExpiry(defaultExpiryInput(365));
      setServiceAccountProjects([]);
      setServiceAccountError(null);
      setOneTimeKey({
        title: `${response.service_account.name} key`,
        key: response.key,
        prefix: response.service_account.keys?.[0]?.token_prefix,
        expiresAt: response.service_account.keys?.[0]?.expires_at
      });
      await queryClient.invalidateQueries({ queryKey: ["service-accounts", workspace] });
    },
    onError: (error) => setServiceAccountError((error as Error).message)
  });

  const createServiceAccountKeyMutation = useMutation({
    mutationFn: ({ accountID, name, expiresAt }: { accountID: string; name?: string; expiresAt: string }) =>
      createServiceAccountKey(accountID, { name, expires_at: expiresAt }),
    onSuccess: async (response) => {
      setKeyModalAccount(null);
      setKeyName("");
      setKeyExpiry(defaultExpiryInput(90));
      setKeyError(null);
      setOneTimeKey({
        title: "New service account key",
        key: response.key,
        prefix: response.token_prefix,
        expiresAt: response.expires_at
      });
      await queryClient.invalidateQueries({ queryKey: ["service-accounts", workspace] });
    },
    onError: (error) => setKeyError((error as Error).message)
  });

  const revokeServiceAccountKeyMutation = useMutation({
    mutationFn: ({ accountID, keyID }: { accountID: string; keyID: string }) =>
      revokeServiceAccountKey(accountID, keyID),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["service-accounts", workspace] });
    }
  });

  const disableServiceAccountMutation = useMutation({
    mutationFn: disableServiceAccount,
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["service-accounts", workspace] });
    }
  });

  const accessibleProjects = useMemo(
    () =>
      (projectsQuery.data ?? []).filter((project) =>
        hasDirectWorkspaceAccess ||
        memberships.projects.some((m) => m.workspace_slug === workspace && m.project_slug === project.slug)
      ),
    [hasDirectWorkspaceAccess, memberships.projects, projectsQuery.data, workspace]
  );

  const inviteableProjects = useMemo(
    () => (projectsQuery.data ?? []).filter((project) => isProjectOwner(workspace, project.slug)),
    [isProjectOwner, projectsQuery.data, workspace]
  );

  const selectedProjectInviteSlug = inviteableProjects.some((project) => project.slug === projectInviteProjectSlug)
    ? projectInviteProjectSlug
    : inviteableProjects[0]?.slug ?? "";

  const projectMembersQuery = useQuery({
    queryKey: ["project-members-matrix", workspace, accessibleProjects.map((p) => p.slug).join(",")],
    queryFn: async () => {
      const settled = await Promise.allSettled(
        accessibleProjects.map(async (project) => ({
          project,
          members: await listProjectMembers(project.slug)
        }))
      );
      const entries: { project: Project; members: Member[] }[] = [];
      const bySubject: Record<string, { projectSlug: string; projectName: string; role: string }[]> = {};
      for (const entry of settled) {
        if (entry.status !== "fulfilled") continue;
        entries.push(entry.value);
        for (const member of entry.value.members.filter((m) => !isServiceAccountMember(m))) {
          if (!bySubject[member.subject]) {
            bySubject[member.subject] = [];
          }
          bySubject[member.subject].push({
            projectSlug: entry.value.project.slug,
            projectName: entry.value.project.name,
            role: member.role
          });
        }
      }
      return { bySubject, entries };
    },
    enabled: accessibleProjects.length > 0
  });

  const memberRows = useMemo(() => {
    const matrix = projectMembersQuery.data?.bySubject ?? {};
    return (workspaceMembersQuery.data ?? [])
      .filter((member) => !isServiceAccountMember(member))
      .map((member) => ({
        member,
        projectAccess: matrix[member.subject] ?? []
      }));
  }, [workspaceMembersQuery.data, projectMembersQuery.data]);

  const projectMemberRows = useMemo(
    () =>
      (projectMembersQuery.data?.entries ?? []).map((entry) => ({
        ...entry,
        members: entry.members.filter((member) => !isServiceAccountMember(member))
      })),
    [projectMembersQuery.data]
  );

  async function onInviteWorkspace() {
    const email = workspaceInviteEmail.trim();
    if (!email) {
      setWorkspaceInviteError("Email is required.");
      return;
    }
    setWorkspaceInviting(true);
    setWorkspaceInviteError(null);
    setWorkspaceInviteResult(null);
    try {
      const res = await createWorkspaceInvite(email);
      const link = `${window.location.origin}/accept-invite?token=${encodeURIComponent(res.invite_token)}`;
      setWorkspaceInviteResult(link);
      setWorkspaceInviteEmail("");
    } catch (error) {
      setWorkspaceInviteError((error as Error).message);
    } finally {
      setWorkspaceInviting(false);
    }
  }

  async function onInviteProject() {
    const email = projectInviteEmail.trim();
    const projectSlug = selectedProjectInviteSlug;
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

  function toggleServiceAccountProject(projectSlug: string) {
    setServiceAccountProjects((current) =>
      current.includes(projectSlug)
        ? current.filter((slug) => slug !== projectSlug)
        : [...current, projectSlug]
    );
  }

  async function onCreateServiceAccount() {
    const name = serviceAccountName.trim();
    const expiresAt = toISOFromDateTimeLocal(serviceAccountExpiry);
    if (!name) {
      setServiceAccountError("Name is required.");
      return;
    }
    if (!expiresAt) {
      setServiceAccountError("Expiry is required.");
      return;
    }
    setServiceAccountError(null);
    await createServiceAccountMutation.mutateAsync({
      name,
      description: serviceAccountDescription.trim() || undefined,
      expires_at: expiresAt,
      project_slugs: serviceAccountProjects
    });
  }

  async function onCreateServiceAccountKey() {
    if (!keyModalAccount) return;
    const expiresAt = toISOFromDateTimeLocal(keyExpiry);
    if (!expiresAt) {
      setKeyError("Expiry is required.");
      return;
    }
    setKeyError(null);
    await createServiceAccountKeyMutation.mutateAsync({
      accountID: keyModalAccount.id,
      name: keyName.trim() || undefined,
      expiresAt
    });
  }

  async function copyOneTimeKey() {
    if (!oneTimeKey) return;
    try {
      await navigator.clipboard.writeText(oneTimeKey.key);
    } catch {
      // no-op
    }
  }

  async function onRevokeServiceAccountKey(accountID: string, keyID: string) {
    if (!window.confirm("Revoke this service account key?")) return;
    await revokeServiceAccountKeyMutation.mutateAsync({ accountID, keyID });
  }

  async function onDisableServiceAccount(account: ServiceAccount) {
    if (!window.confirm(`Disable service account "${account.name}"?`)) return;
    await disableServiceAccountMutation.mutateAsync(account.id);
  }

  async function onCreateRegistrySecret() {
    const host = secretHost.trim();
    const username = secretUsername.trim();
    const password = secretPassword.trim();
    if (!host || !username || !password) {
      setSecretCreateError("Host, username, and password are required.");
      return;
    }
    await createRegistrySecretMutation.mutateAsync({ host, username, password });
  }

  async function onRotateSecret(secretID: string) {
    const password = rotatePassword.trim();
    if (!password) {
      setRotateError("New password is required.");
      return;
    }
    await rotateRegistrySecretMutation.mutateAsync({ secretID, password });
  }

  async function onDeactivateSecret(secretID: string) {
    await deleteRegistrySecretMutation.mutateAsync(secretID);
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title="Settings"
        subtitle="Environment configuration and access management for this workspace."
      />

      <div className="card p-6">
        <div className="mb-3 text-xs uppercase tracking-[0.2em] text-ink-400">Access Management</div>
        <div className="space-y-6">
          {owner ? (
            <div>
              <div className="mb-3 flex flex-wrap items-start justify-between gap-4">
                <div>
                  <div className="text-sm font-semibold text-ink-900">Workspace Access</div>
                  <p className="mt-1 text-xs text-ink-500">
                    Workspace members can create projects and use workspace-level resources.
                  </p>
                </div>
                <div className="w-full rounded-lg border border-ink-100 p-4 md:w-[30rem]">
                  <div className="text-sm font-semibold text-ink-900">Invite To Workspace</div>
                  <p className="mt-1 text-xs text-ink-500">Creates a member-level workspace invite.</p>
                  {workspaceInviteError && <p className="mt-3 text-sm text-red-600">{workspaceInviteError}</p>}
                  <div className="mt-3 flex gap-2">
                    <input
                      className="min-w-0 flex-1 rounded-lg border border-ink-200 px-3 py-2 text-sm"
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
                        <input
                          className="min-w-0 flex-1 rounded-lg border border-ink-200 px-3 py-2 text-xs"
                          readOnly
                          value={workspaceInviteResult}
                        />
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
                </div>
              </div>
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
          ) : (
            <div>
              <div className="text-sm font-semibold text-ink-900">Workspace Access</div>
              <p className="mt-1 text-sm text-ink-600">
                You need workspace owner access to manage workspace members and workspace invites.
              </p>
            </div>
          )}

          <div>
            <div className="mb-3 flex flex-wrap items-start justify-between gap-4">
              <div>
                <div className="text-sm font-semibold text-ink-900">Project Access</div>
                <p className="mt-1 text-xs text-ink-500">
                  Project membership is separate from workspace membership.
                </p>
              </div>
              <div className="w-full rounded-lg border border-ink-100 p-4 md:w-[30rem]">
                <div className="text-sm font-semibold text-ink-900">Invite To Project</div>
                <p className="mt-1 text-xs text-ink-500">Creates a member-level project invite.</p>
                {projectInviteError && <p className="mt-3 text-sm text-red-600">{projectInviteError}</p>}
                {inviteableProjects.length === 0 ? (
                  <p className="mt-3 text-sm text-ink-500">No projects are available for you to invite members.</p>
                ) : (
                  <div className="mt-3 space-y-2">
                    <select
                      className="w-full rounded-lg border border-ink-200 px-3 py-2 text-sm"
                      value={selectedProjectInviteSlug}
                      onChange={(event) => setProjectInviteProjectSlug(event.target.value)}
                    >
                      {inviteableProjects.map((project) => (
                        <option key={project.slug} value={project.slug}>
                          {project.name}
                        </option>
                      ))}
                    </select>
                    <div className="flex gap-2">
                      <input
                        className="min-w-0 flex-1 rounded-lg border border-ink-200 px-3 py-2 text-sm"
                        placeholder="person@example.com"
                        value={projectInviteEmail}
                        onChange={(event) => setProjectInviteEmail(event.target.value)}
                      />
                      <button
                        type="button"
                        disabled={projectInviting}
                        onClick={onInviteProject}
                        className="rounded-full bg-ink-900 px-4 py-2 text-sm font-semibold text-white disabled:opacity-60"
                      >
                        Invite
                      </button>
                    </div>
                  </div>
                )}
                {projectInviteResult && (
                  <div className="mt-3 space-y-2">
                    <div className="text-xs text-ink-500">Invite link</div>
                    <div className="flex gap-2">
                      <input
                        className="min-w-0 flex-1 rounded-lg border border-ink-200 px-3 py-2 text-xs"
                        readOnly
                        value={projectInviteResult}
                      />
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

            <div className="space-y-4">
              {projectMemberRows.map((entry) => (
                <div key={entry.project.slug} className="overflow-x-auto rounded-lg border border-ink-100">
                  <div className="border-b border-ink-100 px-4 py-3">
                    <div className="text-sm font-semibold text-ink-900">{entry.project.name}</div>
                    <div className="font-mono text-xs text-ink-400">{entry.project.slug}</div>
                  </div>
                  <table className="min-w-full text-sm">
                    <thead className="bg-ink-50 text-left text-xs uppercase tracking-[0.15em] text-ink-500">
                      <tr>
                        <th className="px-4 py-3">Member</th>
                        <th className="px-4 py-3">Project Role</th>
                        <th className="px-4 py-3">Added</th>
                      </tr>
                    </thead>
                    <tbody>
                      {entry.members.map((member) => (
                        <tr key={`${entry.project.slug}-${member.subject}`} className="border-t border-ink-100">
                          <td className="px-4 py-3">
                            <div className="font-medium text-ink-900">
                              {member.display_name ?? member.email ?? member.subject}
                            </div>
                            <div className="text-xs text-ink-500">{member.email ?? member.subject}</div>
                          </td>
                          <td className="px-4 py-3 capitalize text-ink-700">{member.role}</td>
                          <td className="px-4 py-3 text-ink-600">
                            <RelativeTime value={member.added_at} />
                          </td>
                        </tr>
                      ))}
                      {entry.members.length === 0 && (
                        <tr>
                          <td className="px-4 py-4 text-ink-500" colSpan={3}>
                            No project members found.
                          </td>
                        </tr>
                      )}
                    </tbody>
                  </table>
                </div>
              ))}
              {projectMemberRows.length === 0 && (
                <p className="rounded-lg border border-ink-100 px-4 py-4 text-sm text-ink-500">
                  No project membership is visible for this workspace.
                </p>
              )}
            </div>
          </div>

          {owner && (
            <div>
              <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
                <div>
                  <div className="text-sm font-semibold text-ink-900">Service Accounts</div>
                  <p className="mt-1 text-xs text-ink-500">
                    Machine credentials for CI/CD jobs. Keys are only shown once.
                  </p>
                </div>
                <button
                  type="button"
                  onClick={() => {
                    setServiceAccountModalOpen(true);
                    setServiceAccountError(null);
                  }}
                  className="rounded-full bg-ink-900 px-4 py-2 text-sm font-semibold text-white"
                >
                  New Service Account
                </button>
              </div>

              {serviceAccountsQuery.error && (
                <p className="mb-3 text-sm text-red-600">{(serviceAccountsQuery.error as Error).message}</p>
              )}

              <div className="overflow-x-auto rounded-lg border border-ink-100">
                <table className="min-w-full text-sm">
                  <thead className="bg-ink-50 text-left text-xs uppercase tracking-[0.15em] text-ink-500">
                    <tr>
                      <th className="px-4 py-3">Account</th>
                      <th className="px-4 py-3">Scope</th>
                      <th className="px-4 py-3">Status</th>
                      <th className="px-4 py-3">Keys</th>
                      <th className="px-4 py-3">Actions</th>
                    </tr>
                  </thead>
                  <tbody>
                    {(serviceAccountsQuery.data ?? []).map((account) => {
                      const active = !account.disabled_at;
                      const keys = account.keys ?? [];
                      return (
                        <tr key={account.id} className="border-t border-ink-100 align-top">
                          <td className="px-4 py-3">
                            <div className="font-medium text-ink-900">{account.name}</div>
                            {account.description && (
                              <div className="text-xs text-ink-500">{account.description}</div>
                            )}
                            <div className="mt-1 font-mono text-xs text-ink-400">{account.subject}</div>
                            <div className="mt-1 text-xs text-ink-500">
                              Created by {account.created_by} <RelativeTime value={account.created_at} />
                            </div>
                          </td>
                          <td className="px-4 py-3">
                            {(account.project_slugs?.length ?? 0) === 0 ? (
                              <span className="text-xs text-ink-500">All workspace projects</span>
                            ) : (
                              <div className="flex flex-wrap gap-1">
                                {(account.project_slugs ?? []).map((slug) => (
                                  <span key={`${account.id}-${slug}`} className="rounded-full bg-ink-100 px-2 py-0.5 text-xs text-ink-600">
                                    {slug}
                                  </span>
                                ))}
                              </div>
                            )}
                          </td>
                          <td className="px-4 py-3">
                            <span className={active ? "text-emerald-600" : "text-ink-500"}>
                              {active ? "Active" : "Disabled"}
                            </span>
                            {account.disabled_at && (
                              <div className="mt-1 text-xs text-ink-500">
                                <RelativeTime value={account.disabled_at} />
                              </div>
                            )}
                          </td>
                          <td className="px-4 py-3">
                            {keys.length === 0 ? (
                              <span className="text-xs text-ink-400">No keys</span>
                            ) : (
                              <div className="space-y-3">
                                {keys.map((key) => {
                                  return (
                                    <div key={key.id} className="space-y-1">
                                      <div className="font-mono text-xs text-ink-900">{key.token_prefix}...</div>
                                      <div className="text-xs text-ink-500">
                                        {key.name ?? "Unnamed key"} | expires <RelativeTime value={key.expires_at} />
                                      </div>
                                      <div className="text-xs text-ink-500">
                                        Last used {key.last_used_at ? <RelativeTime value={key.last_used_at} /> : "never"}
                                      </div>
                                      {key.revoked_at ? (
                                        <div className="text-xs text-ink-400">
                                          Revoked <RelativeTime value={key.revoked_at} />
                                        </div>
                                      ) : (
                                        <button
                                          type="button"
                                          className="rounded-full border border-red-200 px-3 py-1 text-xs text-red-600"
                                          onClick={() => onRevokeServiceAccountKey(account.id, key.id)}
                                          disabled={!active || revokeServiceAccountKeyMutation.isPending}
                                        >
                                          Revoke Key
                                        </button>
                                      )}
                                    </div>
                                  );
                                })}
                              </div>
                            )}
                          </td>
                          <td className="px-4 py-3">
                            {active ? (
                              <div className="flex flex-wrap gap-2">
                                <button
                                  type="button"
                                  className="rounded-full border border-ink-200 px-3 py-1 text-xs text-ink-700"
                                  onClick={() => {
                                    setKeyModalAccount(account);
                                    setKeyName("");
                                    setKeyExpiry(defaultExpiryInput(90));
                                    setKeyError(null);
                                  }}
                                >
                                  New Key
                                </button>
                                <button
                                  type="button"
                                  className="rounded-full border border-red-200 px-3 py-1 text-xs text-red-600"
                                  onClick={() => onDisableServiceAccount(account)}
                                  disabled={disableServiceAccountMutation.isPending}
                                >
                                  Disable
                                </button>
                              </div>
                            ) : (
                              <span className="text-xs text-ink-400">No actions</span>
                            )}
                          </td>
                        </tr>
                      );
                    })}
                    {(serviceAccountsQuery.data?.length ?? 0) === 0 && (
                      <tr>
                        <td className="px-4 py-4 text-ink-500" colSpan={5}>
                          No service accounts configured.
                        </td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </div>
            </div>
          )}
        </div>
      </div>

      {owner && (
        <div className="card p-6">
        <div className="mb-3 text-xs uppercase tracking-[0.2em] text-ink-400">Secret Management</div>
              <div className="text-sm font-semibold text-ink-900">Registry Secrets</div>
              <p className="mt-1 text-xs text-ink-500">
                Workspace-scoped credentials for private container registries. Secret values are never shown.
              </p>

              <div className="mt-4 overflow-x-auto rounded-lg border border-ink-100">
                <table className="min-w-full text-sm">
                  <thead className="bg-ink-50 text-left text-xs uppercase tracking-[0.15em] text-ink-500">
                    <tr>
                      <th className="px-4 py-3">Registry</th>
                      <th className="px-4 py-3">Status</th>
                      <th className="px-4 py-3">Created</th>
                      <th className="px-4 py-3">Deactivated</th>
                      <th className="px-4 py-3">Actions</th>
                    </tr>
                  </thead>
                  <tbody>
                    {(registrySecretsQuery.data ?? []).map((secret) => (
                      <tr key={secret.id} className="border-t border-ink-100">
                        <td className="px-4 py-3">
                          <div className="font-medium text-ink-900">{secret.host}</div>
                          <div className="text-xs text-ink-500">{secret.username}</div>
                        </td>
                        <td className="px-4 py-3">
                          <span className={secret.active ? "text-emerald-600" : "text-ink-500"}>
                            {secret.active ? "Active" : "Inactive"}
                          </span>
                        </td>
                        <td className="px-4 py-3 text-ink-600">
                          <RelativeTime value={secret.created_at} />
                          <div className="text-xs text-ink-500">{secret.created_by}</div>
                        </td>
                        <td className="px-4 py-3 text-ink-600">
                          {secret.deactivated_at ? (
                            <>
                              <RelativeTime value={secret.deactivated_at} />
                              <div className="text-xs text-ink-500">{secret.deactivated_by ?? "Unknown"}</div>
                            </>
                          ) : (
                            <span className="text-xs text-ink-400">-</span>
                          )}
                        </td>
                        <td className="px-4 py-3">
                          {!secret.active ? (
                            <span className="text-xs text-ink-400">No actions</span>
                          ) : (
                            <div className="space-y-2">
                              {rotatingSecretID === secret.id ? (
                                <div className="flex gap-2">
                                  <input
                                    type="password"
                                    className="rounded-lg border border-ink-200 px-2 py-1 text-xs"
                                    placeholder="New password"
                                    value={rotatePassword}
                                    onChange={(event) => setRotatePassword(event.target.value)}
                                  />
                                  <button
                                    type="button"
                                    className="rounded-full border border-ink-200 px-3 py-1 text-xs text-ink-700"
                                    onClick={() => onRotateSecret(secret.id)}
                                  >
                                    Save
                                  </button>
                                  <button
                                    type="button"
                                    className="rounded-full border border-ink-200 px-3 py-1 text-xs text-ink-500"
                                    onClick={() => {
                                      setRotatingSecretID(null);
                                      setRotatePassword("");
                                      setRotateError(null);
                                    }}
                                  >
                                    Cancel
                                  </button>
                                </div>
                              ) : (
                                <div className="flex gap-2">
                                  <button
                                    type="button"
                                    className="rounded-full border border-ink-200 px-3 py-1 text-xs text-ink-700"
                                    onClick={() => {
                                      setRotatingSecretID(secret.id);
                                      setRotatePassword("");
                                      setRotateError(null);
                                    }}
                                  >
                                    Rotate
                                  </button>
                                  <button
                                    type="button"
                                    className="rounded-full border border-red-200 px-3 py-1 text-xs text-red-600"
                                    onClick={() => onDeactivateSecret(secret.id)}
                                  >
                                    Deactivate
                                  </button>
                                </div>
                              )}
                              {rotateError && rotatingSecretID === secret.id && (
                                <p className="text-xs text-red-600">{rotateError}</p>
                              )}
                            </div>
                          )}
                        </td>
                      </tr>
                    ))}
                    {(registrySecretsQuery.data?.length ?? 0) === 0 && (
                      <tr>
                        <td className="px-4 py-4 text-sm text-ink-500" colSpan={5}>
                          No registry secrets configured.
                        </td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </div>

              <div className="mt-3 grid gap-2 md:grid-cols-4">
                <input
                  className="rounded-lg border border-ink-200 px-3 py-2 text-sm"
                  placeholder="registry.example.com"
                  value={secretHost}
                  onChange={(event) => setSecretHost(event.target.value)}
                />
                <input
                  className="rounded-lg border border-ink-200 px-3 py-2 text-sm"
                  placeholder="username"
                  value={secretUsername}
                  onChange={(event) => setSecretUsername(event.target.value)}
                />
                <input
                  className="rounded-lg border border-ink-200 px-3 py-2 text-sm"
                  placeholder="password"
                  type="password"
                  value={secretPassword}
                  onChange={(event) => setSecretPassword(event.target.value)}
                />
                <button
                  type="button"
                  onClick={onCreateRegistrySecret}
                  disabled={createRegistrySecretMutation.isPending}
                  className="rounded-full bg-ink-900 px-4 py-2 text-sm font-semibold text-white disabled:opacity-60"
                >
                  Add Secret
                </button>
              </div>
              {secretCreateError && <p className="mt-2 text-sm text-red-600">{secretCreateError}</p>}
              
            </div>
      )}

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

      <Modal
        open={serviceAccountModalOpen}
        title="New Service Account"
        description="Create a machine credential for CI/CD job submission."
        onClose={() => setServiceAccountModalOpen(false)}
        footer={
          <div className="flex justify-end gap-2">
            <button type="button" onClick={() => setServiceAccountModalOpen(false)} className="text-sm text-ink-500">
              Close
            </button>
            <button
              type="button"
              onClick={onCreateServiceAccount}
              disabled={createServiceAccountMutation.isPending}
              className="rounded-full bg-ink-900 px-4 py-2 text-sm font-semibold text-white disabled:opacity-60"
            >
              Create
            </button>
          </div>
        }
      >
        <div className="space-y-4 text-sm text-ink-600">
          {serviceAccountError && <p className="text-sm text-red-600">{serviceAccountError}</p>}
          <div>
            <label htmlFor="service-account-name" className="text-xs uppercase tracking-[0.2em] text-ink-400">Name</label>
            <input
              id="service-account-name"
              className="mt-2 w-full rounded-lg border border-ink-200 px-3 py-2"
              placeholder="github-actions"
              value={serviceAccountName}
              onChange={(event) => setServiceAccountName(event.target.value)}
            />
          </div>
          <div>
            <label htmlFor="service-account-description" className="text-xs uppercase tracking-[0.2em] text-ink-400">Description</label>
            <textarea
              id="service-account-description"
              className="mt-2 w-full rounded-lg border border-ink-200 px-3 py-2"
              placeholder="Optional description"
              rows={3}
              value={serviceAccountDescription}
              onChange={(event) => setServiceAccountDescription(event.target.value)}
            />
          </div>
          <div>
            <label htmlFor="service-account-expiry" className="text-xs uppercase tracking-[0.2em] text-ink-400">Key Expiry</label>
            <input
              id="service-account-expiry"
              className="mt-2 w-full rounded-lg border border-ink-200 px-3 py-2"
              type="datetime-local"
              value={serviceAccountExpiry}
              onChange={(event) => setServiceAccountExpiry(event.target.value)}
            />
          </div>
          <div>
            <div className="text-xs uppercase tracking-[0.2em] text-ink-400">Project Scope</div>
            <div className="mt-2 space-y-2 rounded-lg border border-ink-100 p-3">
              {(projectsQuery.data ?? []).map((project) => (
                <label key={project.slug} className="flex items-center gap-2 text-sm text-ink-700">
                  <input
                    type="checkbox"
                    checked={serviceAccountProjects.includes(project.slug)}
                    onChange={() => toggleServiceAccountProject(project.slug)}
                  />
                  <span>{project.name}</span>
                  <span className="font-mono text-xs text-ink-400">{project.slug}</span>
                </label>
              ))}
              {(projectsQuery.data?.length ?? 0) === 0 && (
                <p className="text-xs text-ink-500">No projects are available.</p>
              )}
              <p className="text-xs text-ink-500">
                Leave all unchecked to allow all workspace projects.
              </p>
            </div>
          </div>
        </div>
      </Modal>

      <Modal
        open={!!keyModalAccount}
        title="New Service Account Key"
        description={keyModalAccount ? `Generate a replacement key for ${keyModalAccount.name}.` : undefined}
        onClose={() => setKeyModalAccount(null)}
        footer={
          <div className="flex justify-end gap-2">
            <button type="button" onClick={() => setKeyModalAccount(null)} className="text-sm text-ink-500">
              Close
            </button>
            <button
              type="button"
              onClick={onCreateServiceAccountKey}
              disabled={createServiceAccountKeyMutation.isPending}
              className="rounded-full bg-ink-900 px-4 py-2 text-sm font-semibold text-white disabled:opacity-60"
            >
              Create Key
            </button>
          </div>
        }
      >
        <div className="space-y-4 text-sm text-ink-600">
          {keyError && <p className="text-sm text-red-600">{keyError}</p>}
          <div>
            <label htmlFor="service-account-key-name" className="text-xs uppercase tracking-[0.2em] text-ink-400">Key Name</label>
            <input
              id="service-account-key-name"
              className="mt-2 w-full rounded-lg border border-ink-200 px-3 py-2"
              placeholder="github-actions-rotation"
              value={keyName}
              onChange={(event) => setKeyName(event.target.value)}
            />
          </div>
          <div>
            <label htmlFor="service-account-key-expiry" className="text-xs uppercase tracking-[0.2em] text-ink-400">Expiry</label>
            <input
              id="service-account-key-expiry"
              className="mt-2 w-full rounded-lg border border-ink-200 px-3 py-2"
              type="datetime-local"
              value={keyExpiry}
              onChange={(event) => setKeyExpiry(event.target.value)}
            />
          </div>
        </div>
      </Modal>

      <Modal
        open={!!oneTimeKey}
        title={oneTimeKey?.title ?? "Service Account Key"}
        description="Store this key now. Switchyard cannot show it again."
        onClose={() => setOneTimeKey(null)}
        footer={
          <div className="flex justify-end gap-2">
            <button type="button" onClick={() => setOneTimeKey(null)} className="text-sm text-ink-500">
              Done
            </button>
            <button
              type="button"
              onClick={copyOneTimeKey}
              className="rounded-full bg-ink-900 px-4 py-2 text-sm font-semibold text-white"
            >
              Copy Key
            </button>
          </div>
        }
      >
        {oneTimeKey && (
          <div className="space-y-3 text-sm text-ink-600">
            {oneTimeKey.prefix && (
              <div>
                <div className="text-xs uppercase tracking-[0.2em] text-ink-400">Prefix</div>
                <div className="mt-1 font-mono text-ink-900">{oneTimeKey.prefix}</div>
              </div>
            )}
            {oneTimeKey.expiresAt && (
              <div>
                <div className="text-xs uppercase tracking-[0.2em] text-ink-400">Expires</div>
                <div className="mt-1 text-ink-900">
                  <RelativeTime value={oneTimeKey.expiresAt} />
                </div>
              </div>
            )}
            <textarea
              className="h-28 w-full rounded-lg border border-ink-200 px-3 py-2 font-mono text-xs text-ink-900"
              readOnly
              value={oneTimeKey.key}
            />
          </div>
        )}
      </Modal>
    </div>
  );
}
