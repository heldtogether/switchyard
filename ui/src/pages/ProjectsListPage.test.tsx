import React from "react";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { vi } from "vitest";
import { ProjectsListPage } from "./ProjectsListPage";
import { renderWithProviders } from "../test/render";

vi.mock("../api", () => ({
  listProjects: vi.fn(async () => []),
  listRuns: vi.fn(async () => []),
  createProject: vi.fn(async () => ({
    id: "p1",
    slug: "my-cool-project",
    name: "My Cool Project",
    description: "",
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString()
  }))
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
  it("keeps slug synced with typed project name", async () => {
    const user = userEvent.setup();
    renderWithProviders(<ProjectsListPage />);

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /create project/i })).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: /create project/i }));
    await user.type(screen.getByPlaceholderText("Vision Core"), "My Cool Project");

    expect((screen.getByPlaceholderText("vision-core") as HTMLInputElement).value).toBe(
      "my-cool-project"
    );
  });
});

