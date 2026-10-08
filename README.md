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

Card payments (optional locally; checkout offers cash on delivery without them) use a Stripe
sandbox: put a restricted key (`rk_test_…`, with write access to Checkout Sessions and Refunds)
in `STRIPE_SECRET_KEY`, run `make stripe` and copy the `whsec_…` it prints to
`STRIPE_WEBHOOK_SECRET`. Pay with the test card `4242 4242 4242 4242`.

Or run it all in one terminal with `make dev`: infrastructure, migrations, the API, the web
app, mock-vali and (with Stripe keys in `.env`) Stripe's webhook forwarding, each log line
prefixed by its process, with the addresses printed at the start. Ctrl+C stops them all.

`make help` lists all targets. `make audit` runs everything CI will run.

If another project already uses the Compose ports, set `POSTGRES_HOST_PORT`,
`MAILPIT_UI_HOST_PORT` or `MAILPIT_SMTP_HOST_PORT` in `.env` (and update `DATABASE_URL`).

## API contract

`api/openapi.yaml` is the source of truth for the public API. After changing it, run `make gen`
to regenerate the Go server (`cmd/loadout/api.gen.go`) and the web app's types
(`web/src/lib/api/schema.gen.ts`). `make audit` fails if they are out of date.
