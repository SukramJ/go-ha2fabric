// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package bridge

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// dropEvery is how often a run of dropped items is summed up in the log.
const dropEvery = 10 * time.Second

// dropLog logs a run of dropped items (a full work or command queue) as
// one Error line at its first item and then, every interval while items
// keep being dropped, one line with their count (HR-P2-bound-64): one
// line per item would fill go-fabric's log ring, and the operation page
// would show nothing else.
type dropLog struct {
	log   *slog.Logger
	msg   string
	hint  string
	every time.Duration

	mu     sync.Mutex
	n      int
	active bool
}

// drop counts one dropped item; attrs describe the first of a run.
func (d *dropLog) drop(attrs ...slog.Attr) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.active {
		d.n++
		return
	}
	d.active = true
	d.log.LogAttrs(context.Background(), slog.LevelError, d.msg, append(attrs, slog.Int("dropped", 1), slog.String("hint", d.hint))...)
	time.AfterFunc(d.every, d.flush)
}

// flush logs the items dropped since the last line, and ends the run when
// there were none.
func (d *dropLog) flush() {
	d.mu.Lock()
	defer d.mu.Unlock()
	n := d.n
	d.n = 0
	if n == 0 {
		d.active = false
		return
	}
	d.log.LogAttrs(context.Background(), slog.LevelError, d.msg, slog.Int("dropped", n), slog.Duration("within", d.every), slog.String("hint", d.hint))
	time.AfterFunc(d.every, d.flush)
}
