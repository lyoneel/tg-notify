# Manual proxy e2e runbook

Release-checklist rehearsal that sends tg-notify through a real
third-party proxy to a real Telegram bot, and verifies both sides.
This is manual and never runs in CI; the hermetic automated suite
(`make test`, `make e2e-docker`) covers the same code paths.

## Purpose

- Prove `--proxy`/`TELEGRAM_PROXY` works against production-grade
  proxies (squid, microsocks), not just the in-repo test doubles.
- Rehearse the proxy feature before a release, end to end.

## Prerequisites

- A real bot token from @BotFather.
- A chat ID: message your bot once, then run
  `tg-notify --discover-chat-id` (no proxy needed).
- A throwaway host (VPS, VM, or local docker) for the proxy. Never
  run these proxies exposed to the public internet.

## Option A: HTTP proxy (squid)

1. Start squid with a test-only config.

   ```bash
   docker run -d --name tg-squid -p 3128:3128 \
     -v "$PWD/squid.conf:/etc/squid/squid.conf:ro" \
     ubuntu/squid:latest
   ```

   Minimal `squid.conf` (test-only, world-open on purpose):

   ```text
   http_port 3128
   http_access allow all
   access_log stdio:/var/log/squid/access.log squid
   cache deny all
   visible_hostname tg-notify-e2e
   ```

   Without docker: `apt install squid` or `pacman -S squid`, then
   start the service with the same directives added to the distro
   config.

2. Verify the token through the proxy:

   ```bash
   tg-notify --whoami -P http://127.0.0.1:3128
   ```

3. Send a message:

   ```bash
   tg-notify -P http://127.0.0.1:3128 "manual proxy e2e"
   ```

4. Check both sides:

   - The message arrives in Telegram.
   - The proxy saw the traffic:

     ```bash
     docker logs tg-squid            # image logs squid output
     docker exec tg-squid cat /var/log/squid/access.log
     ```

     Expect lines referencing `api.telegram.org`.

## Option B: SOCKS5 proxy (microsocks)

1. Start microsocks with authentication:

   ```bash
   # apt install microsocks / pacman -S microsocks
   microsocks -i 127.0.0.1 -p 1080 -u socksuser -P sockspass &
   ```

   microsocks logs each accepted connection to stderr; keep the
   terminal visible.

2. Verify and send:

   ```bash
   tg-notify --whoami -P socks5://socksuser:sockspass@127.0.0.1:1080
   tg-notify -P socks5://socksuser:sockspass@127.0.0.1:1080 "manual socks e2e"
   ```

3. Check both sides: the message arrives in Telegram, and microsocks
   printed connection lines for the sends.

## Security notes

- Bind test proxies to loopback or a private interface; never leave a
  world-open squid/microsocks reachable from the internet.
- Revoke and recreate the test bot token via @BotFather afterwards.
- squid's access.log records requests but not response bodies;
  response-side assertions live in the automated Go e2e suite.

## Known limitations

- This runbook exercises `http` and `socks5` proxies. `https` proxies
  are covered by the automated suite (TLS recording proxy with the
  `SetProxyTLSRootCAs` seam); a manual https proxy would require
  installing the proxy's CA into the system trust store.
