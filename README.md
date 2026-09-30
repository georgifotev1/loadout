# loadout

Gaming e-commerce store for Bulgaria, built around a PC configurator. Go modular monolith
(`cmd/loadout`, `internal/*`) + TanStack Start web app (`web/`).

## Local development

Requirements: Go 1.26+, Docker, Node + pnpm.

```sh
cp .env.example .env
make up          # Postgres :5432, Mailpit UI :8025 / SMTP :1025
make migrate     # apply migrations (make migrate cmd=status|down)
make run         # API on :8080  (/livez, /readyz, /api/healthz)
make web         # web on :3000, proxies /api to :8080
```

`make help` lists all targets. `make audit` runs everything CI will run.

If another project already uses the Compose ports, set `POSTGRES_HOST_PORT`,
`MAILPIT_UI_HOST_PORT` or `MAILPIT_SMTP_HOST_PORT` in `.env` (and update `DATABASE_URL`).
