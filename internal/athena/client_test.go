package athena

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientReadsCurrentOccupancyAndAnalyticsContracts(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/presence/count":
			if got := r.URL.Query().Get("facility"); got != "ashtonbee" {
				t.Fatalf("count facility query = %q, want ashtonbee", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"facility_id":"ashtonbee","current_count":7,"observed_at":"2026-04-15T14:00:00Z"}`))
		case "/api/v1/presence/analytics":
			if got := r.URL.Query().Get("bucket_minutes"); got != "15" {
				t.Fatalf("analytics bucket_minutes query = %q, want 15", got)
			}
			if got := r.URL.Query().Get("session_limit"); got != "1" {
				t.Fatalf("analytics session_limit query = %q, want 1", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"facility_id":"ashtonbee",
				"since":"2026-04-15T13:00:00Z",
				"until":"2026-04-15T14:00:00Z",
				"bucket_minutes":15,
				"observation_summary":{"total":3,"pass":2,"fail":1,"committed_pass":2},
				"session_summary":{"open_count":1,"closed_count":1,"unmatched_exit_count":0,"unique_visitors":2,"average_duration_seconds":600,"median_duration_seconds":600,"occupancy_at_end":1},
				"flow_buckets":[{"started_at":"2026-04-15T13:00:00Z","ended_at":"2026-04-15T13:15:00Z","pass_in":1,"pass_out":0,"fail_in":0,"fail_out":0,"occupancy_end":1}],
				"node_breakdown":[{"node_id":"entry-node","total":3,"pass":2,"fail":1,"committed_pass":2}],
				"sessions":[{"session_id":"session-001","state":"open","entry_event_id":"edge-001"}]
			}`))
		default:
			t.Fatalf("unexpected upstream path %s", r.URL.Path)
		}
	}))
	defer upstream.Close()

	client, err := NewClient(upstream.URL, time.Second)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	snapshot, err := client.CurrentOccupancy(context.Background(), "ashtonbee")
	if err != nil {
		t.Fatalf("CurrentOccupancy() error = %v", err)
	}
	if snapshot.CurrentCount != 7 || snapshot.FacilityID != "ashtonbee" {
		t.Fatalf("snapshot = %+v, want ashtonbee count 7", snapshot)
	}

	report, err := client.OccupancyAnalytics(context.Background(), AnalyticsFilter{
		FacilityID:    "ashtonbee",
		Since:         time.Date(2026, 4, 15, 13, 0, 0, 0, time.UTC),
		Until:         time.Date(2026, 4, 15, 14, 0, 0, 0, time.UTC),
		BucketMinutes: 15,
		SessionLimit:  1,
	})
	if err != nil {
		t.Fatalf("OccupancyAnalytics() error = %v", err)
	}
	if report.ObservationSummary.CommittedPass != 2 {
		t.Fatalf("CommittedPass = %d, want 2", report.ObservationSummary.CommittedPass)
	}
	if len(report.Sessions) != 1 || report.Sessions[0].SessionID != "session-001" {
		t.Fatalf("report.Sessions = %+v, want one upstream session fact", report.Sessions)
	}
}

func TestClientHandlesTimeoutStatusAndMalformedPayloads(t *testing.T) {
	t.Run("timeout", func(t *testing.T) {
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(50 * time.Millisecond)
			_, _ = w.Write([]byte(`{"facility_id":"ashtonbee","current_count":1,"observed_at":"2026-04-15T14:00:00Z"}`))
		}))
		defer upstream.Close()

		client, err := NewClient(upstream.URL, time.Millisecond)
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}
		_, err = client.CurrentOccupancy(context.Background(), "ashtonbee")
		if !errors.Is(err, ErrRequestTimeout) {
			t.Fatalf("CurrentOccupancy() error = %v, want timeout", err)
		}
	})

	t.Run("non 200", func(t *testing.T) {
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":"analytics are not configured"}`))
		}))
		defer upstream.Close()

		client, err := NewClient(upstream.URL, time.Second)
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}
		_, err = client.CurrentOccupancy(context.Background(), "ashtonbee")
		var statusErr *UpstreamStatusError
		if !errors.As(err, &statusErr) || statusErr.StatusCode != http.StatusServiceUnavailable {
			t.Fatalf("CurrentOccupancy() error = %v, want upstream status", err)
		}
	})

	t.Run("malformed count", func(t *testing.T) {
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"facility_id":"ashtonbee","current_count":1}`))
		}))
		defer upstream.Close()

		client, err := NewClient(upstream.URL, time.Second)
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}
		_, err = client.CurrentOccupancy(context.Background(), "ashtonbee")
		if !errors.Is(err, ErrMalformedResponse) {
			t.Fatalf("CurrentOccupancy() error = %v, want malformed", err)
		}
	})

	t.Run("malformed analytics", func(t *testing.T) {
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"facility_id":"ashtonbee","since":"2026-04-15T13:00:00Z","until":"2026-04-15T14:00:00Z","bucket_minutes":0}`))
		}))
		defer upstream.Close()

		client, err := NewClient(upstream.URL, time.Second)
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}
		_, err = client.OccupancyAnalytics(context.Background(), AnalyticsFilter{
			FacilityID: "ashtonbee",
			Since:      time.Date(2026, 4, 15, 13, 0, 0, 0, time.UTC),
			Until:      time.Date(2026, 4, 15, 14, 0, 0, 0, time.UTC),
		})
		if !errors.Is(err, ErrAnalyticsMalformed) {
			t.Fatalf("OccupancyAnalytics() error = %v, want malformed", err)
		}
	})
}

func TestClientRejectsUnsafeBaseURL(t *testing.T) {
	for _, baseURL := range []string{"", "ftp://example.com", "http://"} {
		if _, err := NewClient(baseURL, time.Second); err == nil {
			t.Fatalf("NewClient(%q) error = nil, want refusal", baseURL)
		}
	}
	if _, err := NewClient("http://127.0.0.1:8080", 0); !errors.Is(err, ErrTimeoutInvalid) {
		t.Fatalf("NewClient(timeout 0) error = %v, want %v", err, ErrTimeoutInvalid)
	}
}

func TestClientRejectsInvalidAnalyticsFilter(t *testing.T) {
	client, err := NewClient("http://127.0.0.1:8080", time.Second)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	_, err = client.OccupancyAnalytics(context.Background(), AnalyticsFilter{})
	if !errors.Is(err, ErrAnalyticsFacilityMissing) {
		t.Fatalf("OccupancyAnalytics(empty) error = %v, want facility missing", err)
	}

	_, err = client.OccupancyAnalytics(context.Background(), AnalyticsFilter{
		FacilityID:    "ashtonbee",
		Since:         time.Date(2026, 4, 15, 13, 0, 0, 0, time.UTC),
		Until:         time.Date(2026, 4, 15, 14, 0, 0, 0, time.UTC),
		BucketMinutes: -1,
	})
	if !errors.Is(err, ErrAnalyticsBucketInvalid) {
		t.Fatalf("OccupancyAnalytics(negative bucket) error = %v, want bucket invalid", err)
	}

	if err != nil && strings.Contains(err.Error(), "account") {
		t.Fatalf("analytics filter error leaked identity language: %v", err)
	}
}
