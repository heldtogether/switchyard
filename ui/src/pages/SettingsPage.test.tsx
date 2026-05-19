import React from "react";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { vi } from "vitest";
import { SettingsPage } from "./SettingsPage";
import { renderWithProviders } from "../test/render";
import * as api from "../api";

let owner = true;
let workspaceMember = false;
let projectMemberships: { workspace_slug: string; project_slug: string; role: "owner" | "member" }[] = [];
let writeTextMock: ReturnType<typeof vi.fn>;

vi.mock("../auth/AuthProvider", () => ({
  useAuth: () => ({
    memberships: {
      workspaces: owner ? [{ slug: "default", role: "owner" }] : workspaceMember ? [{ slug: "default", role: "member" }] : [],
      projects: projectMemberships
    },
    isWorkspaceOwner: () => owner,
    workspaceRole: () => (owner ? "owner" : workspaceMember ? "member" : null),
    isProjectOwner: (_workspaceSlug: string, projectSlug: string) =>
      owner || projectMemberships.some((m) => m.project_slug === projectSlug && m.role === "owner")
  })
}));

vi.mock("../api", () => ({
  listWorkspaceMembers: vi.fn(async () => [
    {
      subject: "user-1",
      email: "alice@example.com",
      display_name: "Alice",
      role: "owner",
      added_at: new Date().toISOString()
    },
    {
      subject: "service_account:listed-member",
      email: null,
      display_name: "Listed service account",
      role: "member",
      added_at: new Date().toISOString()
    }
  ]),
  listProjects: vi.fn(async () => [
    {
      id: "p1",
      slug: "proj-1",
      name: "Project One",
      description: "",
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString()
    },
    {
      id: "p2",
      slug: "proj-2",
      name: "Project Two",
      description: "",
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString()
    }
  ]),
  listProjectMembers: vi.fn(async (projectSlug: string) =>
    projectSlug === "proj-2"
      ? [
          {
            subject: "user-2",
            email: "bob@example.com",
            display_name: "Bob",
            role: "owner",
            added_at: new Date().toISOString()
          },
          {
            subject: "service_account:project-member",
            email: null,
            display_name: "Project service account",
            role: "member",
            added_at: new Date().toISOString()
          }
        ]
      : [
          {
            subject: "user-1",
            email: "alice@example.com",
            display_name: "Alice",
            role: "member",
            added_at: new Date().toISOString()
          },
          {
            subject: "service_account:project-member",
            email: null,
            display_name: "Project service account",
            role: "member",
            added_at: new Date().toISOString()
          }
        ]
  ),
  createWorkspaceInvite: vi.fn(async () => ({
    invite_id: "invite-workspace",
    invite_url: "http://localhost:8080/accept-invite?token=workspace-token",
    invite_token: "workspace-token",
    expires_at: "2026-01-02T00:00:00Z"
  })),
  createProjectInvite: vi.fn(async () => ({
    invite_id: "invite-project",
    invite_url: "http://localhost:8080/accept-invite?token=project-token",
    invite_token: "project-token",
    expires_at: "2026-01-02T00:00:00Z"
  })),
  listRegistrySecrets: vi.fn(async () => []),
  createRegistrySecret: vi.fn(),
  rotateRegistrySecret: vi.fn(),
  deleteRegistrySecret: vi.fn(),
  listServiceAccounts: vi.fn(async () => [
    {
      id: "sa-1",
      workspace_id: "default",
      principal_id: "principal-1",
      subject: "service_account:sa-1",
      name: "github-actions",
      description: "CI runner",
      disabled_at: null,
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-01T00:00:00Z",
      created_by: "alice@example.com",
      project_slugs: ["proj-1"],
      keys: [
        {
          id: "key-1",
          name: "initial",
          token_prefix: "swsa_abc123",
          expires_at: "2027-01-01T00:00:00Z",
          last_used_at: null,
          revoked_at: null,
          created_at: "2026-01-01T00:00:00Z",
          created_by: "alice@example.com"
        }
      ]
    }
  ]),
  createServiceAccount: vi.fn(async () => ({
    service_account: {
      id: "sa-2",
      workspace_id: "default",
      principal_id: "principal-2",
      subject: "service_account:sa-2",
      name: "deploy-bot",
      description: null,
      disabled_at: null,
      created_at: "2026-01-02T00:00:00Z",
      updated_at: "2026-01-02T00:00:00Z",
      created_by: "alice@example.com",
      project_slugs: ["proj-1"],
      keys: [
        {
          id: "key-2",
          name: null,
          token_prefix: "swsa_new",
          expires_at: "2027-01-01T00:00:00Z",
          created_at: "2026-01-02T00:00:00Z",
          created_by: "alice@example.com"
        }
      ]
    },
    key: "swsa_new_secret"
  })),
  createServiceAccountKey: vi.fn(async () => ({
    key_id: "key-3",
    key: "swsa_rotated_secret",
    token_prefix: "swsa_rot",
    expires_at: "2027-02-01T00:00:00Z"
  })),
  revokeServiceAccountKey: vi.fn(async () => undefined),
  disableServiceAccount: vi.fn(async () => undefined)
}));

vi.mock("react-router-dom", async () => {
  const actual = await vi.importActual<typeof import("react-router-dom")>("react-router-dom");
  return {
    ...actual,
    useParams: () => ({ workspace: "default" })
  };
});

describe("SettingsPage", () => {
  beforeEach(() => {
    owner = true;
    workspaceMember = false;
    projectMemberships = [];
    vi.clearAllMocks();
    vi.spyOn(window, "confirm").mockReturnValue(true);
    writeTextMock = vi.fn();
    Object.defineProperty(navigator, "clipboard", {
      value: { writeText: writeTextMock },
      configurable: true
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("renders workspace members with project access chips", async () => {
    renderWithProviders(<SettingsPage />);

    await waitFor(() => {
      expect(screen.getAllByText("Alice").length).toBeGreaterThan(0);
      expect(screen.getByText("Project One (member)")).toBeInTheDocument();
      expect(screen.getByText("Registry Secrets")).toBeInTheDocument();
      expect(screen.getByText("github-actions")).toBeInTheDocument();
    });

    expect(screen.getAllByText("owner").length).toBeGreaterThan(0);
    expect(screen.getByText("swsa_abc123...")).toBeInTheDocument();
    expect(screen.getAllByText("proj-1").length).toBeGreaterThan(0);
    expect(screen.queryByText("Listed service account")).not.toBeInTheDocument();
    expect(screen.queryByText("Project service account")).not.toBeInTheDocument();
  });

  it("creates workspace and project invites for workspace owners", async () => {
    const user = userEvent.setup();
    renderWithProviders(<SettingsPage />);

    const workspaceInvite = await screen.findByText("Invite To Workspace");
    const workspaceCard = workspaceInvite.closest("div.rounded-lg") as HTMLElement;
    await user.type(within(workspaceCard).getByPlaceholderText("person@example.com"), "workspace@example.com");
    await user.click(within(workspaceCard).getByRole("button", { name: "Invite" }));

    await waitFor(() => {
      expect(api.createWorkspaceInvite).toHaveBeenCalledWith("workspace@example.com");
    });
    expect(await within(workspaceCard).findByDisplayValue(/workspace-token/)).toBeInTheDocument();

    const projectInvite = screen.getByText("Invite To Project");
    const projectCard = projectInvite.closest("div.rounded-lg") as HTMLElement;
    await user.selectOptions(within(projectCard).getByRole("combobox"), "proj-2");
    await user.type(within(projectCard).getByPlaceholderText("person@example.com"), "project@example.com");
    await user.click(within(projectCard).getByRole("button", { name: "Invite" }));

    await waitFor(() => {
      expect(api.createProjectInvite).toHaveBeenCalledWith("proj-2", "project@example.com");
    });
    expect(await within(projectCard).findByDisplayValue(/project-token/)).toBeInTheDocument();
  });

  it("lets direct project owners invite to their own projects without workspace owner access", async () => {
    const user = userEvent.setup();
    owner = false;
    projectMemberships = [{ workspace_slug: "default", project_slug: "proj-2", role: "owner" }];
    renderWithProviders(<SettingsPage />);

    expect(await screen.findByText(/you need workspace owner access/i)).toBeInTheDocument();
    expect(screen.queryByText("Service Accounts")).not.toBeInTheDocument();
    expect(screen.queryByText("Registry Secrets")).not.toBeInTheDocument();
    expect(api.listWorkspaceMembers).not.toHaveBeenCalled();
    expect(api.listServiceAccounts).not.toHaveBeenCalled();

    const projectCard = screen.getByText("Invite To Project").closest("div.rounded-lg") as HTMLElement;
    const selector = await within(projectCard).findByRole("combobox");
    expect(within(selector).queryByRole("option", { name: "Project One" })).not.toBeInTheDocument();
    expect(within(selector).getByRole("option", { name: "Project Two" })).toBeInTheDocument();

    await user.type(within(projectCard).getByPlaceholderText("person@example.com"), "project-owner@example.com");
    await user.click(within(projectCard).getByRole("button", { name: "Invite" }));

    await waitFor(() => {
      expect(api.createProjectInvite).toHaveBeenCalledWith("proj-2", "project-owner@example.com");
    });
  });

  it("shows all project memberships to direct workspace members without project invite controls", async () => {
    owner = false;
    workspaceMember = true;
    renderWithProviders(<SettingsPage />);

    expect(await screen.findByText("Project One")).toBeInTheDocument();
    expect(screen.getByText("Project Two")).toBeInTheDocument();
    expect(screen.getByText("Alice")).toBeInTheDocument();
    expect(screen.getByText("Bob")).toBeInTheDocument();
    expect(screen.getByText(/no projects are available for you to invite members/i)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Invite" })).not.toBeInTheDocument();
  });

  it("hides invite controls for direct project members without owner access", async () => {
    owner = false;
    projectMemberships = [{ workspace_slug: "default", project_slug: "proj-1", role: "member" }];
    renderWithProviders(<SettingsPage />);

    expect(await screen.findByText("Invite To Project")).toBeInTheDocument();
    expect(screen.getByText(/no projects are available for you to invite members/i)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Invite" })).not.toBeInTheDocument();
    expect(api.listWorkspaceMembers).not.toHaveBeenCalled();
    expect(api.listServiceAccounts).not.toHaveBeenCalled();
  });

  it("creates a service account and reveals the one-time key", async () => {
    const user = userEvent.setup();
    Object.defineProperty(navigator, "clipboard", {
      value: { writeText: writeTextMock },
      configurable: true
    });
    renderWithProviders(<SettingsPage />);

    await user.click(await screen.findByRole("button", { name: /new service account/i }));
    await user.type(screen.getByLabelText("Name"), "deploy-bot");
    await user.type(screen.getByLabelText("Description"), "Deploy pipeline");
    await user.click(screen.getByLabelText(/project one/i));
    await user.click(screen.getByRole("button", { name: "Create" }));

    await waitFor(() => {
      expect(api.createServiceAccount).toHaveBeenCalled();
    });
    expect(vi.mocked(api.createServiceAccount).mock.calls[0][0]).toMatchObject({
      name: "deploy-bot",
      description: "Deploy pipeline",
      project_slugs: ["proj-1"]
    });
    expect(await screen.findByDisplayValue("swsa_new_secret")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /copy key/i }));
    expect(writeTextMock).toHaveBeenCalledWith("swsa_new_secret");
  });

  it("creates a replacement key for an account", async () => {
    const user = userEvent.setup();
    renderWithProviders(<SettingsPage />);

    await user.click(await screen.findByRole("button", { name: /new key/i }));
    await user.type(screen.getByLabelText("Key Name"), "rotation");
    await user.click(screen.getByRole("button", { name: /create key/i }));

    await waitFor(() => {
      expect(api.createServiceAccountKey).toHaveBeenCalled();
    });
    expect(vi.mocked(api.createServiceAccountKey).mock.calls[0][0]).toBe("sa-1");
    expect(vi.mocked(api.createServiceAccountKey).mock.calls[0][1]).toMatchObject({ name: "rotation" });
    expect(await screen.findByDisplayValue("swsa_rotated_secret")).toBeInTheDocument();
  });

  it("revokes service account keys and disables accounts", async () => {
    const user = userEvent.setup();
    renderWithProviders(<SettingsPage />);

    await user.click(await screen.findByRole("button", { name: /revoke key/i }));
    await waitFor(() => {
      expect(api.revokeServiceAccountKey).toHaveBeenCalledWith("sa-1", "key-1");
    });

    await user.click(screen.getByRole("button", { name: "Disable" }));
    await waitFor(() => {
      expect(api.disableServiceAccount).toHaveBeenCalled();
    });
    expect(vi.mocked(api.disableServiceAccount).mock.calls[0][0]).toBe("sa-1");
  });

  it("hides service account controls for non-owner users", async () => {
    owner = false;
    projectMemberships = [{ workspace_slug: "default", project_slug: "proj-1", role: "member" }];
    renderWithProviders(<SettingsPage />);

    expect(await screen.findByText(/you need workspace owner access/i)).toBeInTheDocument();
    expect(screen.queryByText("Service Accounts")).not.toBeInTheDocument();
    expect(screen.queryByText("Registry Secrets")).not.toBeInTheDocument();
    expect(api.listWorkspaceMembers).not.toHaveBeenCalled();
    expect(api.listServiceAccounts).not.toHaveBeenCalled();
  });
});
