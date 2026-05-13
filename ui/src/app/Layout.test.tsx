import React from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
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
        retry: false,
        refetchOnWindowFocus: false,
        refetchOnReconnect: false,
        refetchOnMount: false
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
    window.__ENV = { VERSION: "v1.2.3" };
    listWorkspacesMock.mockClear();
    createWorkspaceMock.mockClear();
    setWorkspaceSlugMock.mockClear();
  });

  it("shows the configured app version in the sidebar", async () => {
    renderLayout();

    await waitFor(() => {
      expect(screen.getByText("v1.2.3")).toBeInTheDocument();
    });
  });

  it("creates workspace from switcher popover", async () => {
    const user = userEvent.setup();
    renderLayout();

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /default workspace/i })).toBeInTheDocument();
    });
    expect(screen.getByRole("link", { name: "Billing" })).toHaveAttribute("href", "/default/billing");

    await user.click(screen.getByRole("button", { name: /default workspace/i }));
    await user.click(screen.getByRole("button", { name: /create workspace/i }));

    await user.type(screen.getByPlaceholderText("Acme"), "New Org");
    expect((screen.getByPlaceholderText("acme") as HTMLInputElement).value).toBe("new-org");
    await user.click(screen.getByRole("button", { name: "Create" }));

    await waitFor(() => {
      expect(createWorkspaceMock).toHaveBeenCalledWith({
        name: "New Org",
        slug: "new-org",
        description: undefined
      });
    });
  });

  it("blocks reserved workspace slug before submit", async () => {
    const user = userEvent.setup();
    renderLayout();

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /default workspace/i })).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: /default workspace/i }));
    await user.click(screen.getByRole("button", { name: /create workspace/i }));
    await user.type(screen.getByPlaceholderText("Acme"), "Reserved Workspace");
    await user.clear(screen.getByPlaceholderText("acme"));
    await user.type(screen.getByPlaceholderText("acme"), "runs");
    await user.click(screen.getByRole("button", { name: "Create" }));

    await waitFor(() => {
      expect(screen.getByText("Slug is reserved for system routes.")).toBeInTheDocument();
    });
    expect(createWorkspaceMock).not.toHaveBeenCalled();
  });
});
