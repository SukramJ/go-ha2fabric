# The held reports of phases 0–2, published

Decisions 21, 37 and 53 (`../decisions.md`). These reports were kept out of
the repository while the items they describe were open. They are published
unchanged, with phase 3's pull request, after the `0.7.0` tag.

| Row | Status at publication |
| --- | --- |
| HR-P0-auth-20, HR-P0-auth-50, HR-P0-auth-51, HR-P0-word-52 | fixed in phase 0's pull request (#69), shipped in `0.7.0` |
| HR-P2-data-30 | already published by decision 36 |
| HR-P1-auth-01 | **open, documented** (decision 37 as amended by the owner on 2026-11-25: docs only, no Supervisor issue). The paragraph on co-installed add-ons is in `docs/operations.md`; decision 54's bind keeps LAN peers off the listener, not another add-on on the Supervisor network |

HR-P1-auth-01, what was measured after phase 2 (2026-11-25, the owner's
production HAOS and a dedicated test installation, owner-authorised):
every precondition holds on a real HAOS — other add-ons run with
`NET_RAW` on the Supervisor's `hassio` network, which they share at layer 2
with the Supervisor's address; `rp_filter` is loose; the Supervisor's
Ingress proxy forwards the user headers and no secret the add-on could
verify. On the test installation a container on that network took over the
Supervisor's address by ARP and received the add-on's replies meant for
the Supervisor: **the interception was measured; the full forged exchange
was not completed against the live Supervisor** (the real Supervisor's
traffic raced the spoofed connection). Confidence that the weakness is
real: high. The fix needs a verifiable secret from the Supervisor; this
repository cannot add one.

The fuzz seeds of `phase-0-fuzz-seeds/` are in the tree since phase 0
(`internal/ha/testdata/fuzz/FuzzHTTPBase/`, `internal/nodes/testdata/fuzz/FuzzSlug/`).
