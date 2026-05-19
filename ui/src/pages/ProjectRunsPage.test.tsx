import React from "react";
import { screen, waitFor } from "@testing-library/react";
import { vi } from "vitest";
import { ProjectRunsPage } from "./ProjectRunsPage";
import { renderWithProviders } from "../test/render";

const getProjectMock = vi.fn(async () => ({
  id: "p1",
  slug: "proj",
  name: "Project Name",
  description: "A project",
  created_at: new Date().toISOString(),
  updated_at: new Date().toISOString()
}));
const listRunsMock = vi.fn(async () => []);
const listCurrentPromotionsMock = vi.fn(async () => []);

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return {
    ...actual,
    getProject: (...args: any[]) => getProjectMock(...args),
    listRuns: (...args: any[]) => listRunsMock(...args),
    listCurrentPromotions: (...args: any[]) => listCurrentPromotionsMock(...args)
  };
});

vi.mock("react-router-dom", async () => {
  const actual = await vi.importActual<typeof import("react-router-dom")>("react-router-dom");
  return {
    ...actual,
    useParams: () => ({ workspace: "default", projectSlug: "proj" }),
    useNavigate: () => vi.fn()
  };
});

describe("ProjectRunsPage", () => {
  beforeEach(() => {
    getProjectMock.mockClear();
    listRunsMock.mockClear();
    listCurrentPromotionsMock.mockClear();
  });

  it("renders breadcrumbs for projects hierarchy", async () => {
    renderWithProviders(<ProjectRunsPage />);

    await waitFor(() => {
      expect(screen.getAllByText("Project Name").length).toBeGreaterThan(0);
    });

    expect(screen.getByRole("link", { name: "Projects" })).toHaveAttribute("href", "/default");
    expect(screen.getAllByText("Project Name").length).toBeGreaterThan(0);
    expect(getProjectMock).toHaveBeenCalledWith("proj", "default");
    expect(listRunsMock).toHaveBeenCalledWith("proj", "default");
    expect(listCurrentPromotionsMock).toHaveBeenCalledWith("proj", "default");
  });
});
