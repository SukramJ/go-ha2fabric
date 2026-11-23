// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SukramJ/go-ha2fabric/internal/config"
)

// TestRights: the add-on admits and administers by the Ingress proxy's
// address; stand-alone no Host predicate (names only), the status only by
// default, the UI's token mode with WEB_ADMIN.
func TestRights(t *testing.T) {
	proxy := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	proxy.RemoteAddr = config.IngressProxy + ":4000"
	lan := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	lan.RemoteAddr = "192.168.1.20:4000"

	addon := &config.Config{Addon: true}
	if h := allowHost(addon); h == nil || !h(proxy) || h(lan) {
		t.Fatal("add-on: the Host check does not admit the Ingress proxy alone")
	}
	if a := admin(addon); a == nil || !a(proxy) || a(lan) {
		t.Fatal("add-on: the administrator is not the Ingress proxy alone")
	}
	alone := &config.Config{}
	if allowHost(alone) != nil {
		t.Fatal("stand-alone: a Host predicate")
	}
	if a := admin(alone); a == nil || a(proxy) || a(lan) {
		t.Fatal("stand-alone without WEB_ADMIN: an administrator")
	}
	if admin(&config.Config{WebAdmin: true}) != nil {
		t.Fatal("WEB_ADMIN: not the token mode")
	}
}
