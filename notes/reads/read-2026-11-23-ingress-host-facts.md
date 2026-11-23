# Read 2026-11-23 — Ingress: which Host/Origin reaches the add-on

Pins read: home-assistant/core tag `2026.10.3` (1f214b84, `git show 2026.10.3:…`, checkout untouched);
home-assistant/supervisor `main` @ fe91b8db (2026-11-19, shallow clone; latest release tag 2026.11.0 — not diffed
against main); aiohttp 3.14.5 (pinned by both: core `requirements.txt:10`, supervisor `requirements.txt:3`);
go-fabric-ui v0.13.0 (module cache). go-ha2fabric `go.mod:7` still pins v0.9.2.

## Hop 1 — core `homeassistant/components/hassio/ingress.py` @ 2026.10.3
- Upstream is `http://{supervisor host}/ingress/{token}/{path}` (:73, :78-81), sent with `headers=source_header`
  (:177-180; WebSocket :138, :149-151).
- `_init_header` copies **every** browser header except `INIT_HEADERS_FILTER` (:268-272). The filter (:28-37) holds only
  Content-Length, Content-Encoding, Transfer-Encoding, Accept-Encoding and four Sec-WebSocket-*. **`Host`, `Origin`,
  `Cookie` are not in it → forwarded unchanged.**
- Added/overwritten: `X-Hass-Source: core.ingress` (:274), `X-Ingress-Path: /api/hassio_ingress/{token}` (:275),
  `X-Forwarded-For` = existing + peer IP (:278-284, :260-263), `X-Forwarded-Host` = existing, else `request.host`
  (:287-289), `X-Forwarded-Proto` = existing, else `request.scheme` (:292-295).

## Hop 2 — supervisor `supervisor/api/ingress.py` @ fe91b8db
- Route `/ingress/{token}/{path:.*}` (`api/__init__.py:862`), session cookie required (:181-184); upstream URL
  `http://{app.ip_address}:{app.ingress_port}/{path}` (:125-127), sent with `headers=source_header` (:298-301).
- `_init_header` (:400-427) copies every header except `_REQUEST_HEADERS_FILTER` (:47-64: Content-Length/-Encoding,
  Transfer-Encoding, Sec-WebSocket-*, Supervisor token headers, X-Remote-User-*). **`Host` and `Origin` are not filtered
  → forwarded.** Adds `X-Remote-User-*` (:412-419), appends its peer to `X-Forwarded-For` (:422-425);
  `X-Forwarded-Host`/`-Proto` and `X-Ingress-Path` pass through from core untouched.
- aiohttp keeps a caller-supplied Host: `ClientRequest.update_headers` sets Host from the URL, then a `Host` in
  `headers` overwrites it (3.14.5 `client_reqrep.py:1276`, :1285-1287). Confirmed by running a local 3.14.5
  client→server: `headers={"Host": "homeassistant.local:8123"}` arrived as the single Host; without it, the URL's.
- Source address: Supervisor is `DOCKER_IPV4_NETWORK_MASK[2]` of 172.30.32.0/23 = 172.30.32.2 (`const.py:69`,
  `docker/network.py:188-190`). The add-on is `host_network: true` (`addon/config.yaml:37`), so `app.ip_address` is the
  gateway 172.30.32.1 (`docker/app.py:188-191`): Supervisor dials `172.30.32.1:8486`.

## What arrives at the add-on (browser at `https://homeassistant.local:8123`)
- `Host: homeassistant.local:8123` — the browser's, **not** `172.30.32.1:8486`. High (both hops + aiohttp run).
- `Origin: https://homeassistant.local:8123` when the browser sends one (writes; not a same-origin GET). High.
- `X-Forwarded-Host: homeassistant.local:8123`, `X-Forwarded-Proto: https` (or a front proxy's values),
  `X-Forwarded-For: <browser>, <core as Supervisor sees it>`, `X-Ingress-Path: /api/hassio_ingress/<token>`. High.
- `RemoteAddr 172.30.32.2:<port>`. Medium-high: the address is code; no NAT on the bridge→gateway path is inferred,
  backed by the shipped `WEB_ALLOW=172.30.32.2` (`docs/operations.md:86-87`).

## Verdict for go-fabric-ui v0.13.0
- Host check (`fabricui/host.go:64-76`): `homeassistant.local` is no loopback name, IP or `AllowedHosts` entry (:49-60);
  without `AllowHost` every request, bundle included, is 403 `host_not_allowed` (:89-95). With
  `AllowHost: web.From("172.30.32.2")` (`internal/web/web.go:126`, compares `r.RemoteAddr`) it is admitted
  `byPredicate` (:72-73). **Passes** — high, given the source address.
- Origin check on admin routes (`fabricui/handler.go:759-773`): the Origin's name fails the name rules (:768), then
  `byPredicate && EqualFold(u.Host, r.Host)` (:771-772): both `homeassistant.local:8123`. **Passes** — high. This is
  `docs/embedding.md:525-531` (second kind of proxy). `AllowHost` alone suffices; the Supervisor does not rewrite Host.
  Reached by IP, Host and Origin pass by the IP name rule instead (`host.go:56-58`, `handler.go:768`). High.
- **Edge, fails:** a reverse proxy in front of HA that rewrites `Host` makes core forward the rewritten one. Rewritten
  to an IP: Host admitted by name (`byPredicate` false), the browser's DNS-name Origin refused → 403
  `origin_not_allowed` on admin routes, non-admin answers on open ones (:753). Rewritten to another name: Origin != Host,
  same refusal. fabric-ui never reads `X-Forwarded-Host`. Medium-high (code reading; no proxy tested).

## The live test must check
1. `r.RemoteAddr` is 172.30.32.2 with `host_network: true` on the installed HAOS/Supervisor (log it once).
2. `Host` and `Origin` of one admin write through the Ingress panel are the same host:port, and it answers 2xx; also
   on the default port (`https://name` on 443: Origin has no port — Host must have none either).
3. Nabu Casa remote URL and any front reverse proxy: Host forwarded unchanged (edge above)? Installed Supervisor
   release against `main` @ fe91b8db: `_init_header` unchanged?
