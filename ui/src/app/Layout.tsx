import React from "react";
import { NavLink, useNavigate, useParams } from "react-router-dom";
import clsx from "clsx";
import { useAuth } from "../auth/AuthProvider";
import { useQuery } from "@tanstack/react-query";
import { listWorkspaces, setWorkspaceSlug } from "../api";

export function Layout({ children }: { children: React.ReactNode }) {
  const navigate = useNavigate();
  const { workspace = "" } = useParams();
  setWorkspaceSlug(workspace);

  const { data: workspaces = [] } = useQuery({
    queryKey: ["workspaces"],
    queryFn: listWorkspaces
  });

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
            <button
              type="button"
              className="hidden w-72 rounded-full border border-ink-200 bg-white px-4 py-2 text-left text-sm text-ink-400 shadow-sm md:block"
            >
              ⌘K Search
            </button>
            <select
              className="rounded-full border border-ink-200 bg-white px-3 py-1 text-xs text-ink-600"
              value={workspace}
              onChange={(e) => navigate(`/${e.target.value}`)}
            >
              {workspaces.map((ws) => (
                <option key={ws.slug} value={ws.slug}>
                  {ws.name}
                </option>
              ))}
            </select>
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
    </div>
  );
}
