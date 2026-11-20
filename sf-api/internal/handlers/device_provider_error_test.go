package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"strong-fish-api/internal/devicelink"
)

// writeProviderError is a sink for the device/provider reads. A client that
// walks away mid-read cancels the request's context, and the provider call
// returns an error wrapping context.Canceled: that is a closed tab, not a
// provider outage, so it must answer 499 and never a 5xx (which would log an
// error and trip the [ERROR] Discord alert). A genuine provider failure still
// answers 502.
func TestWriteProviderError_CanceledIsNot5xx(t *testing.T) {
	h := &DeviceHandler{}

	cases := []struct {
		name string
		err  error
		want int
	}{
		{"client canceled", fmt.Errorf("garmin: get heart rate: %w", context.Canceled), statusClientClosedRequest},
		{"reauthorize", devicelink.ErrReauthorize, http.StatusFailedDependency},
		{"not linked", devicelink.ErrNotLinked, http.StatusConflict},
		{"genuine upstream", errors.New("garmin returned 503"), http.StatusBadGateway},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			h.writeProviderError(rr, tc.err, "garmin")
			if rr.Code != tc.want {
				t.Fatalf("status = %d, want %d", rr.Code, tc.want)
			}
		})
	}
}
