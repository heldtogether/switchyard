import React from "react";
import { screen, waitFor, fireEvent } from "@testing-library/react";
import { vi } from "vitest";
import { RunDetailPage } from "./RunDetailPage";
import { renderWithProviders } from "../test/render";

const { rerunRunMock, navigateMock } = vi.hoisted(() => ({
  rerunRunMock: vi.fn(async () => ({ run: { slug: "run-2" } })),
  navigateMock: vi.fn()
}));

vi.mock("../api", () => ({
  getProject: vi.fn(async () => ({ id: "p1", slug: "proj", name: "Project Name" })),
  getRun: vi.fn(async () => ({ id: "r1", slug: "run-1", name: "run-1", status: "RUNNING", created_at: new Date().toISOString() })),
  listJobs: vi.fn(async () => [
    { id: "j1", name: "build", image: "build-image", status: "SUCCEEDED", executor_type: "docker" },
    { id: "j2", name: "same", image: "same", status: "FAILED", executor_type: "docker" }
  ]),
  getRunBillingBreakdown: vi.fn(async () => ({
    workspace_id: "default",
    project_id: "p1",
    run_id: "r1",
    cpu_seconds: 1,
    memory_gb_seconds: 1,
    gpu_seconds: 3,
    estimated_total_minor: 3,
    estimated_total_minor_exact: 3.25,
    currency: "USD",
    items: [
      {
        job_id: "j1",
        cpu_seconds: 1,
        memory_gb_seconds: 1,
        gpu_seconds: 3,
        estimated_cpu_minor: 1,
        estimated_memory_minor: 1,
        estimated_gpu_minor: 1,
        estimated_total_minor: 2,
        estimated_cpu_minor_exact: 1.2,
        estimated_memory_minor_exact: 1.1,
        estimated_gpu_minor_exact: 0.5,
        estimated_total_minor_exact: 2.3,
        pricing_version: "v1",
        currency: "USD",
        created_at: new Date().toISOString()
      },
      {
        job_id: "j2",
        cpu_seconds: 1,
        memory_gb_seconds: 1,
        gpu_seconds: 0,
        estimated_cpu_minor: 0,
        estimated_memory_minor: 1,
        estimated_gpu_minor: 0,
        estimated_total_minor: 1,
        estimated_cpu_minor_exact: 0.1,
        estimated_memory_minor_exact: 0.6,
        estimated_gpu_minor_exact: 0,
        estimated_total_minor_exact: 0.7,
        pricing_version: "v1",
        currency: "USD",
        created_at: new Date().toISOString()
      }
    ]
  })),
  listArtefacts: vi.fn(async () => []),
  createPromotion: vi.fn(),
  listCurrentPromotions: vi.fn(async () => []),
  rerunRun: rerunRunMock
}));

vi.mock("react-router-dom", async () => {
  const actual = await vi.importActual<typeof import("react-router-dom")>("react-router-dom");
  return {
    ...actual,
    useParams: () => ({ workspace: "default", projectSlug: "proj", runSlug: "run-1" }),
    useNavigate: () => navigateMock
  };
});

describe("RunDetailPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders job name and image in overview table", async () => {
    renderWithProviders(<RunDetailPage />);

    await waitFor(() => {
      expect(screen.getByText("build")).toBeInTheDocument();
    });

    expect(screen.getByRole("link", { name: "Projects" })).toHaveAttribute("href", "/default");
    expect(screen.getByRole("link", { name: "Project Name" })).toHaveAttribute("href", "/default/proj");
    expect(screen.getByText("run-1")).toBeInTheDocument();

    expect(screen.getByText("build-image")).toBeInTheDocument();
    expect(screen.getByText("$0.023")).toBeInTheDocument();
    expect(screen.getByText("GPU s: 3")).toBeInTheDocument();
    const sameLabels = screen.getAllByText("same");
    expect(sameLabels.length).toBe(1);
  });

  it("renders job name and image in jobs tab table", async () => {
    renderWithProviders(<RunDetailPage />);

    await waitFor(() => {
      expect(screen.getByText("build")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: "Jobs" }));
    expect(screen.getByText("build-image")).toBeInTheDocument();
    expect(screen.getByText("$0.023")).toBeInTheDocument();
    expect(screen.getAllByText("GPU s: 3").length).toBeGreaterThan(0);
    const sameLabels = screen.getAllByText("same");
    expect(sameLabels.length).toBe(1);
  });

  it("shows rerun options and submits selected rerun mode", async () => {
    renderWithProviders(<RunDetailPage />);

    await waitFor(() => {
      expect(screen.getByText("build")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: "Re-run" }));
    fireEvent.click(screen.getByRole("button", { name: "Re-run failed jobs only" }));

    await waitFor(() => {
      expect(rerunRunMock).toHaveBeenCalledWith("proj", "run-1", { mode: "failed_only" }, "default");
    });
    expect(navigateMock).toHaveBeenCalledWith("/default/proj/run-2");
  });
});
