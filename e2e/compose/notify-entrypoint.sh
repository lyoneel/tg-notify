#!/bin/sh
# tg-notify proxy e2e scenarios, run inside the Compose stack.
set -eu

# Wait for the fake Bot API (busybox wget; up to 30 iterations).
i=0
until wget -q -O- "$FAKEAPI_URL/botTESTTOKEN/getMe" >/dev/null 2>&1; do
	i=$((i + 1))
	if [ "$i" -ge 30 ]; then
		echo "fakeapi not ready" >&2
		exit 1
	fi
	sleep 1
done

fail=0

check() {
	# check <scenario-name> <needle> <actual-output>
	name="$1"
	needle="$2"
	actual="$3"
	case "$actual" in
	*"$needle"*)
		echo "PASS: $name"
		;;
	*)
		echo "FAIL: $name (output: $actual)" >&2
		fail=1
		;;
	esac
}

out=$(/tg-notify --token TESTTOKEN --chat-id 123 --base-url "$FAKEAPI_URL" \
	--proxy "$HTTP_PROXY_URL" --no-retry "compose smoke" 2>&1)
check "message via HTTP proxy" "Sent (message_id: 42)" "$out"

out=$(/tg-notify --token TESTTOKEN --chat-id 123 --base-url "$FAKEAPI_URL" \
	--proxy "$SOCKS5_PROXY_URL" --no-retry "compose socks" 2>&1)
check "message via SOCKS5 proxy with auth" "Sent (message_id: 42)" "$out"

out=$(/tg-notify --whoami --token TESTTOKEN --base-url "$FAKEAPI_URL" \
	--proxy "$HTTP_PROXY_URL" --no-retry 2>&1)
check "whoami via HTTP proxy" "@testbot (id: 1)" "$out"

exit "$fail"
