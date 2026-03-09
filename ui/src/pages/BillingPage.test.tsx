import React from "react";
import { screen, waitFor } from "@testing-library/react";
import { vi } from "vitest";
import { renderWithProviders } from "../test/render";
import { BillingPage } from "./BillingPage";

vi.mock("react-router-dom", async () => {
  const actual = await vi.importActual<typeof import("react-router-dom")>("react-router-dom");
  return {
    ...actual,
    useParams: () => ({ workspace: "default" })
  };
});

describe("BillingPage", () => {
  it("renders heading and empty state when no billing data is available", async () => {
    renderWithProviders(<BillingPage />);

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: "Billing" })).toBeInTheDocument();
    });

    expect(screen.getByText("No billed runs yet")).toBeInTheDocument();
    expect(screen.getByText("GPU Seconds")).toBeInTheDocument();
  });
});
