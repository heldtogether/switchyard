import React from "react";
import { screen, waitFor } from "@testing-library/react";
import { vi } from "vitest";
import { RunsListPage } from "./RunsListPage";
import { renderWithProviders } from "../test/render";

const listProjectsMock = vi.fn(async () => [{ id: "p1", slug: "proj", name: "Project" }]);
const listAllRunsMock = vi.fn(async () => [
  {
    id: "r1",
    slug: "run-1",
    name: "Run One",
    status: "SUCCEEDED",
    trigger: "manual",
    project_slug: "proj",
    project_name: "Project",
    created_at: new Date().toISOString()
  }
]);

vi.mock("../api", () => ({
  listProjects: (...args: any[]) => listProjectsMock(...args),
  listAllRuns: (...args: any[]) => listAllRunsMock(...args)
}));

vi.mock("react-router-dom", async () => {
  const actual = await vi.importActual<typeof import("react-router-dom")>("react-router-dom");
  return {
    ...actual,
    useParams: () => ({ workspace: "default" }),
    useNavigate: () => vi.fn()
  };
});

describe("RunsListPage", () => {
  beforeEach(() => {
    listProjectsMock.mockClear();
    listAllRunsMock.mockClear();
  });

  it("loads projects and runs for the route workspace", async () => {
    renderWithProviders(<RunsListPage />);

    await waitFor(() => {
      expect(screen.getByText("Run One")).toBeInTheDocument();
    });

    expect(listProjectsMock).toHaveBeenCalledWith("default");
    expect(listAllRunsMock).toHaveBeenCalledWith("default");
  });
});
