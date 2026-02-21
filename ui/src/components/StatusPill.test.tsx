import React from "react";
import { render, screen } from "@testing-library/react";
import { StatusPill } from "./StatusPill";

describe("StatusPill", () => {
  it("renders labels for all statuses", () => {
    render(
      <div>
        <StatusPill status="SUCCEEDED" />
        <StatusPill status="FAILED" />
        <StatusPill status="RUNNING" />
        <StatusPill status="PENDING" />
        <StatusPill status="CANCELLED" />
        <StatusPill status="TIMEOUT" />
        <StatusPill status="PARTIAL" />
      </div>
    );

    expect(screen.getByText("Succeeded")).toBeInTheDocument();
    expect(screen.getByText("Failed")).toBeInTheDocument();
    expect(screen.getByText("Running")).toBeInTheDocument();
    expect(screen.getByText("Pending")).toBeInTheDocument();
    expect(screen.getByText("Cancelled")).toBeInTheDocument();
    expect(screen.getByText("Timeout")).toBeInTheDocument();
    expect(screen.getByText("Partial")).toBeInTheDocument();
  });
});
