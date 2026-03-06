import React from "react";
import { screen, waitFor } from "@testing-library/react";
import { vi } from "vitest";
import { SettingsPage } from "./SettingsPage";
import { renderWithProviders } from "../test/render";

vi.mock("../auth/AuthProvider", () => ({
  useAuth: () => ({
    isWorkspaceOwner: () => true
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
  deleteRegistrySecret: vi.fn()
}));

vi.mock("react-router-dom", async () => {
  const actual = await vi.importActual<typeof import("react-router-dom")>("react-router-dom");
  return {
    ...actual,
    useParams: () => ({ workspace: "default" })
  };
});

describe("SettingsPage", () => {
  it("renders workspace members with project access chips", async () => {
    renderWithProviders(<SettingsPage />);

    await waitFor(() => {
      expect(screen.getByText("Alice")).toBeInTheDocument();
      expect(screen.getByText("Project One (member)")).toBeInTheDocument();
      expect(screen.getByText("Registry Secrets")).toBeInTheDocument();
    });

    expect(screen.getByText("owner")).toBeInTheDocument();
  });
});
