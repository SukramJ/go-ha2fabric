// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package config

import (
	"io"
	"net"
	"slices"
	"strings"
	"testing"

	"thingfabric.dev/go-fabric/node"
)

// FuzzLoad holds Load against any value of the list and number variables
// (the add-on's options and the stand-alone environment are operator
// input): it never panics, and what it accepts is in range — a valid
// passcode or none, a 12-bit discriminator, non-zero ids, a node limit within
// bounds, a label of at most 32 bytes, IP addresses in WEB_ALLOW, and list
// items that are non-empty, trimmed and, for MDNS_INTERFACES, unique.
func FuzzLoad(f *testing.F) {
	f.Add("light.*, switch.a", "sensor.x", "Living room,\nKitchen", "", "172.30.32.2", "ha.local", "eth0\nwlan0", "20202021", "3840", "0xFFF1", "0x8001", "5", "Bridge")
	f.Add("", "", "", ",,", "::1, 127.0.0.1", "", "", "12345678", "0", "1", "65535", "0", "x")
	f.Add("[", "*", " , ", "a,,b", "not-an-ip", "http://x:1/", "eth0,eth0", "11111111", "4096", "0", "0x10000", "-1", strings.Repeat("ü", 17))
	f.Add("a\tb", "\n", "\x00", "", "", "*.local", ",", "99999999", "0x0FFF", "0b1", "1e3", "999", " ")
	f.Fuzz(func(t *testing.T, include, exclude, includeLabels, excludeLabels, allow, hosts, mdns, passcode, discr, vendor, product, limit, label string) {
		env := MapEnv(map[string]string{
			"HA_URL": "ws://ha.local:8123/api/websocket", "HA_TOKEN": "t",
			"INCLUDE": include, "EXCLUDE": exclude, "INCLUDE_LABELS": includeLabels, "EXCLUDE_LABELS": excludeLabels,
			"WEB_ALLOW": allow, "WEB_ALLOWED_HOSTS": hosts, "MDNS_INTERFACES": mdns,
			"PASSCODE": passcode, "DISCRIMINATOR": discr, "VENDOR_ID": vendor, "PRODUCT_ID": product,
			"NODE_LIMIT": limit, "LABEL": label,
		})
		c, err := Load(nil, env, io.Discard)
		if err != nil {
			return
		}
		// 0 is unset: the daemon's random passcode.
		if c.Passcode != 0 && node.ValidateSetupPIN(c.Passcode) != nil || c.Discriminator > node.MaxDiscriminator || c.VendorID == 0 || c.ProductID == 0 {
			t.Fatalf("accepted passcode %d, discriminator %d, ids %#x %#x", c.Passcode, c.Discriminator, c.VendorID, c.ProductID)
		}
		if c.NodeLimit < 0 || c.NodeLimit > MaxNodeLimit {
			t.Fatalf("accepted node limit %d", c.NodeLimit)
		}
		if c.Label == "" || len(c.Label) > 32 {
			t.Fatalf("accepted label %q", c.Label)
		}
		for _, a := range c.WebAllow {
			if net.ParseIP(a) == nil {
				t.Fatalf("accepted WEB_ALLOW %q", a)
			}
		}
		for _, l := range [][]string{c.Include, c.Exclude, c.IncludeLabels, c.ExcludeLabels, c.WebAllowedHosts, c.MDNSInterfaces} {
			for _, x := range l {
				if x == "" || x != strings.Trim(x, " \t\n") || strings.ContainsAny(x, ",\n") {
					t.Fatalf("accepted list item %q", x)
				}
			}
		}
		for i, x := range c.MDNSInterfaces {
			if slices.Contains(c.MDNSInterfaces[:i], x) {
				t.Fatalf("accepted MDNS_INTERFACES %q twice", x)
			}
		}
		for _, h := range c.WebAllowedHosts {
			if allowedHost(h) != "" {
				t.Fatalf("accepted WEB_ALLOWED_HOSTS %q", h)
			}
		}
	})
}
