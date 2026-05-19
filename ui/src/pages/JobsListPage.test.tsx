import React from "react";
import { screen, waitFor } from "@testing-library/react";
import { vi } from "vitest";
import { JobsListPage } from "./JobsListPage";
import { renderWithProviders } from "../test/render";

const listProjectsMock = vi.fn(async () => [{ id: "p1", slug: "proj", name: "Project" }]);
const listAllJobsMock = vi.fn(async () => [
  { id: "j1", name: "build", image: "build-image", status: "SUCCEEDED", project_slug: "proj", run_slug: "r1" },
  { id: "j2", name: "same", image: "same", status: "FAILED", project_slug: "proj", run_slug: "r1" },
  { id: "j3", name: undefined, image: "img-only", status: "RUNNING", project_slug: "proj", run_slug: "r1" }
]);

vi.mock("../api", () => ({
  listProjects: (...args: any[]) => listProjectsMock(...args),
  listAllJobs: (...args: any[]) => listAllJobsMock(...args)
}));

vi.mock("react-router-dom", async () => {
  const actual = await vi.importActual<typeof import("react-router-dom")>("react-router-dom");
  return {
    ...actual,
    useParams: () => ({ workspace: "default" }),
    useNavigate: () => vi.fn()
  };
});

describe("JobsListPage", () => {
  beforeEach(() => {
    listProjectsMock.mockClear();
    listAllJobsMock.mockClear();
  });

  it("loads projects and jobs for the route workspace", async () => {
    renderWithProviders(<JobsListPage />);

    await waitFor(() => {
      expect(screen.getByText("build")).toBeInTheDocument();
    });

    expect(listProjectsMock).toHaveBeenCalledWith("default");
    expect(listAllJobsMock).toHaveBeenCalledWith("default");
  });

  it("renders job name as primary and image as secondary when different", async () => {
    renderWithProviders(<JobsListPage />);

    await waitFor(() => {
      expect(screen.getByText("build")).toBeInTheDocument();
    });

    expect(screen.getByText("build")).toBeInTheDocument();
    expect(screen.getByText("build-image")).toBeInTheDocument();
  });

  it("hides secondary image when it matches the name", async () => {
    renderWithProviders(<JobsListPage />);

    await waitFor(() => {
      expect(screen.getByText("same")).toBeInTheDocument();
    });

    const sameLabels = screen.getAllByText("same");
    expect(sameLabels.length).toBe(1);
  });

  it("falls back to image as primary when name is missing", async () => {
    renderWithProviders(<JobsListPage />);

    await waitFor(() => {
      expect(screen.getByText("img-only")).toBeInTheDocument();
    });

    const imgOnlyLabels = screen.getAllByText("img-only");
    expect(imgOnlyLabels.length).toBe(1);
  });
});
