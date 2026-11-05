// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package web

import (
	"io"
	"log/slog"
	"time"

	fabricui "github.com/SukramJ/go-fabric-ui/fabricui"
	"github.com/SukramJ/go-fabric/logring"
)

// NewLogger is the daemon's logger: go-fabric's log ring in front of the
// stderr text handler, both gated by lvl — the one level source the
// Operation page ([Operations]) reads and moves. It mirrors
// go-fabric-server's internal/daemon/operation.go newLogger.
func NewLogger(stderr io.Writer, lvl *slog.LevelVar) (*slog.Logger, *logring.Handler) {
	ring := logring.New(slog.NewTextHandler(stderr, &slog.HandlerOptions{Level: lvl}), logring.Options{Level: lvl})
	return slog.New(ring), ring
}

// Operations is go-fabric-ui's Operations port (fabricui.Options.Operations)
// over the daemon's log ring: the level, the ring's records and their
// stream, and what the daemon is. A level set on the page holds until the
// daemon restarts; LOG_LEVEL is the level at start.
type Operations struct {
	ring    *logring.Handler
	version string
	started time.Time
}

var _ fabricui.Operations = Operations{}

// NewOperations is the port over ring, for the daemon of version
// (internal/version.Version) started at started.
func NewOperations(ring *logring.Handler, version string, started time.Time) Operations {
	return Operations{ring: ring, version: version, started: started}
}

// LogLevel is the level in force.
func (o Operations) LogLevel() slog.Level { return o.ring.Level() }

// SetLogLevel moves the level of the ring and of stderr together.
func (o Operations) SetLogLevel(l slog.Level) error {
	o.ring.SetLevel(l)
	return nil
}

// Log are the ring's records from since, oldest first, the newest limit
// of them when limit > 0.
func (o Operations) Log(since uint64, limit int) []fabricui.LogRecord {
	recs := o.ring.Records(since, limit)
	out := make([]fabricui.LogRecord, len(recs))
	for i, r := range recs {
		out[i] = logRecord(r)
	}
	return out
}

// ObserveLog registers fn for every record the ring takes from now on.
func (o Operations) ObserveLog(fn func(fabricui.LogRecord)) func() {
	if fn == nil {
		return func() {}
	}
	return o.ring.Observe(func(r logring.Record) { fn(logRecord(r)) })
}

// Info is the daemon's version and uptime; GoFabric is the build
// information's (fabricui.GoFabricVersion, empty when there is none).
func (o Operations) Info() fabricui.OperationInfo {
	return fabricui.OperationInfo{
		Version: o.version, GoFabric: fabricui.GoFabricVersion(),
		StartedAt: o.started, Uptime: time.Since(o.started),
	}
}

// logRecord maps a ring record one to one: the attributes as text
// (slog.Value.String) under the ring's dotted group keys.
func logRecord(r logring.Record) fabricui.LogRecord {
	attrs := make(map[string]string, len(r.Attrs))
	for _, a := range r.Attrs {
		attrs[a.Key] = a.Value.String()
	}
	return fabricui.LogRecord{Seq: r.Seq, Time: r.Time, Level: r.Level, Message: r.Message, Attrs: attrs}
}
