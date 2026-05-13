import React from "react";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { vi } from "vitest";
import { SettingsPage } from "./SettingsPage";
import { renderWithProviders } from "../test/render";
import * as api from "../api";

let owner = true;
let writeTextMock: ReturnType<typeof vi.fn>;

vi.mock("../auth/AuthProvider", () => ({
  useAuth: () => ({
    isWorkspaceOwner: () => owner
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
    }
  ]),
  listProjectMembers: vi.fn(async () => [
    {
      subject: "user-1",
      email: "alice@example.com",
      display_name: "Alice",
      role: "member",
      added_at: new Date().toISOString()
    }
  ]),
  createWorkspaceInvite: vi.fn(),
  createProjectInvite: vi.fn(),
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
      expect(screen.getByText("Alice")).toBeInTheDocument();
      expect(screen.getByText("Project One (member)")).toBeInTheDocument();
      expect(screen.getByText("Registry Secrets")).toBeInTheDocument();
      expect(screen.getByText("github-actions")).toBeInTheDocument();
    });

    expect(screen.getByText("owner")).toBeInTheDocument();
    expect(screen.getByText("swsa_abc123...")).toBeInTheDocument();
    expect(screen.getAllByText("proj-1").length).toBeGreaterThan(0);
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
    renderWithProviders(<SettingsPage />);

    expect(await screen.findByText(/you need workspace owner access/i)).toBeInTheDocument();
    expect(screen.queryByText("Service Accounts")).not.toBeInTheDocument();
    expect(api.listServiceAccounts).not.toHaveBeenCalled();
  });
});
