package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthEndpoint(t *testing.T) {
	handler := NewHandler(Dependencies{ConsumerEnabled: true})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	body := recorder.Body.String()
	if !strings.Contains(body, "\"service\":\"apollo\"") {
		t.Fatalf("body = %q, want service field", body)
	}
	if !strings.Contains(body, "\"consumer_enabled\":true") {
		t.Fatalf("body = %q, want consumer_enabled true", body)
	}
}

func TestMetricsEndpointExportsCompetitionTelemetry(t *testing.T) {
	handler := NewHandler(Dependencies{ConsumerEnabled: true})

	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	body := recorder.Body.String()
	for _, metric := range []string{
		"competition_result_write_attempt_total",
		"trusted_surface_failure_total",
		"game_identity_projection_duration_seconds",
	} {
		if !strings.Contains(body, metric) {
			t.Fatalf("body missing %q: %s", metric, body)
		}
	}
}
