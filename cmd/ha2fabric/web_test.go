// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SukramJ/go-ha2fabric/internal/config"
	"github.com/SukramJ/go-ha2fabric/internal/web"
)

// TestRights: the add-on admits by the Ingress proxy's address and
// administers by the proxy's address and the user's administrator group,
// once the Supervisor's version is known to be recent enough; stand-alone
// no Host predicate (names only), the status only by default, the UI's
// token mode with WEB_ADMIN.
func TestRights(t *testing.T) {
	proxy := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	proxy.RemoteAddr = config.IngressProxy + ":4000"
	proxyAdmin := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	proxyAdmin.RemoteAddr = config.IngressProxy + ":4000"
	proxyAdmin.Header.Set(web.UserGroupsHeader, "system-users,system-admin")
	lan := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	lan.RemoteAddr = "192.168.1.20:4000"
	lan.Header.Set(web.UserGroupsHeader, "system-admin")

	addon := &config.Config{Addon: true}
	if h := allowHost(addon); h == nil || !h(proxy) || h(lan) {
		t.Fatal("add-on: the Host check does not admit the Ingress proxy alone")
	}
	g := &supervisorGate{}
	if a := admin(addon, g); a == nil || a(proxyAdmin) || a(proxy) || a(lan) {
		t.Fatal("add-on: an administrator before the Supervisor's version is known")
	}
	g.state.Store(int32(gateOpen))
	if a := admin(addon, g); a == nil || !a(proxyAdmin) || a(proxy) || a(lan) {
		t.Fatal("add-on: the administrator is not the proxy's request of an administrator user alone")
	}
	alone := &config.Config{}
	if allowHost(alone) != nil {
		t.Fatal("stand-alone: a Host predicate")
	}
	if a := admin(alone, nil); a == nil || a(proxy) || a(lan) {
		t.Fatal("stand-alone without WEB_ADMIN: an administrator")
	}
	if admin(&config.Config{WebAdmin: true}, nil) != nil {
		t.Fatal("WEB_ADMIN: not the token mode")
	}
}
