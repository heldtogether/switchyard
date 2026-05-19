import React from "react";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { vi } from "vitest";
import { ProjectsListPage } from "./ProjectsListPage";
import { renderWithProviders } from "../test/render";

const listProjectsMock = vi.fn(async () => []);
const listRunsMock = vi.fn(async () => []);
const createProjectMock = vi.fn(async () => ({
  id: "p1",
  slug: "my-cool-project",
  name: "My Cool Project",
  description: "",
  created_at: new Date().toISOString(),
  updated_at: new Date().toISOString()
}));

vi.mock("../api", () => ({
  listProjects: (...args: any[]) => listProjectsMock(...args),
  listRuns: (...args: any[]) => listRunsMock(...args),
  createProject: (...args: any[]) => createProjectMock(...args)
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
    listProjectsMock.mockReset();
    listProjectsMock.mockResolvedValue([]);
    listRunsMock.mockReset();
    listRunsMock.mockResolvedValue([]);
    createProjectMock.mockClear();
  });

  it("loads and renders projects for the route workspace", async () => {
    listProjectsMock.mockResolvedValueOnce([
      {
        id: "p-existing",
        slug: "existing",
        name: "Existing Project",
        description: "Already here",
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString()
      }
    ]);
    renderWithProviders(<ProjectsListPage />);

    expect(await screen.findByText("Existing Project")).toBeInTheDocument();
    expect(listProjectsMock).toHaveBeenCalledWith("default");
    expect(listRunsMock).toHaveBeenCalledWith("existing", "default");
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

  it("creates projects in the route workspace", async () => {
    const user = userEvent.setup();
    renderWithProviders(<ProjectsListPage />);

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /create project/i })).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: /create project/i }));
    await user.type(screen.getByPlaceholderText("Vision Core"), "My Cool Project");
    await user.click(screen.getByRole("button", { name: "Create" }));

    await waitFor(() => {
      expect(createProjectMock).toHaveBeenCalled();
    });
    expect(createProjectMock).toHaveBeenCalledWith(
      {
        name: "My Cool Project",
        slug: "my-cool-project",
        description: undefined
      },
      "default"
    );
  });
});
