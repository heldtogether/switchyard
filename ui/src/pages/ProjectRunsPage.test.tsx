import React from "react";
import { screen, waitFor } from "@testing-library/react";
import { vi } from "vitest";
import { ProjectRunsPage } from "./ProjectRunsPage";
import { renderWithProviders } from "../test/render";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return {
    ...actual,
    getProject: vi.fn(async () => ({
      id: "p1",
      slug: "proj",
      name: "Project Name",
      description: "A project",
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString()
    })),
    listRuns: vi.fn(async () => []),
    listCurrentPromotions: vi.fn(async () => [])
  };
});

vi.mock("react-router-dom", async () => {
  const actual = await vi.importActual<typeof import("react-router-dom")>("react-router-dom");
  return {
    ...actual,
    useParams: () => ({ projectSlug: "proj" }),
    useNavigate: () => vi.fn()
  };
});

describe("ProjectRunsPage", () => {
  it("renders breadcrumbs for projects hierarchy", async () => {
    renderWithProviders(<ProjectRunsPage />);

    await waitFor(() => {
      expect(screen.getAllByText("Project Name").length).toBeGreaterThan(0);
    });

    expect(screen.getByRole("link", { name: "Projects" })).toHaveAttribute("href", "/");
    expect(screen.getAllByText("Project Name").length).toBeGreaterThan(0);
  });
});
