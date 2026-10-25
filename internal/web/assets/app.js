// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.
"use strict";

const $ = (id) => document.getElementById(id);
const esc = (s) => String(s ?? "").replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]);
let lastQR = "";
let plan = [];

async function get(path) {
  const r = await fetch(path, { cache: "no-store" });
  return r.json();
}

function renderHealth(h) {
  $("version").textContent = "Version " + h.version;
  const b = h.bridge;
  $("ha").innerHTML = b.ha_connected ? '<span class="ok">connected</span>' : '<span class="bad">not connected</span>';
  $("node").innerHTML = h.node ? (h.node.healthy ? '<span class="ok">healthy</span>' : '<span class="bad">' + esc(h.node.note) + "</span>") : "no sample yet";
  $("endpoints").textContent = b.endpoints + " endpoints from " + b.candidates + " candidate entities";
  $("sync").textContent = b.last_sync && !b.last_sync.startsWith("0001") ? new Date(b.last_sync).toLocaleString() : "not yet";
  $("commands").textContent = b.commands + " service calls, " + b.command_errors + " failed" + (b.last_command_error ? " (" + b.last_command_error + ")" : "");
  if (h.pairing) {
    $("manual").textContent = h.pairing.manual;
    if (h.pairing.qr !== lastQR) {
      lastQR = h.pairing.qr;
      $("qr").innerHTML = qrSVG(h.pairing.qr, 6) + '<p class="note">' + esc(h.pairing.qr) + "</p>";
    }
  }
  const f = h.fabrics || [];
  $("fabrics").innerHTML = f.length ? f.map((x) => "<li>" + esc(x.index) + ": fabric " + esc(x.fabric_id) + ", vendor " + esc(x.vendor_id) + (x.label ? " (" + esc(x.label) + ")" : "") + "</li>").join("") : "<li>none — not paired yet</li>";
}

function renderPlan() {
  const only = $("only").checked;
  const rows = plan.filter((v) => !only || v.exposed).map((v) =>
    "<tr><td>" + esc(v.entity_id) + (v.name ? "<small>" + esc(v.name) + "</small>" : "") + "</td>" +
    "<td>" + esc(v.device) + "</td>" +
    '<td class="' + (v.exposed ? "ok" : "") + '">' + (v.exposed ? "exposed" : "not exposed") + "<small>" + esc(v.expose_reason) + "</small></td>" +
    '<td class="' + esc(v.state) + '">' + esc(v.state ? v.state.replace("_", " ") : "") + (v.row ? "<small>" + esc(v.row) + "</small>" : "") + (v.reason ? "<small>" + esc(v.reason) + "</small>" : "") + "</td>" +
    "<td>" + (v.endpoint ? esc(v.endpoint) : "") + "</td></tr>");
  $("plan").innerHTML = rows.join("") || '<tr><td colspan="5">no entities</td></tr>';
}

async function refresh() {
  try {
    renderHealth(await get("api/health"));
    plan = await get("api/plan");
    renderPlan();
  } catch (e) {
    $("ha").textContent = "the daemon does not answer: " + e;
  }
}

$("only").addEventListener("change", renderPlan);
refresh();
setInterval(refresh, 5000);
