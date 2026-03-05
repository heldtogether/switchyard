import React, { createContext, useContext, useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { ApiError, fetchJson } from "../api/client";

export type AuthUser = {
  subject: string;
  email?: string;
  name?: string;
  picture_url?: string;
  provider: string;
  auth_method: string;
};

export type WorkspaceMembership = {
  slug: string;
  role: "owner" | "member";
};

export type ProjectMembership = {
  workspace_slug: string;
  project_slug: string;
  role: "owner" | "member";
};

export type AuthMemberships = {
  workspaces: WorkspaceMembership[];
  projects: ProjectMembership[];
};

type AuthMeResponse = {
  user: AuthUser;
  memberships?: AuthMemberships;
};

type AuthContextValue = {
  user: AuthUser | null;
  memberships: AuthMemberships;
  isLoading: boolean;
  isAuthenticated: boolean;
  loginUrl: string;
  logoutUrl: string;
  workspaceRole: (workspaceSlug: string) => "owner" | "member" | null;
  isWorkspaceOwner: (workspaceSlug: string) => boolean;
};

const runtimeEnv = (window as any).__ENV ?? {};
const API_BASE_URL = runtimeEnv.API_BASE_URL ?? import.meta.env.VITE_API_BASE_URL ?? "http://localhost:8080";
const AUTH_LOGIN_URL = runtimeEnv.AUTH_LOGIN_URL;
const AUTH_LOGOUT_URL = runtimeEnv.AUTH_LOGOUT_URL;
const USE_MOCKS = runtimeEnv.USE_MOCKS === "true" || import.meta.env.VITE_USE_MOCKS === "true";

const AuthContext = createContext<AuthContextValue | undefined>(undefined);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const userQuery = useQuery({
    queryKey: ["auth", "me"],
    queryFn: async () => {
      if (USE_MOCKS) {
        return {
          user: {
            subject: "mock-user",
            email: "mock@example.com",
            name: "Mock User",
            provider: "mock",
            auth_method: "mock"
          } as AuthUser,
          memberships: {
            workspaces: [{ slug: "default", role: "owner" }],
            projects: []
          } as AuthMemberships
        };
      }
      return fetchJson<AuthMeResponse>("/v1/auth/me");
    },
    retry: false
  });

  const value = useMemo<AuthContextValue>(() => {
    const user = userQuery.isSuccess ? (userQuery.data?.user ?? null) : null;
    const memberships: AuthMemberships = userQuery.isSuccess
      ? (userQuery.data?.memberships ?? { workspaces: [], projects: [] })
      : { workspaces: [], projects: [] };
    const isUnauthorized = userQuery.error instanceof ApiError && userQuery.error.status === 401;
    return {
      user,
      memberships,
      isLoading: userQuery.isLoading,
      isAuthenticated: userQuery.isSuccess && !!user && !isUnauthorized,
      loginUrl: AUTH_LOGIN_URL ?? `${API_BASE_URL}/v1/auth/login`,
      logoutUrl: AUTH_LOGOUT_URL ?? `${API_BASE_URL}/v1/auth/logout`,
      workspaceRole: (workspaceSlug: string) =>
        memberships.workspaces.find((m) => m.slug === workspaceSlug)?.role ?? null,
      isWorkspaceOwner: (workspaceSlug: string) =>
        memberships.workspaces.some((m) => m.slug === workspaceSlug && m.role === "owner")
    };
  }, [userQuery.data, userQuery.error, userQuery.isLoading, userQuery.isSuccess]);

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error("useAuth must be used inside AuthProvider");
  }
  return context;
}
