# Switchyard UI

SaaS-grade frontend for Switchyard. Vite + React + Tailwind.

## Local development
```bash
cd ui
npm install
npm run dev
```

## Configuration
Create a `.env.local` file in `ui/` with:
```
VITE_API_BASE_URL=http://localhost:8080
VITE_API_KEY=your-api-key
VITE_WORKSPACE_SLUG=default
VITE_USE_MOCKS=true
```

- Set `VITE_USE_MOCKS=false` to hit real endpoints.
- `VITE_API_KEY` is sent as `X-API-Key`.

## Endpoints used
**Real endpoints (available in API):**
- `GET /v1/workspaces/:workspace/projects`
- `GET /v1/workspaces/:workspace/projects/:projectSlug`
- `GET /v1/workspaces/:workspace/projects/:projectSlug/runs`
- `GET /v1/workspaces/:workspace/projects/:projectSlug/runs/:runSlug`
- `GET /v1/workspaces/:workspace/projects/:projectSlug/runs/:runSlug/jobs`
- `GET /v1/workspaces/:workspace/projects/:projectSlug/runs/:runSlug/jobs/:jobId`
- `GET /v1/workspaces/:workspace/projects/:projectSlug/runs/:runSlug/jobs/:jobId/logs`
- `GET /v1/workspaces/:workspace/projects/:projectSlug/runs/:runSlug/jobs/:jobId/artefacts`

**Mocked / client-side only:**
- Promotions (stored in `localStorage`)
- Run tags, run number, triggers (read from `metadata` when present)
