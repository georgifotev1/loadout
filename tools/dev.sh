#!/usr/bin/env bash
# Runs everything local development needs in one terminal (make dev): the infrastructure, the
# migrations, then the API, the web app, mock-vali and, when .env has Stripe keys, the Stripe
# webhook forwarder. Each line of their logs is prefixed with the process's name. Ctrl+C stops
# them all.
set -euo pipefail
cd "$(dirname "$0")/.."

set -a
# shellcheck disable=SC1091
[[ -f .env ]] && . ./.env
set +a

make -s up
make -s migrate

# Stop every process this script started on exit, Ctrl+C included.
trap 'trap - INT TERM EXIT; kill 0 2>/dev/null; wait 2>/dev/null' INT TERM EXIT

# run starts a make target in the background, prefixing its output with a coloured name.
run() {
	local name=$1 color=$2 target=$3
	local prefix
	prefix=$(printf '\033[%sm%-9s\033[0m│ ' "$color" "$name")
	make -s "$target" 2>&1 | while IFS= read -r line; do printf '%s%s\n' "$prefix" "$line"; done &
}

api_port=${HTTP_ADDR:-:8080}
api_port=${api_port##*:}
stripe=no
[[ -n ${STRIPE_SECRET_KEY:-} ]] && stripe=yes

run mock-vali 36 mock-vali
run api 32 run
run web 35 web
[[ $stripe == yes ]] && run stripe 34 stripe

cat <<EOF

  shop       http://localhost:3000
  admin      http://localhost:3000/admin
  api        http://localhost:${api_port}
  mock-vali  http://localhost:8081
  mailpit    http://localhost:${MAILPIT_UI_HOST_PORT:-8025}
  river ui   http://localhost:${RIVERUI_HOST_PORT:-8082}
  stripe     $([[ $stripe == yes ]] && echo "forwarding webhooks to :${api_port}" || echo "off (no STRIPE_SECRET_KEY in .env)")

  Ctrl+C stops everything (Postgres and Mailpit keep running: make down).

EOF

wait
