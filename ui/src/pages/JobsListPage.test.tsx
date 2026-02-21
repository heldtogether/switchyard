import React from "react";
import { screen, waitFor } from "@testing-library/react";
import { vi } from "vitest";
import { JobsListPage } from "./JobsListPage";
import { renderWithProviders } from "../test/render";

vi.mock("../api", () => ({
  listProjects: vi.fn(async () => [{ id: "p1", slug: "proj", name: "Project" }]),
  listAllJobs: vi.fn(async () => [
    { id: "j1", name: "build", image: "build-image", status: "SUCCEEDED", project_slug: "proj", run_slug: "r1" },
    { id: "j2", name: "same", image: "same", status: "FAILED", project_slug: "proj", run_slug: "r1" },
    { id: "j3", name: undefined, image: "img-only", status: "RUNNING", project_slug: "proj", run_slug: "r1" }
  ])
}));

describe("JobsListPage", () => {
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
