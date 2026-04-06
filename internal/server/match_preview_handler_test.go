package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ixxet/apollo/internal/ares"
	"github.com/ixxet/apollo/internal/auth"
)

type stubMatchPreviewReader struct {
	response ares.MatchPreview
	err      error
}

func (s stubMatchPreviewReader) GetLobbyMatchPreview(context.Context) (ares.MatchPreview, error) {
	if s.err != nil {
		return ares.MatchPreview{}, s.err
	}

	return s.response, nil
}

func TestLobbyMatchPreviewEndpointRequiresAuthentication(t *testing.T) {
	handler := NewHandler(Dependencies{
		Auth:         stubAuthenticator{cookieName: "apollo_session"},
		MatchPreview: stubMatchPreviewReader{},
	})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/lobby/match-preview", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestLobbyMatchPreviewEndpointReturnsStablePreviewShape(t *testing.T) {
	generatedAt := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)
	memberOne := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	memberTwo := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	handler := NewHandler(Dependencies{
		Auth: stubAuthenticator{
			cookieName: "apollo_session",
			principal:  auth.Principal{UserID: uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")},
		},
		MatchPreview: stubMatchPreviewReader{
			response: ares.MatchPreview{
				GeneratedAt:    &generatedAt,
				CandidateCount: 2,
				PreviewVersion: ares.PreviewVersion,
				Matches: []ares.Match{
					{
						MemberIDs:    []uuid.UUID{memberOne, memberTwo},
						MemberLabels: []string{"11111111", "22222222"},
						Score:        2,
						Reasons: []ares.Reason{
							{Code: "explicit_joined_membership"},
							{Code: "compatible_visibility_mode", Value: "discoverable"},
							{Code: "compatible_availability_mode", Value: "available_now"},
							{Code: "stable_pair_order", Value: "user_id_asc"},
						},
					},
				},
			},
		},
	})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/lobby/match-preview", nil)
	request.AddCookie(&http.Cookie{Name: "apollo_session", Value: "signed"})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload["candidate_count"] != float64(2) {
		t.Fatalf("candidate_count = %#v, want 2", payload["candidate_count"])
	}
	if payload["preview_version"] != ares.PreviewVersion {
		t.Fatalf("preview_version = %#v, want %q", payload["preview_version"], ares.PreviewVersion)
	}
	matches, ok := payload["matches"].([]any)
	if !ok || len(matches) != 1 {
		t.Fatalf("matches = %#v, want one entry", payload["matches"])
	}
	match, ok := matches[0].(map[string]any)
	if !ok {
		t.Fatalf("match = %#v, want object", matches[0])
	}
	memberIDs, ok := match["member_ids"].([]any)
	if !ok || len(memberIDs) != 2 {
		t.Fatalf("member_ids = %#v, want two ids", match["member_ids"])
	}
	if memberIDs[0] != memberOne.String() || memberIDs[1] != memberTwo.String() {
		t.Fatalf("member_ids = %#v, want [%q %q]", memberIDs, memberOne, memberTwo)
	}
	if match["score"] != float64(2) {
		t.Fatalf("score = %#v, want 2", match["score"])
	}
}

func TestLobbyMatchPreviewEndpointMapsFailuresClearly(t *testing.T) {
	handler := NewHandler(Dependencies{
		Auth: stubAuthenticator{
			cookieName: "apollo_session",
			principal:  auth.Principal{UserID: uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")},
		},
		MatchPreview: stubMatchPreviewReader{err: errors.New("boom")},
	})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/lobby/match-preview", nil)
	request.AddCookie(&http.Cookie{Name: "apollo_session", Value: "signed"})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}
