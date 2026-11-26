# Phase 0 — verification, held rows

Untracked (`held/`). Published with the pull request that carries the
fix. Marks and prefixes as in `notes/review/2026-11-23/phase-0-verify.md`.
Supervisor files read with `gh api … -H 'Accept: application/vnd.github.raw'`
at the ref named. Nothing was executed.

## HR-P0-auth-20 — confirmed, medium; three corrections

### What holds (M)

- The daemon's administrator in the add-on is the peer address and
  nothing else: `cmd/ha2fabric/main.go:194-197`,
  `internal/web/web.go:136-143`, `internal/config/config.go:63`; go-fabric-ui
  adds only `X-Fabric-UI: 1` (`ui:fabricui/handler.go:778-779`). No header
  about the user is read (grep `X-Remote`, `X-Forwarded`, `X-Ingress`,
  `X-Hass` over non-test Go: none).
- Core lets a non-admin create an Ingress session and read an add-on's
  info: `core:homeassistant/components/hassio/websocket_api.py:50-59`,
  `:126-129`, `:135-139` (the user id is attached), `:157-160` (only
  `options` removed). The Ingress view needs no Core authentication:
  `core:homeassistant/components/hassio/ingress.py:62-67`.
- The add-on info carries the Ingress entry with the token — the author's
  inferred link, now read: `sup:supervisor/api/apps.py:301` and
  `sup:supervisor/apps/app.py:635-639` at `fe91b8db`
  (`/api/hassio_ingress/<ingress_token>`); the same at `2025.06.0`
  (`supervisor/api/addons.py:273`, `supervisor/addons/addon.py:454-458`)
  and `2026.10.1` (`api/apps.py:299`, `apps/app.py:572-576`).
- `panel_admin` defaults to true (`sup:supervisor/apps/validate.py:492` at
  `fe91b8db`; `supervisor/addons/validate.py:347` at `2025.06.0`); the
  manifest does not set it (`addon/config.yaml:43-46`).
- At the pin the Supervisor refuses a non-admin when `panel_admin` is
  true: `sup:supervisor/api/ingress.py:189-197`; `is_admin` is owner or
  group `system-admin` (`sup:supervisor/const.py:625`, `:646-651`).
- Before that change the handler has no check of the user at all:
  `supervisor/api/ingress.py:140-166` at `2026.10.0` (session cookie,
  then proxy). The author's second inferred link ("no equivalent check
  elsewhere before") holds for the two places a request passes: Core's
  view (above) and this handler. Core's panel registration
  (`require_admin`) hides the panel in the frontend and is not on the
  request path (I: frontend behaviour not read).

### Corrections

1. **The boundary is Supervisor 2026.10.2, not 2026.11.0.** The check is
   commit `bf71ccb5` "Enforce panel_admin for ingress sessions (#7308)",
   2026-10-13. `grep -c 'panel_admin and'` on `supervisor/api/ingress.py`:
   `2025.06.0` 0, `2026.03.1` 0, `2026.06.0` 0, `2026.09.0` 0, `2026.10.0`
   0, `2026.10.1` 0, **`2026.10.2` 1**, `2026.10.3` 1, `2026.10.4` 1,
   `2026.10.5` 1, `2026.11.0` 1, `fe91b8db` 1 (M). The author tested
   `2026.10.0` and `2026.11.0` only. The exposed population is
   installations whose Supervisor is older than about six weeks, which
   means auto-update off or the host offline.
2. **`X-Remote-User-Groups` exists from 2026.10.4** (commit `3fbb083b`
   "Forward the Core user's groups to apps over ingress (#7398)",
   2026-10-21): the header constant appears in `ingress.py` at `2026.10.4`,
   `2026.10.5`, `2026.11.0`, `fe91b8db` and not at `2026.10.3` or before
   (M, grep). At the pin: set at `:412-419`, stripped from the browser's
   request at `:47-64`, `:406-408`; the value is `effective_group_ids`
   (`sup:supervisor/const.py:653-659`, owner folded into `system-admin`).
3. **On a Supervisor before 2026.10.4 the browser can forge that header.**
   The old filter lists the user id, name and display name only
   (`supervisor/api/ingress.py:335-349` at `2026.10.0`), and Core's view
   strips none of the `X-Remote-User-*` headers
   (`core:homeassistant/components/hassio/ingress.py:28-37`, `:266-272`).
   So a request with `X-Remote-User-Groups: system-admin` from a non-admin
   reaches the add-on unchanged on exactly the Supervisors that lack the
   `panel_admin` check (M for the two filters; the end-to-end path is I,
   not run). **The author's option 1 alone therefore does not close the
   hole on an old Supervisor**, whichever way "header absent" is decided:
   the attacker supplies it.

### Severity

Medium holds, at its lower edge. For: an authenticated non-admin becomes
the bridge's administrator (pairing codes and with them a standing fabric,
erase, the node registry, the log), and the daemon's comments and docs
state an invariant it does not hold (`main.go:190-193`,
`web.go:132-135`, `docs/operations.md:314-315`). Against: it needs a Home
Assistant login and a Supervisor that has not updated itself since
mid-October 2026; no LAN peer gets anything. Not high. The path was not
run (decision 12); the owner's live test with a non-admin user on the
installed Supervisor is the measurement.

### Fix shape (recommendation: both, plus a version gate)

1. `Admin` in add-on mode = peer is `172.30.32.2` **and**
   `X-Remote-User-Groups` (split on `,`, exact match) contains
   `system-admin`; header absent → not admin (fail closed). This makes
   the daemon state its own rule and survives a later `panel_admin: false`.
2. **Minimum Supervisor 2026.10.4, checked by the daemon**, because of
   correction 3: at start in add-on mode read the Supervisor's version
   from `GET http://supervisor/info` with `SUPERVISOR_TOKEN` — an app's
   token passes that path without `hassio_api`
   (`sup:supervisor/api/middleware/security.py:107-116`, `:447-451`; the
   answer carries the version, `sup:supervisor/api/root.py:40-44`; route
   `sup:supervisor/api/__init__.py:498`). Below 2026.10.4 the add-on
   serves the status page only (`web.ReadOnly`) with a `Warning` that
   names the required version. Without this gate the header check is
   decoration on the versions that need it. (I: that the call works from
   the add-on's network as read; for the live test.)
3. `panel_admin: true` explicit in `addon/config.yaml`; the minimum
   Supervisor version in `addon/DOCS.md`; the three comments and
   `docs/operations.md:314-315` corrected to say who decides
   "administrator".

If the owner does not want the daemon to call the Supervisor: 1 and 3,
with the docs saying plainly that a Supervisor before 2026.10.2 lets any
Home Assistant user administer the bridge — that leaves the hole open
there and is the weaker choice.

### Contract

Only this host. go-fabric-ui's `Options.Admin` is the host's predicate
over the request (`ui:docs/embedding.md:542-546`, "for example by the
proxy's source address"); no API or behaviour of go-fabric-ui changes. A
sentence there that the proxy's authentication is not an administrator
check would be a docs issue for go-fabric-ui, not a dependency of this
fix. For this host the behaviour changes: a non-admin user (on any
Supervisor) and every user on a Supervisor before 2026.10.4 see the
status page only — a `changelog.md` line and the DOCS.

### For the other rows

No other phase 0 finding is judged a still-open weakness that needs
holding beyond what the author decided: 21, 22 and 23 rest on conditions
the docs already publish.
