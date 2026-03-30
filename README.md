# EchoLog

EchoLog is a small blog system with a Vue frontend and a Go backend.

## Structure

```text
echolog/
  frontend/               # Vue app
  backend/
    cmd/server/           # HTTP server entry
    data/                 # Local JSON content config
    internal/             # Backend business logic
```

## Local Run

Backend:

```bash
cd backend
go run ./cmd/server
```

Frontend:

```bash
cd frontend
npm install
npm run dev
```

During local development, Vite runs on `http://localhost:5173` and proxies `/v1/echolog/...` to `http://127.0.0.1:8080`.

If frontend and backend are deployed under the same domain, the frontend will call `/v1/echolog/...` on the current origin by default. For custom split deployment, set `VITE_API_BASE`.

## Content and Admin

- Public site config and posts are stored in `backend/data/store.local.json`.
- A sample file lives at `backend/data/store.example.json`.
- The management page is available at `/thesky9531`.
- Set your own local admin secret in `backend/data/store.local.json`.

## Current Capabilities

- Public homepage with configurable navigation and ICP footer.
- Navigation items can point to either a local post or any custom URL.
- Backend-managed login session for the management page.
- General settings editor for site name, description, ICP number, and nav items.
- Markdown-based post creation, preview, editing, and deletion from the management page.
