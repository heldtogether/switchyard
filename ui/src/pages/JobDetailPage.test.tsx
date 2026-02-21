import React from "react";
import { screen, waitFor } from "@testing-library/react";
import { vi } from "vitest";
import { JobDetailPage } from "./JobDetailPage";
import { renderWithProviders } from "../test/render";

vi.mock("../api", () => ({
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
      expect(screen.getByText("build")).toBeInTheDocument();
    });

    expect(screen.getByText("build-image")).toBeInTheDocument();
  });
});
