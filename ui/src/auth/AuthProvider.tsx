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

type AuthContextValue = {
  user: AuthUser | null;
  isLoading: boolean;
  isAuthenticated: boolean;
  loginUrl: string;
  logoutUrl: string;
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
          } as AuthUser
        };
      }
      return fetchJson<{ user: AuthUser }>("/v1/auth/me");
    },
    retry: false
  });

  const value = useMemo<AuthContextValue>(() => {
    const user = userQuery.isSuccess ? (userQuery.data?.user ?? null) : null;
    const isUnauthorized = userQuery.error instanceof ApiError && userQuery.error.status === 401;
    return {
      user,
      isLoading: userQuery.isLoading,
      isAuthenticated: userQuery.isSuccess && !!user && !isUnauthorized,
      loginUrl: AUTH_LOGIN_URL ?? `${API_BASE_URL}/v1/auth/login`,
      logoutUrl: AUTH_LOGOUT_URL ?? `${API_BASE_URL}/v1/auth/logout`
    };
  }, [userQuery.data, userQuery.error, userQuery.isLoading, userQuery]);

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error("useAuth must be used inside AuthProvider");
  }
  return context;
}
