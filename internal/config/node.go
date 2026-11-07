// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package config

import (
	"log/slog"

	"github.com/SukramJ/go-fabric/node"

	"github.com/SukramJ/go-ha2fabric/internal/version"
)

// Product identity the node reports in BasicInformation.
const (
	VendorName  = "go-ha2fabric"
	ProductName = "Home Assistant bridge"
)

// spake2Salt is the SPAKE2+ salt of the node's passcode verifier (16 to 32
// bytes, node.PBKDFSaltMin..Max); it is not a secret, the passcode is.
var spake2Salt = []byte("go-ha2fabric-spake2p-salt")

// spake2Iterations is the verifier's PBKDF2 iteration count, the value of
// go-fabric's cookbook daemon (go-zendure2mqtt internal/fabric).
const spake2Iterations = 1000

// Node is the node's configuration from the daemon's (go-fabric's
// cookbook "Embed in a daemon", §2): the listen address, the
// commissioning values, the identity with the daemon's version, the store
// file, the daemon's logger and the health probe; everything else is
// go-fabric's default. With the camera on, the node also accepts Matter
// over TCP: a camera's signals carry an SDP, which rarely fits one UDP
// message, and the node sends them over the controller's TCP session
// (go-fabric docs/guides/camera.md:121-128, bridge/tcp_listen.go:20-31 at
// v0.15.5). The configured node and every registry node get theirs here.
func (c *Config) Node(logger *slog.Logger) node.Config {
	return node.Config{
		Network: node.Network{Listen: c.Listen, TCP: node.TCPConfig{Incoming: c.Camera}},
		Commissioning: node.Commissioning{
			Passcode:        c.Passcode,
			Discriminator:   c.Discriminator,
			PBKDFIterations: spake2Iterations,
			Salt:            spake2Salt,
		},
		ProductDescription: node.ProductDescription{VendorID: c.VendorID, ProductID: c.ProductID},
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
