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
import { LoginPage } from "../pages/LoginPage";
import { RequireAuth } from "../auth/RequireAuth";

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
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route element={<ProtectedLayout />}>
        <Route path="/" element={<ProjectsListPage />} />
        <Route path="/:projectSlug" element={<ProjectRunsPage />} />
        <Route path="/runs" element={<RunsListPage />} />
        <Route path="/jobs" element={<JobsListPage />} />
        <Route path="/artefacts" element={<ArtefactsListPage />} />
        <Route path="/executors" element={<ExecutorsPage />} />
        <Route path="/settings" element={<SettingsPage />} />
        <Route path="/:projectSlug/:runSlug" element={<RunDetailPage />} />
        <Route path="/:projectSlug/:runSlug/:jobId" element={<JobDetailPage />} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
