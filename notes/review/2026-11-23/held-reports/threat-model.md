# Threat model, phase 0 — held rows

Untracked (`.git/info/exclude`: `held/`). Published with the pull request
that carries the fix. Baseline `10ee7b4`; marks and prefixes as in
`notes/review/2026-11-23/threat-model.md`. Nothing was executed.

## HR-P0-auth-20 — medium — the add-on's administrator is any Home Assistant user on a Supervisor before 2026.11.0

Role R6, threat T4.

**Claim.** In the add-on the web UI's administrator is "the peer is
`172.30.32.2`" and nothing else. Whether the Home Assistant user behind the
Ingress request is an administrator is decided by the Supervisor alone, and
only from Supervisor `2026.11.0` on. On an older Supervisor a non-admin
Home Assistant user reaches every admin route of the bridge: the pairing
codes (and with them a fabric of their own on the bridge), the selection,
fabric removal, erase, the node registry, the log.

**Lines.**

| Step | Line | M/I |
| --- | --- | --- |
| The daemon's admin test is the peer address | `cmd/ha2fabric/main.go:196-197`; `internal/web/web.go:136-143`; `internal/config/config.go:63` | M |
| It reads no header about the user (grep `X-Remote-User`, `X-Forwarded`, `X-Ingress`, `X-Hass` over non-test Go: none) | — | M |
| The comments and the docs equate "authenticated" with "administrator" | `main.go:190-193`; `web.go:132-135`; `docs/operations.md:314-315` | M |
| Core lets a non-admin create an Ingress session and read an add-on's info | `core:homeassistant/components/hassio/websocket_api.py:52-59`, `:126-129` (`WS_NO_ADMIN_ENDPOINTS`: `/ingress/session`, `/ingress/validate_session`, `/addons/<slug>/info`); `:157-160` strips only `options` for a non-admin | M |
| Core's Ingress view asks for no Core authentication | `core:homeassistant/components/hassio/ingress.py:62-67` (`requires_auth = False`) | M |
| The Supervisor at the pin refuses a non-admin when the add-on's `panel_admin` is true | `sup:supervisor/api/ingress.py:189-197` at `fe91b8db` | M |
| `panel_admin` defaults to true; this manifest does not set it | `sup:supervisor/apps/validate.py:492` at `fe91b8db`; `addon/config.yaml` (grep `panel_admin`: none) | M |
| That refusal exists at tag `2026.11.0` and not at `2026.10.0`, `2026.06.0`, `2026.01.0`, `2025.06.0` | `grep -c 'panel_admin and'` on `supervisor/api/ingress.py` at each tag: 1, 0, 0, 0, 0; at `2026.10.0` the file's only `admin` lines are the import and the panel list (`:20`, `:104`), no `HTTPForbidden` | M for the grep; that no equivalent check sat elsewhere before is I |
| The add-on info a non-admin can read carries the Ingress entry with the add-on's token | not read at a pin | I |
| The Supervisor at the pin hands the user to the add-on: id, name, display name and **groups** | `sup:supervisor/api/ingress.py:412-419` (`X-Remote-User-Groups`); it strips the same headers from the browser's request (`:47-64`, `:407-408`) | M |

**Path (I, not run).** A non-admin user's frontend session calls
`supervisor/api` with `/addons/<slug>/info` and `/ingress/session`, sets
the session cookie and requests `/api/hassio_ingress/<token>/api/v1/node/
pairing` with `X-Fabric-UI: 1`. Before 2026.11.0 the Supervisor forwards
it; the daemon sees `172.30.32.2` and answers as to an administrator.

**Why medium, not high.** The Supervisor updates itself, so the condition
is an installation that has not taken `2026.11.0` (released this month;
`main` at the pin was not diffed against the tag beyond this file). A
non-admin Home Assistant user can already call most services in Core; what
the path adds is the pairing code (a standing fabric on the bridge that
outlives the user), erase, and the selection. No LAN peer without a Home
Assistant login gets anything from it.

**Options for the wizard.**

1. **Second factor from the Supervisor's headers** (recommended; the best
   design, because the daemon then states its own rule instead of
   inheriting one): in add-on mode `Admin` requires the peer address
   **and** `X-Remote-User-Groups` to contain `system-admin`. The Supervisor
   strips these headers from the browser's request and sets its own, so
   they are as trustworthy as the peer address. Needs the group id and the
   header's presence confirmed at the Supervisor versions the add-on
   supports (the header may be younger than the user-id one); where the
   header is absent the choice is refuse (safe, breaks old Supervisors) or
   admit (today's behaviour).
2. **State the requirement**: `panel_admin: true` explicit in
   `addon/config.yaml`, a minimum Supervisor version in `addon/DOCS.md`,
   and the comments and `docs/operations.md:314-315` corrected to say who
   decides "administrator". No code; leaves old Supervisors open.
3. Both: 2 now, 1 with the phase 1 fix pull request.

**Read further.** Phase 1 S (the gate), X1 (the Ingress flow end to end,
the header's history in the Supervisor), the owner's live Ingress test
(a non-admin user against the installed Supervisor).

**Contract.** Yes for option 1: a non-admin Home Assistant user who today
sees the full UI would see the status page only.
