import React from "react";
import { screen, waitFor } from "@testing-library/react";
import { vi } from "vitest";
import { ArtefactsListPage } from "./ArtefactsListPage";
import { renderWithProviders } from "../test/render";

const listProjectsMock = vi.fn(async () => [{ id: "p1", slug: "proj", name: "Project" }]);
const listAllArtefactsMock = vi.fn(async () => [
  {
    id: "a1",
    path: "outputs/result.json",
    size_bytes: 128,
    project_slug: "proj",
    project_name: "Project",
    run_slug: "run-1",
    run_number: 1,
    job_name: "build",
    job_image: "build-image"
  }
]);

vi.mock("../api", () => ({
  listProjects: (...args: any[]) => listProjectsMock(...args),
  listAllArtefacts: (...args: any[]) => listAllArtefactsMock(...args)
}));

vi.mock("react-router-dom", async () => {
  const actual = await vi.importActual<typeof import("react-router-dom")>("react-router-dom");
  return {
    ...actual,
    useParams: () => ({ workspace: "default" })
  };
});

describe("ArtefactsListPage", () => {
  beforeEach(() => {
    listProjectsMock.mockReset();
    listProjectsMock.mockResolvedValue([{ id: "p1", slug: "proj", name: "Project" }]);
    listAllArtefactsMock.mockReset();
    listAllArtefactsMock.mockResolvedValue([
      {
        id: "a1",
        path: "outputs/result.json",
        size_bytes: 128,
        project_slug: "proj",
        project_name: "Project",
        run_slug: "run-1",
        run_number: 1,
        job_name: "build",
        job_image: "build-image"
      }
    ]);
  });

  it("loads projects and artefacts for the route workspace", async () => {
    renderWithProviders(<ArtefactsListPage />);

    expect(await screen.findByText("outputs/result.json")).toBeInTheDocument();
    expect(listProjectsMock).toHaveBeenCalledWith("default");
    expect(listAllArtefactsMock).toHaveBeenCalledWith("default");
  });

  it("does not load aggregate artefacts when the workspace project request fails", async () => {
    listProjectsMock.mockRejectedValueOnce(new Error("Workspace not found"));

    renderWithProviders(<ArtefactsListPage />);

    await waitFor(() => {
      expect(screen.getByText("Workspace not found")).toBeInTheDocument();
    });

    expect(listAllArtefactsMock).not.toHaveBeenCalled();
  });
});
