import React from "react";
import { screen, waitFor } from "@testing-library/react";
import { vi } from "vitest";
import { renderWithProviders } from "../test/render";
import { BillingPage } from "./BillingPage";

const getWorkspaceMonthToDateBillingMock = vi.fn(async () => ({
  workspace_id: "default",
  month_key: "2026-05",
  cpu_seconds: 0,
  memory_gb_seconds: 0,
  gpu_seconds: 0,
  estimated_total_minor: 0,
  estimated_total_minor_exact: 0,
  currency: "USD"
}));
const listProjectsMock = vi.fn(async () => []);
const listRunsMock = vi.fn(async () => []);
const getRunBillingBreakdownMock = vi.fn(async () => ({
  workspace_id: "default",
  project_id: "p1",
  run_id: "r1",
  cpu_seconds: 0,
  memory_gb_seconds: 0,
  gpu_seconds: 0,
  estimated_total_minor: 0,
  estimated_total_minor_exact: 0,
  currency: "USD",
  items: []
}));

vi.mock("../api", () => ({
  getWorkspaceMonthToDateBilling: (...args: any[]) => getWorkspaceMonthToDateBillingMock(...args),
  listProjects: (...args: any[]) => listProjectsMock(...args),
  listRuns: (...args: any[]) => listRunsMock(...args),
  getRunBillingBreakdown: (...args: any[]) => getRunBillingBreakdownMock(...args)
}));

vi.mock("react-router-dom", async () => {
  const actual = await vi.importActual<typeof import("react-router-dom")>("react-router-dom");
  return {
    ...actual,
    useParams: () => ({ workspace: "default" })
  };
});

describe("BillingPage", () => {
  beforeEach(() => {
    getWorkspaceMonthToDateBillingMock.mockClear();
    listProjectsMock.mockReset();
    listProjectsMock.mockResolvedValue([]);
    listRunsMock.mockReset();
    listRunsMock.mockResolvedValue([]);
    getRunBillingBreakdownMock.mockClear();
  });

  it("renders heading and empty state when no billing data is available", async () => {
    renderWithProviders(<BillingPage />);

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: "Billing" })).toBeInTheDocument();
    });

    expect(screen.getByText("No billed runs yet")).toBeInTheDocument();
    expect(screen.getByText("GPU Seconds")).toBeInTheDocument();
    expect(getWorkspaceMonthToDateBillingMock).toHaveBeenCalledWith("default");
    expect(listProjectsMock).toHaveBeenCalledWith("default");
  });

  it("loads recent billed runs for the route workspace", async () => {
    listProjectsMock.mockResolvedValueOnce([{ id: "p1", slug: "proj", name: "Project" }]);
    listRunsMock.mockResolvedValueOnce([
      {
        id: "r1",
        slug: "run-1",
        name: "Run One",
        status: "SUCCEEDED",
        created_at: new Date().toISOString()
      }
    ]);
    getRunBillingBreakdownMock.mockResolvedValueOnce({
      workspace_id: "default",
      project_id: "p1",
      run_id: "r1",
      cpu_seconds: 0,
      memory_gb_seconds: 0,
      gpu_seconds: 0,
      estimated_total_minor: 1,
      estimated_total_minor_exact: 1,
      currency: "USD",
      items: []
    });

    renderWithProviders(<BillingPage />);

    expect(await screen.findByText("Run One")).toBeInTheDocument();
    expect(listProjectsMock).toHaveBeenCalledWith("default");
    expect(listRunsMock).toHaveBeenCalledWith("proj", "default");
    expect(getRunBillingBreakdownMock).toHaveBeenCalledWith("proj", "run-1", "default");
  });
});
