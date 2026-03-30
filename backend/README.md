# Backend

Run the API server with:

```bash
go run ./cmd/server
```

The server listens on `:8080` by default.

## Important Files

- `backend/data/store.local.json`: live local config and content
- `backend/data/store.example.json`: sample data
- `backend/cmd/server/main.go`: HTTP entrypoint

## Routes

- `GET /health`
- `GET /v1/echolog/site`
- `GET /v1/echolog/posts`
- `GET /v1/echolog/posts/{slug}`
- `POST /v1/echolog/auth/login`
- `POST /v1/echolog/auth/logout`
- `GET /v1/echolog/auth/session`
- `GET|PUT /v1/echolog/manage/settings`
- `GET|POST /v1/echolog/manage/posts`
- `PUT|DELETE /v1/echolog/manage/posts/{id}`
