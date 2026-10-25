#!/usr/bin/with-contenv bashio
# SPDX-License-Identifier: Apache-2.0
# Copyright (C) 2026 SukramJ.
#
# Home Assistant add-on entrypoint for go-ha2fabric: maps the add-on options
# (/data/options.json, read with bashio) onto the daemon's environment and
# exec's the daemon, which becomes PID 1 and receives the signals.
# SUPERVISOR_TOKEN is already in the environment (with-contenv); the daemon
# reads it, connects to ws://supervisor/core/websocket, and never logs or
# stores it.
set -e

bashio::log.info "Starting go-ha2fabric add-on..."

# --- The Matter node ---
export LISTEN=":$(bashio::config 'port')"
export PASSCODE="$(bashio::config 'passcode')"
export DISCRIMINATOR="$(bashio::config 'discriminator')"
export VENDOR_ID="$(bashio::config 'vendor_id')"
export PRODUCT_ID="$(bashio::config 'product_id')"
export LABEL="$(bashio::config 'label')"
# The node's store (fabrics, credentials, endpoint numbers) on the add-on's
# persistent /data: losing it unpairs the bridge.
export STORE_PATH="/data/ha2fabric.db"

# --- Exposure ---
export EXPOSE_ASSIST="$(bashio::config 'expose_assist')"
# Lists arrive one item per line; the daemon splits on newlines and commas.
INCLUDE=""
if bashio::config.has_value 'include'; then
  INCLUDE="$(bashio::config 'include')"
fi
export INCLUDE
EXCLUDE=""
if bashio::config.has_value 'exclude'; then
  EXCLUDE="$(bashio::config 'exclude')"
fi
export EXCLUDE

export LOG_LEVEL="$(bashio::config 'log_level')"

# --- Web UI through Ingress ---
# On the host's network: listen on ingress_port and serve only the Ingress
# proxy (172.30.32.2).
export WEB_LISTEN="0.0.0.0:8486"
export WEB_ALLOW="172.30.32.2"

bashio::log.info "Matter node on UDP ${LISTEN}, store ${STORE_PATH}; web UI through Ingress."

exec /usr/bin/ha2fabric
