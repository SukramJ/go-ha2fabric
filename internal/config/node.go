// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package config

import (
	"log/slog"
	"slices"

	"thingfabric.dev/go-fabric/device"
	"thingfabric.dev/go-fabric/node"

	"thingfabric.dev/go-ha2fabric/internal/version"
)

// Product identity the node reports in BasicInformation.
const (
	VendorName  = "go-ha2fabric"
	ProductName = "Home Assistant bridge"
)

// spake2Salt is the node's configured SPAKE2+ salt (16 to 32 bytes,
// node.PBKDFSaltMin..Max). Since go-fabric v0.23.0 it answers no
// commissioning window: the node's own and an administrator's basic
// window each draw a fresh 32-byte salt when they open, as matter.js
// (go-fabric CHANGELOG 0.23.0, review II decision 251; node.Commissioning
// Salt's godoc at v0.25.0). node.Config.Validate still checks it — a salt
// outside the bounds fails (node/config.go:960-969 at v0.27.0) — so it
// stays.
var spake2Salt = []byte("go-ha2fabric-spake2p-salt")

// spake2Iterations is the verifier's PBKDF2 iteration count, the value of
// go-fabric's cookbook daemon (go-zendure2mqtt internal/fabric).
const spake2Iterations = 1000

// Node is the node's configuration from the daemon's (go-fabric's
// cookbook "Embed in a daemon", §2): the listen address, the
// commissioning values, the identity with the daemon's version, the store
// file, the mDNS interfaces (MDNS_INTERFACES), the daemon's logger and
// the health probe; everything else is
// go-fabric's default. With the camera on, the node also accepts Matter
// over TCP: a camera's signals carry an SDP, which rarely fits one UDP
// message, and the node sends them over the controller's TCP session
// (go-fabric docs/guides/camera.md:137-144, bridge/tcp_listen.go:18-31 at
// v0.27.0). The configured node and every registry node get theirs here.
func (c *Config) Node(logger *slog.Logger) node.Config {
	return node.Config{
		Network: node.Network{
			Listen: c.Listen,
			TCP:    node.TCPConfig{Incoming: c.Camera},
			MDNS:   node.MDNS{Interfaces: slices.Clone(c.MDNSInterfaces)},
		},
		Commissioning: node.Commissioning{
			Passcode:        c.Passcode,
			Discriminator:   c.Discriminator,
			PBKDFIterations: spake2Iterations,
			Salt:            spake2Salt,
		},
		// The commissionable advertisement names the node an Aggregator
		// (owner, 2026-11-22): go-fabric infers DT from the first bridged
		// device otherwise (node.ProductDescription.DeviceType, v0.23.0), so
		// a commissioner's icon would follow whichever entity came first.
		ProductDescription: node.ProductDescription{DeviceType: uint16(device.AggregatorType().ID), VendorID: c.VendorID, ProductID: c.ProductID},
		BasicInformation: node.BasicInformation{
			VendorName:            VendorName,
			ProductName:           ProductName,
			NodeLabel:             c.Label,
			SoftwareVersion:       version.Software(),
			SoftwareVersionString: version.Version,
		},
		Store:  node.Store{Path: c.StorePath},
		Health: &node.Health{Interval: node.DefaultProbeInterval},
		Logger: logger,
	}
}
