// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package bridge

import (
	"context"
	"log/slog"

	"github.com/SukramJ/go-fabric/device"
	hamapping "github.com/SukramJ/go-hamapping"
	"github.com/SukramJ/go-hamqtt/model"

	"github.com/SukramJ/go-ha2fabric/internal/camera"
)

// ports is go-hamapping's Options.Ports: a camera entity's host, made on
// the first ask and kept per entity id; nil for every other entity, and
// for a camera while the option is off.
func (b *Bridge) ports(e model.Entity) any {
	if !b.opts.Camera || b.opts.CameraUpstream == nil || e.Platform() != "camera" {
		return nil
	}
	if h := b.hosts[e.Key()]; h != nil {
		return h
	}
	if b.hosts == nil {
		b.hosts = map[string]*camera.Host{}
	}
	h := camera.New(e.Key(), b.opts.CameraUpstream, camera.Options{Logger: b.log})
	b.hosts[e.Key()] = h
	return h
}

// dryHost stands in for a camera's host in a plan without a node: Map
// only checks that the ports exist.
var dryHost = camera.New("", nil, camera.Options{})

// dryPorts is Options.Ports for Dry.
func dryPorts(cameras bool) func(model.Entity) any {
	return func(e model.Entity) any {
		if cameras && e.Platform() == "camera" {
			return dryHost
		}
		return nil
	}
}

// attachCameras attaches each camera host to the *camera.Provider of the
// device the plan built on its ports. A camera device without one is a
// bug in the plan or the kit: logged, and its host stays detached.
func (b *Bridge) attachCameras(g *group, plan *hamapping.Plan, devs []*device.Device) {
	for i := range plan.Endpoints {
		h := b.hosts[plan.Endpoints[i].Key]
		if h == nil || i >= len(devs) {
			continue
		}
		if err := h.AttachDevice(devs[i]); err != nil {
			b.log.Error("bridge.camera_unattached", slog.String("entity", h.Entity()), slog.String("row", plan.Endpoints[i].Row), slog.String("error", err.Error()))
			continue
		}
		if g.cameras == nil {
			g.cameras = map[string]bool{}
		}
		g.cameras[h.Entity()] = true
	}
}

// closeCameras closes the hosts no built device carries any more: the
// entity left the bridge, its row changed, or its build failed.
func (b *Bridge) closeCameras(ctx context.Context) {
	for id, h := range b.hosts {
		kept := false
		for _, g := range b.groups {
			kept = kept || g.cameras[id]
		}
		if !kept {
			h.Close(ctx)
			delete(b.hosts, id)
		}
	}
}
