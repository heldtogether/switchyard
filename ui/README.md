# Switchyard UI

SaaS-grade frontend for Switchyard. Vite + React + Tailwind.

## Local development
```bash
cd ui
npm install
npm run dev
```
The local sidebar version defaults to the nearest release tag plus the current short Git SHA, and appends `-dirty` when the worktree has uncommitted changes. Clean tagged commits show the release tag only. Set `VITE_VERSION` to override it.

## Configuration
Create a `.env.local` file in `ui/` with:
```
VITE_API_BASE_URL=http://localhost:8080
VITE_USE_MOCKS=false
VITE_WORKSPACE_SLUG=default
VITE_AGGREGATE_LIMIT=5
VITE_VERSION=v0.8.0+sha.local
```

- Set `VITE_USE_MOCKS=false` to hit real endpoints.
- Browser requests include cookies for SSO sessions.
- In split-origin setups (UI host different from API host), configure API OIDC redirects to UI absolute URLs:
  - `api.auth.oidc.post_login_redirect: "https://ui.example.com/"`
  - `api.auth.oidc.post_logout_redirect: "https://ui.example.com/login"`

## Runtime config (Docker)
The UI image reads runtime env vars and writes `/config.js` on container start:
```
UI_API_BASE_URL=http://localhost:8080
UI_AUTH_LOGIN_URL=http://localhost:8080/v1/auth/login
UI_AUTH_LOGOUT_URL=http://localhost:8080/v1/auth/logout
UI_WORKSPACE_SLUG=default
UI_USE_MOCKS=false
UI_AGGREGATE_LIMIT=5
```
- `UI_AUTH_LOGOUT_URL` should point to API `GET /v1/auth/logout` (browser redirect logout flow).

## Routes
- `/login` Login page (unauthenticated)
- `/accept-invite` Invite acceptance page (`?token=...`)
- `/:workspace` Projects list
- `/:workspace/:projectSlug` Project runs list
- `/:workspace/settings` Workspace settings (members, invites, registry secrets)
- `/:workspace/runs` Runs list (all projects)
- `/:workspace/jobs` Jobs list (all projects)
- `/:workspace/artefacts` Artefacts list (all projects)
- `/:workspace/:projectSlug/:runSlug` Run detail
- `/:workspace/:projectSlug/:runSlug/:jobId` Job detail

## Endpoints used
**Real endpoints (available in API):**
- `GET /v1/workspaces`
- `GET /v1/auth/me`
- `GET /v1/workspaces/:workspace/projects`
- `GET /v1/workspaces/:workspace/members`
- `POST /v1/workspaces/:workspace/invites`
- `POST /v1/workspace-invites/accept`
- `GET /v1/workspaces/:workspace/projects/:projectSlug/members`
- `POST /v1/workspaces/:workspace/projects/:projectSlug/invites`
- `POST /v1/project-invites/accept`
- `GET /v1/workspaces/:workspace/projects/:projectSlug`
- `GET /v1/workspaces/:workspace/projects/:projectSlug/runs`
- `GET /v1/workspaces/:workspace/projects/:projectSlug/runs/:runSlug`
- `GET /v1/workspaces/:workspace/projects/:projectSlug/runs/:runSlug/jobs`
- `GET /v1/workspaces/:workspace/projects/:projectSlug/runs/:runSlug/jobs/:jobId`
- `GET /v1/workspaces/:workspace/projects/:projectSlug/runs/:runSlug/jobs/:jobId/logs`
- `GET /v1/workspaces/:workspace/projects/:projectSlug/runs/:runSlug/jobs/:jobId/artefacts`
- `POST /v1/workspaces/:workspace/projects/:projectSlug/promotions`
- `GET /v1/workspaces/:workspace/projects/:projectSlug/promotions`
- `GET /v1/workspaces/:workspace/projects/:projectSlug/promotions/history`
- `GET /v1/workspaces/:workspace/projects/:projectSlug/promotions/:channel`
- `GET /v1/workspaces/:workspace/projects/:projectSlug/promotions/:channel/artefacts/:logicalKey`
- `GET /v1/workspaces/:workspace/registry-secrets`

**Mocked / client-side only:**
- Run tags, run number, triggers (read from `metadata` when present)

**Write endpoints used:**
- `POST /v1/workspaces` (workspace menu -> Create Workspace)
- `POST /v1/workspaces/:workspace/projects/:projectSlug/runs`
- `POST /v1/workspaces/:workspace/projects/:projectSlug/runs/:runSlug/jobs`
- `POST /v1/workspaces/:workspace/registry-secrets`
- `DELETE /v1/workspaces/:workspace/registry-secrets/:secretId`
- `POST /v1/workspaces/:workspace/registry-secrets/:secretId/rotate`
