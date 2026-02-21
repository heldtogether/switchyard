import React from "react";
import { Route, Routes } from "react-router-dom";
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

export default function App() {
  return (
    <Layout>
      <Routes>
        <Route path="/" element={<ProjectsListPage />} />
        <Route path="/projects/:projectSlug" element={<ProjectRunsPage />} />
        <Route path="/runs" element={<RunsListPage />} />
        <Route path="/jobs" element={<JobsListPage />} />
        <Route path="/artefacts" element={<ArtefactsListPage />} />
        <Route path="/executors" element={<ExecutorsPage />} />
        <Route path="/settings" element={<SettingsPage />} />
        <Route path="/:projectSlug/:runSlug" element={<RunDetailPage />} />
        <Route path="/:projectSlug/:runSlug/:jobId" element={<JobDetailPage />} />
      </Routes>
    </Layout>
  );
}
