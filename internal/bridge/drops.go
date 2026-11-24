// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package bridge

import (
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
	args := make([]any, 0, len(attrs)+2)
	for _, a := range attrs {
		args = append(args, a)
	}
	d.log.Error(d.msg, append(args, slog.Int("dropped", 1), slog.String("hint", d.hint))...)
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
	d.log.Error(d.msg, slog.Int("dropped", n), slog.Duration("within", d.every), slog.String("hint", d.hint))
	time.AfterFunc(d.every, d.flush)
}
