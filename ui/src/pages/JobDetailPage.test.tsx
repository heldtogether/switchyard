import React from "react";
import { screen, waitFor } from "@testing-library/react";
import { vi } from "vitest";
import { JobDetailPage } from "./JobDetailPage";
import { renderWithProviders } from "../test/render";

vi.mock("../api", () => ({
  getProject: vi.fn(async () => ({ id: "p1", slug: "proj", name: "Project Name" })),
  getRun: vi.fn(async () => ({ id: "r1", slug: "run-1", name: "Run Name", status: "SUCCEEDED", created_at: new Date().toISOString() })),
  getJob: vi.fn(async () => ({ id: "j1", name: "build", image: "build-image", status: "SUCCEEDED" })),
  getJobLogs: vi.fn(async () => ""),
  listArtefacts: vi.fn(async () => [])
}));

vi.mock("react-router-dom", async () => {
  const actual = await vi.importActual<typeof import("react-router-dom")>("react-router-dom");
  return {
    ...actual,
    useParams: () => ({ projectSlug: "proj", runSlug: "run-1", jobId: "j1" })
  };
});

describe("JobDetailPage", () => {
  it("uses name as title and image as subtitle when different", async () => {
    renderWithProviders(<JobDetailPage />);

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: "build" })).toBeInTheDocument();
    });

    expect(screen.getByRole("link", { name: "Projects" })).toHaveAttribute("href", "/");
    expect(screen.getByRole("link", { name: "Project Name" })).toHaveAttribute("href", "/proj");
    expect(screen.getByRole("link", { name: "Run Name" })).toHaveAttribute("href", "/proj/run-1");
    expect(screen.getByText("build-image")).toBeInTheDocument();
  });
});
