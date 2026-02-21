import React from "react";
import { Route, Routes, Navigate } from "react-router-dom";
import { Layout } from "./Layout";
import { ProjectsListPage } from "../pages/ProjectsListPage";
import { ProjectRunsPage } from "../pages/ProjectRunsPage";
import { RunDetailPage } from "../pages/RunDetailPage";
import { JobDetailPage } from "../pages/JobDetailPage";
import { PlaceholderPage } from "../pages/PlaceholderPage";

export default function App() {
  return (
    <Layout>
      <Routes>
        <Route path="/" element={<Navigate to="/projects" replace />} />
        <Route path="/projects" element={<ProjectsListPage />} />
        <Route path="/projects/:projectSlug" element={<ProjectRunsPage />} />
        <Route path="/projects/:projectSlug/runs/:runSlug" element={<RunDetailPage />} />
        <Route
          path="/projects/:projectSlug/runs/:runSlug/jobs/:jobId"
          element={<JobDetailPage />}
        />
        <Route path="/runs" element={<PlaceholderPage title="Runs" />} />
        <Route path="/jobs" element={<PlaceholderPage title="Jobs" />} />
        <Route path="/artefacts" element={<PlaceholderPage title="Artefacts" />} />
        <Route path="/executors" element={<PlaceholderPage title="Executors" />} />
        <Route path="/settings" element={<PlaceholderPage title="Settings" />} />
      </Routes>
    </Layout>
  );
}
