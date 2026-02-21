import React from "react";
import { NavLink } from "react-router-dom";
import clsx from "clsx";

const navItems = [
  { label: "Projects", to: "/" },
  { label: "Runs", to: "/runs" },
  { label: "Jobs", to: "/jobs" },
  { label: "Artefacts", to: "/artefacts" },
  { label: "Executors", to: "/executors" },
  { label: "Settings", to: "/settings" }
];

export function Layout({ children }: { children: React.ReactNode }) {
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
            <div className="flex h-9 w-9 items-center justify-center rounded-full bg-ink-900 text-xs font-semibold text-white">
              JS
            </div>
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
                  end={item.to === "/"}
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
        <main className="flex-1 space-y-8">{children}</main>
      </div>
    </div>
  );
}
