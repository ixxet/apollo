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
