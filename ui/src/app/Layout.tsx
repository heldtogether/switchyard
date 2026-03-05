import React, { useEffect, useRef, useState } from "react";
import { NavLink, useNavigate, useParams } from "react-router-dom";
import clsx from "clsx";
import { useAuth } from "../auth/AuthProvider";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { createWorkspace, listWorkspaces, setWorkspaceSlug } from "../api";
import { Modal } from "../components/Modal";
import { slugify } from "../utils/slug";

export function Layout({ children }: { children: React.ReactNode }) {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { workspace = "" } = useParams();
  const [workspaceMenuOpen, setWorkspaceMenuOpen] = useState(false);
  const [workspaceModalOpen, setWorkspaceModalOpen] = useState(false);
  const [workspaceName, setWorkspaceName] = useState("");
  const [workspaceSlugInput, setWorkspaceSlugInput] = useState("");
  const [workspaceDescription, setWorkspaceDescription] = useState("");
  const [workspaceCreateError, setWorkspaceCreateError] = useState<string | null>(null);
  const [workspaceCreating, setWorkspaceCreating] = useState(false);
  const menuRef = useRef<HTMLDivElement | null>(null);

  setWorkspaceSlug(workspace);

  const { data: workspaces = [] } = useQuery({
    queryKey: ["workspaces"],
    queryFn: listWorkspaces
  });
  const currentWorkspace = workspaces.find((ws) => ws.slug === workspace);

  useEffect(() => {
    function onDocumentMouseDown(event: MouseEvent) {
      if (!workspaceMenuOpen) {
        return;
      }
      if (!menuRef.current?.contains(event.target as Node)) {
        setWorkspaceMenuOpen(false);
      }
    }
    function onDocumentEscape(event: KeyboardEvent) {
      if (event.key === "Escape") {
        setWorkspaceMenuOpen(false);
      }
    }
    document.addEventListener("mousedown", onDocumentMouseDown);
    document.addEventListener("keydown", onDocumentEscape);
    return () => {
      document.removeEventListener("mousedown", onDocumentMouseDown);
      document.removeEventListener("keydown", onDocumentEscape);
    };
  }, [workspaceMenuOpen]);

  const navItems = [
    { label: "Projects", to: `/${workspace}` },
    { label: "Runs", to: `/${workspace}/runs` },
    { label: "Jobs", to: `/${workspace}/jobs` },
    { label: "Artefacts", to: `/${workspace}/artefacts` },
    { label: "Executors", to: `/${workspace}/executors` },
    { label: "Settings", to: `/${workspace}/settings` }
  ];

  const { user, logoutUrl } = useAuth();
  const displayName = user?.name ?? user?.email ?? "User";
  const avatarInitials = displayName
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase() ?? "")
    .join("") || "U";

  async function onCreateWorkspace() {
    const name = workspaceName.trim();
    const slug = workspaceSlugInput.trim();
    if (!name || !slug) {
      setWorkspaceCreateError("Name and slug are required.");
      return;
    }
    setWorkspaceCreating(true);
    setWorkspaceCreateError(null);
    try {
      const created = await createWorkspace({
        name,
        slug,
        description: workspaceDescription.trim() || undefined
      });
      await queryClient.invalidateQueries({ queryKey: ["workspaces"] });
      setWorkspaceModalOpen(false);
      setWorkspaceMenuOpen(false);
      setWorkspaceName("");
      setWorkspaceSlugInput("");
      setWorkspaceDescription("");
      navigate(`/${created.slug}`);
    } catch (error) {
      setWorkspaceCreateError((error as Error).message);
    } finally {
      setWorkspaceCreating(false);
    }
  }

  return (
    <div className="min-h-screen">
      <header className="sticky top-0 z-40 border-b border-ink-100 bg-white/80 backdrop-blur">
        <div className="mx-auto flex max-w-6xl items-center justify-between px-6 py-4">
          <div className="flex items-center gap-4">
            <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-ink-900 text-sm font-semibold text-white">
              SY
            </div>
            <div>
              <p className="text-sm font-semibold text-ink-900">Switchyard</p>
            </div>
          </div>
          <div className="flex items-center gap-4">
            {/* <button
              type="button"
              className="hidden w-72 rounded-full border border-ink-200 bg-white px-4 py-2 text-left text-sm text-ink-400 shadow-sm md:block"
            >
              ⌘K Search
            </button> */}
            <div className="relative" ref={menuRef}>
              <button
                type="button"
                className="rounded-full border border-ink-200 bg-white px-3 py-1 text-xs text-ink-700"
                onClick={() => setWorkspaceMenuOpen((open) => !open)}
              >
                {currentWorkspace?.name ?? workspace} ▾
              </button>
              {workspaceMenuOpen && (
                <div className="absolute right-0 top-10 z-50 w-72 rounded-xl border border-ink-200 bg-white p-3 shadow-lg">
                  <label className="text-[11px] uppercase tracking-[0.15em] text-ink-500">Workspace</label>
                  <select
                    className="mt-2 w-full rounded-lg border border-ink-200 bg-white px-3 py-2 text-sm text-ink-700"
                    value={workspace}
                    onChange={(e) => {
                      navigate(`/${e.target.value}`);
                      setWorkspaceMenuOpen(false);
                    }}
                  >
                    {workspaces.map((ws) => (
                      <option key={ws.slug} value={ws.slug}>
                        {ws.name}
                      </option>
                    ))}
                  </select>
                  <button
                    type="button"
                    className="mt-3 w-full rounded-full border border-ink-200 px-3 py-2 text-sm font-medium text-ink-700 hover:bg-ink-50"
                    onClick={() => {
                      setWorkspaceModalOpen(true);
                      setWorkspaceMenuOpen(false);
                    }}
                  >
                    Create Workspace
                  </button>
                </div>
              )}
            </div>
            <div className="hidden text-right md:block">
              <p className="text-xs font-semibold text-ink-900">{displayName}</p>
              {user?.email && <p className="text-[11px] text-ink-500">{user.email}</p>}
            </div>
            {user?.picture_url ? (
              <img
                src={user.picture_url}
                alt={displayName}
                className="h-9 w-9 rounded-full border border-ink-200 object-cover"
              />
            ) : (
              <div className="flex h-9 w-9 items-center justify-center rounded-full bg-ink-900 text-xs font-semibold text-white">
                {avatarInitials}
              </div>
            )}
            <button
              type="button"
              className="rounded-full border border-ink-200 px-3 py-1 text-xs text-ink-600"
              onClick={() => window.location.assign(logoutUrl)}
            >
              Logout
            </button>
          </div>
        </div>
      </header>
      <div className="mx-auto flex max-w-6xl gap-6 px-6 py-8">
        <aside className="hidden w-56 flex-shrink-0 md:block">
          <nav className="surface p-4">
            <p className="text-xs uppercase tracking-[0.2em] text-ink-400">Navigation</p>
            <div className="mt-4 flex flex-col gap-1">
              {navItems.map((item) => (
                <NavLink
                  key={item.to}
                  to={item.to}
                  end={item.to === `/${workspace}`}
                  className={({ isActive }) =>
                    clsx(
                      "rounded-lg px-3 py-2 text-sm font-medium",
                      isActive
                        ? "bg-ink-900 text-white"
                        : "text-ink-500 hover:bg-ink-100 hover:text-ink-900"
                    )
                  }
                >
                  {item.label}
                </NavLink>
              ))}
            </div>
          </nav>
        </aside>
        <main className="flex-1 min-w-0 space-y-8">{children}</main>
      </div>
      <Modal
        open={workspaceModalOpen}
        title="Create Workspace"
        description="Create a new workspace for projects, members, and runs."
        onClose={() => setWorkspaceModalOpen(false)}
        footer={
          <div className="flex justify-end gap-2">
            <button
              type="button"
              onClick={() => setWorkspaceModalOpen(false)}
              className="text-sm text-ink-500"
            >
              Close
            </button>
            <button
              type="button"
              onClick={onCreateWorkspace}
              disabled={workspaceCreating}
              className="rounded-full bg-ink-900 px-4 py-2 text-sm font-semibold text-white disabled:opacity-60"
            >
              Create
            </button>
          </div>
        }
      >
        <div className="space-y-4 text-sm text-ink-600">
          {workspaceCreateError && <p className="text-sm text-red-600">{workspaceCreateError}</p>}
          <div>
            <label className="text-xs uppercase tracking-[0.2em] text-ink-400">Name</label>
            <input
              className="mt-2 w-full rounded-lg border border-ink-200 px-3 py-2"
              placeholder="Acme"
              value={workspaceName}
              onChange={(e) => {
                const nextName = e.target.value;
                setWorkspaceName(nextName);
                if (!workspaceSlugInput.trim()) {
                  setWorkspaceSlugInput(slugify(nextName));
                }
              }}
            />
          </div>
          <div>
            <label className="text-xs uppercase tracking-[0.2em] text-ink-400">Slug</label>
            <input
              className="mt-2 w-full rounded-lg border border-ink-200 px-3 py-2"
              placeholder="acme"
              value={workspaceSlugInput}
              onChange={(e) => setWorkspaceSlugInput(slugify(e.target.value))}
            />
          </div>
          <div>
            <label className="text-xs uppercase tracking-[0.2em] text-ink-400">Description</label>
            <textarea
              className="mt-2 w-full rounded-lg border border-ink-200 px-3 py-2"
              placeholder="Optional description"
              rows={3}
              value={workspaceDescription}
              onChange={(e) => setWorkspaceDescription(e.target.value)}
            />
          </div>
        </div>
      </Modal>
    </div>
  );
}
