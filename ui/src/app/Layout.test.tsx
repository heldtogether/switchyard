import React from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { vi } from "vitest";
import { Layout } from "./Layout";

const listWorkspacesMock = vi.fn(async () => [
  {
    id: "ws-default",
    slug: "default",
    name: "Default Workspace",
    description: "",
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString()
  }
]);
const createWorkspaceMock = vi.fn(async () => ({
  id: "ws-new",
  slug: "new-org",
  name: "New Org",
  description: "",
  created_at: new Date().toISOString(),
  updated_at: new Date().toISOString()
}));
const setWorkspaceSlugMock = vi.fn();

vi.mock("../api", () => ({
  listWorkspaces: () => listWorkspacesMock(),
  createWorkspace: (payload: { name: string; slug: string; description?: string }) => createWorkspaceMock(payload),
  setWorkspaceSlug: (slug?: string) => setWorkspaceSlugMock(slug)
}));

vi.mock("../auth/AuthProvider", () => ({
  useAuth: () => ({
    user: { email: "owner@example.com", name: "Owner User" },
    logoutUrl: "/logout"
  })
}));

function renderLayout() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false
      }
    }
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={["/default"]}>
        <Routes>
          <Route
            path="/:workspace"
            element={
              <Layout>
                <div>Child Content</div>
              </Layout>
            }
          />
          <Route path="/:workspace/settings" element={<div>Settings</div>} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>
  );
}

describe("Layout workspace creation", () => {
  beforeEach(() => {
    listWorkspacesMock.mockClear();
    createWorkspaceMock.mockClear();
    setWorkspaceSlugMock.mockClear();
  });

  it("creates workspace from switcher popover", async () => {
    renderLayout();

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /default workspace/i })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: /default workspace/i }));
    fireEvent.click(screen.getByRole("button", { name: /create workspace/i }));

    fireEvent.change(screen.getByPlaceholderText("Acme"), { target: { value: "New Org" } });
    fireEvent.click(screen.getByRole("button", { name: "Create" }));

    await waitFor(() => {
      expect(createWorkspaceMock).toHaveBeenCalledWith({
        name: "New Org",
        slug: "new-org",
        description: undefined
      });
    });
  });
});
