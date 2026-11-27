// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package bridge

import (
	"errors"
	"testing"
)

// TestEndpointsDoneOwed (follow-up 2a): a change of the endpoints is
// handed on by the first sync whose batch returned nil, once — also by a
// healing sync with an empty diff after a failed reassembly, a case the
// end-to-end harness cannot produce.
func TestEndpointsDoneOwed(t *testing.T) {
	failed := errors.New("batch")
	b := &Bridge{}
	steps := []struct {
		name    string
		changed bool
		err     error
		want    bool
	}{
		{"nothing changed", false, nil, false},
		{"a change, its batch refused in part", true, failed, false},
		{"the retry, its batch refused again", true, failed, false},
		{"a failed reassembly, empty diff", false, failed, false},
		{"the healing sync, empty diff", false, nil, true},
		{"a later sync without changes", false, nil, false},
		{"a change applied at once", true, nil, true},
		{"and nothing after", false, nil, false},
	}
	for _, s := range steps {
		if got := b.endpointsDone(s.changed, s.err); got != s.want {
			t.Fatalf("%s: endpoints %v, want %v", s.name, got, s.want)
		}
	}
}
