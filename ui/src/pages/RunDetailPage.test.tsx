import React from "react";
import { screen, waitFor, fireEvent } from "@testing-library/react";
import { vi } from "vitest";
import { RunDetailPage } from "./RunDetailPage";
import { renderWithProviders } from "../test/render";

vi.mock("../api", () => ({
  getRun: vi.fn(async () => ({ id: "r1", name: "run-1", status: "RUNNING", created_at: new Date().toISOString() })),
  listJobs: vi.fn(async () => [
    { id: "j1", name: "build", image: "build-image", status: "SUCCEEDED", executor_type: "swarm" },
    { id: "j2", name: "same", image: "same", status: "FAILED", executor_type: "swarm" }
  ]),
  listArtefacts: vi.fn(async () => []),
  savePromotion: vi.fn(),
  listPromotions: vi.fn(() => [])
}));

vi.mock("react-router-dom", async () => {
  const actual = await vi.importActual<typeof import("react-router-dom")>("react-router-dom");
  return {
    ...actual,
    useParams: () => ({ projectSlug: "proj", runSlug: "run-1" }),
    useNavigate: () => vi.fn()
  };
});

describe("RunDetailPage", () => {
  it("renders job name and image in overview table", async () => {
    renderWithProviders(<RunDetailPage />);

    await waitFor(() => {
      expect(screen.getByText("build")).toBeInTheDocument();
    });

    expect(screen.getByText("build-image")).toBeInTheDocument();
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
    const sameLabels = screen.getAllByText("same");
    expect(sameLabels.length).toBe(1);
  });
});
