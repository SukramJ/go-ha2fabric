// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package ha

import (
	"errors"
	"fmt"
	"net/url"
	"testing"
)

// TestScrubURL: the URL of a net/http error is shown without its query in
// the error and the messages around it; an error without a URL is left as
// it is (HR-P0-ha-56).
func TestScrubURL(t *testing.T) {
	inner := &url.Error{Op: "Get", URL: "http://ha.local/api/websocket?key=s3cret", Err: errors.New("refused")}
	if got := scrubURL(fmt.Errorf("dial: %w", inner)).Error(); got != `dial: Get "http://ha.local/api/websocket": refused` {
		t.Fatalf("scrubbed %q", got)
	}
	empty := fmt.Errorf("dial: %w", &url.Error{Op: "Get", URL: "", Err: errors.New("refused")})
	if got := scrubURL(empty).Error(); got != empty.Error() {
		t.Fatalf("an empty URL changed the message: %q", got)
	}
}
