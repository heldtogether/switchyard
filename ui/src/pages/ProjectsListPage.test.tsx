import React from "react";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { vi } from "vitest";
import { ProjectsListPage } from "./ProjectsListPage";
import { renderWithProviders } from "../test/render";

const createProjectMock = vi.fn(async () => ({
  id: "p1",
  slug: "my-cool-project",
  name: "My Cool Project",
  description: "",
  created_at: new Date().toISOString(),
  updated_at: new Date().toISOString()
}));

vi.mock("../api", () => ({
  listProjects: vi.fn(async () => []),
  listRuns: vi.fn(async () => []),
  createProject: (payload: { name: string; slug: string; description?: string }) => createProjectMock(payload)
}));

vi.mock("react-router-dom", async () => {
  const actual = await vi.importActual<typeof import("react-router-dom")>("react-router-dom");
  return {
    ...actual,
    useParams: () => ({ workspace: "default" }),
    useNavigate: () => vi.fn()
  };
});

describe("ProjectsListPage", () => {
  beforeEach(() => {
    createProjectMock.mockClear();
  });

  it("keeps slug synced with typed project name", async () => {
    const user = userEvent.setup();
    renderWithProviders(<ProjectsListPage />);

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /create project/i })).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: /create project/i }));
    await user.type(screen.getByPlaceholderText("Vision Core"), "My Cool Project");

    await waitFor(() => {
      expect((screen.getByPlaceholderText("vision-core") as HTMLInputElement).value).toBe(
        "my-cool-project"
      );
    });
  });

  it("blocks reserved slugs before submit", async () => {
    const user = userEvent.setup();
    renderWithProviders(<ProjectsListPage />);

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /create project/i })).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: /create project/i }));
    await user.type(screen.getByPlaceholderText("Vision Core"), "Reserved");
    await user.clear(screen.getByPlaceholderText("vision-core"));
    await user.type(screen.getByPlaceholderText("vision-core"), "runs");
    await user.click(screen.getByRole("button", { name: "Create" }));

    await waitFor(() => {
      expect(screen.getByText("Slug is reserved for system routes.")).toBeInTheDocument();
    });
    expect(createProjectMock).not.toHaveBeenCalled();
  });
});
