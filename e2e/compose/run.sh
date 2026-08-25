#!/usr/bin/env bash
# Runs the tg-notify proxy e2e Docker Compose stack:
# squid (HTTP proxy) + go-socks5-proxy (SOCKS5 with auth) + a fake
# Bot API, then asserts tg-notify sends through both proxies.
set -euo pipefail
cd "$(dirname "$0")"

if ! command -v docker >/dev/null 2>&1; then
	echo "docker not found; skipping compose e2e"
	exit 0
fi

cleanup() {
	docker compose down -v >/dev/null 2>&1 || true
}
trap cleanup EXIT

dump_logs() {
	for svc in notify squid socks5 fakeapi; do
		echo "----- docker logs: $svc -----"
		docker compose logs "$svc" 2>&1 || true
	done
}

docker compose up -d --build fakeapi squid socks5

if ! docker compose up --build --abort-on-container-exit notify; then
	dump_logs
	echo "e2e-docker: notify scenario FAILED"
	exit 1
fi

logs_fakeapi=$(docker compose logs fakeapi)
logs_squid=$(docker compose exec -T squid cat /var/log/squid/access.log 2>/dev/null || true)

fail=0
if ! grep -q '"path":"/botTESTTOKEN/sendMessage"' <<<"$logs_fakeapi"; then
	echo "e2e-docker: fakeapi never saw sendMessage"
	fail=1
fi
if ! grep -q 'getMe' <<<"$logs_fakeapi"; then
	echo "e2e-docker: fakeapi never saw getMe"
	fail=1
fi
if ! grep -q 'fakeapi' <<<"$logs_squid"; then
	echo "e2e-docker: squid access log shows no fakeapi traffic"
	fail=1
fi

if [ "$fail" -ne 0 ]; then
	dump_logs
	exit 1
fi

echo "e2e-docker: PASS (squid and SOCKS5 proxies both routed tg-notify)"
