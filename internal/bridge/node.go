// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package bridge

import (
	"log/slog"

	"github.com/SukramJ/go-fabric/node"

	"github.com/SukramJ/go-ha2fabric/internal/config"
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

// NodeConfig is the node's configuration from the daemon's (cookbook §2):
// the listen address, the commissioning values, the identity with the
// daemon's version, the store file, the daemon's logger and the health
// probe; everything else is go-fabric's default.
func NodeConfig(c *config.Config, logger *slog.Logger) node.Config {
	return node.Config{
		Network: node.Network{Listen: c.Listen},
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
		Health: node.Health{Interval: node.DefaultProbeInterval},
		Logger: logger,
	}
}

// LogPairing writes the pairing codes to the log (cookbook §7).
func LogPairing(n *node.Node, logger *slog.Logger) {
	qr, manual, err := n.PairingCodes()
	if err != nil {
		logger.Warn("ha2fabric.pairing_codes", slog.String("error", err.Error()))
		return
	}
	logger.Info("ha2fabric.pairing",
		slog.String("manual_code", manual), slog.String("qr_payload", qr),
		slog.String("hint", "pair from a Matter controller with the manual code or the QR code in the web UI; the commissioning window is open while no controller has paired"))
}
