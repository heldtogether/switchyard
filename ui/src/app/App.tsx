import React from "react";
import { Navigate, Outlet, Route, Routes } from "react-router-dom";
import { Layout } from "./Layout";
import { ProjectsListPage } from "../pages/ProjectsListPage";
import { ProjectRunsPage } from "../pages/ProjectRunsPage";
import { RunDetailPage } from "../pages/RunDetailPage";
import { JobDetailPage } from "../pages/JobDetailPage";
import { RunsListPage } from "../pages/RunsListPage";
import { JobsListPage } from "../pages/JobsListPage";
import { ArtefactsListPage } from "../pages/ArtefactsListPage";
import { ExecutorsPage } from "../pages/ExecutorsPage";
import { SettingsPage } from "../pages/SettingsPage";
import { BillingPage } from "../pages/BillingPage";
import { LoginPage } from "../pages/LoginPage";
import { RequireAuth } from "../auth/RequireAuth";
import { AcceptInvitePage } from "../pages/AcceptInvitePage";

function ProtectedLayout() {
  return (
    <RequireAuth>
      <Layout>
        <Outlet />
      </Layout>
    </RequireAuth>
  );
}

export default function App() {
  const runtimeEnv = (window as any).__ENV ?? {};
  const defaultWorkspace = runtimeEnv.WORKSPACE_SLUG ?? import.meta.env.VITE_WORKSPACE_SLUG ?? "default";
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/accept-invite" element={<AcceptInvitePage />} />
      <Route element={<ProtectedLayout />}>
        <Route path="/:workspace" element={<ProjectsListPage />} />
        <Route path="/:workspace/:projectSlug" element={<ProjectRunsPage />} />
        <Route path="/:workspace/runs" element={<RunsListPage />} />
        <Route path="/:workspace/jobs" element={<JobsListPage />} />
        <Route path="/:workspace/artefacts" element={<ArtefactsListPage />} />
        <Route path="/:workspace/billing" element={<BillingPage />} />
        <Route path="/:workspace/executors" element={<ExecutorsPage />} />
        <Route path="/:workspace/settings" element={<SettingsPage />} />
        <Route path="/:workspace/:projectSlug/:runSlug" element={<RunDetailPage />} />
        <Route path="/:workspace/:projectSlug/:runSlug/:jobId" element={<JobDetailPage />} />
      </Route>
      <Route path="/" element={<Navigate to={`/${defaultWorkspace}`} replace />} />
      <Route path="*" element={<Navigate to={`/${defaultWorkspace}`} replace />} />
    </Routes>
  );
}
