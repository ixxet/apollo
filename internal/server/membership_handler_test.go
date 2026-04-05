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

	"github.com/ixxet/apollo/internal/auth"
	"github.com/ixxet/apollo/internal/membership"
)

type stubMembershipManager struct {
	getResponse   membership.LobbyMembership
	getErr        error
	joinResponse  membership.LobbyMembership
	joinErr       error
	leaveResponse membership.LobbyMembership
	leaveErr      error
}

func (s stubMembershipManager) GetLobbyMembership(context.Context, uuid.UUID) (membership.LobbyMembership, error) {
	if s.getErr != nil {
		return membership.LobbyMembership{}, s.getErr
	}

	return s.getResponse, nil
}

func (s stubMembershipManager) JoinLobbyMembership(context.Context, uuid.UUID) (membership.LobbyMembership, error) {
	if s.joinErr != nil {
		return membership.LobbyMembership{}, s.joinErr
	}

	return s.joinResponse, nil
}

func (s stubMembershipManager) LeaveLobbyMembership(context.Context, uuid.UUID) (membership.LobbyMembership, error) {
	if s.leaveErr != nil {
		return membership.LobbyMembership{}, s.leaveErr
	}

	return s.leaveResponse, nil
}

func TestLobbyMembershipEndpointsRequireAuthentication(t *testing.T) {
	handler := NewHandler(Dependencies{
		Auth:       stubAuthenticator{cookieName: "apollo_session"},
		Membership: stubMembershipManager{},
	})

	for _, testCase := range []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/api/v1/lobby/membership"},
		{method: http.MethodPost, path: "/api/v1/lobby/membership/join"},
		{method: http.MethodPost, path: "/api/v1/lobby/membership/leave"},
	} {
		request := httptest.NewRequest(testCase.method, testCase.path, nil)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s status = %d, want %d", testCase.method, testCase.path, recorder.Code, http.StatusUnauthorized)
		}
	}
}

func TestLobbyMembershipEndpointsReturnStableMembershipShapes(t *testing.T) {
	joinedAt := time.Date(2026, 4, 5, 18, 0, 0, 0, time.UTC)
	leftAt := time.Date(2026, 4, 5, 18, 30, 0, 0, time.UTC)
	manager := stubMembershipManager{
		getResponse: membership.LobbyMembership{
			Status:   membership.StatusNotJoined,
			JoinedAt: &joinedAt,
			LeftAt:   &leftAt,
		},
		joinResponse: membership.LobbyMembership{
			Status:   membership.StatusJoined,
			JoinedAt: &joinedAt,
		},
		leaveResponse: membership.LobbyMembership{
			Status:   membership.StatusNotJoined,
			JoinedAt: &joinedAt,
			LeftAt:   &leftAt,
		},
	}
	handler := NewHandler(Dependencies{
		Auth: stubAuthenticator{
			cookieName: "apollo_session",
			principal:  auth.Principal{UserID: uuid.MustParse("11111111-1111-1111-1111-111111111111")},
		},
		Membership: manager,
	})

	requestCookie := &http.Cookie{Name: "apollo_session", Value: "signed"}

	getRequest := httptest.NewRequest(http.MethodGet, "/api/v1/lobby/membership", nil)
	getRequest.AddCookie(requestCookie)
	getRecorder := httptest.NewRecorder()
	handler.ServeHTTP(getRecorder, getRequest)
	if getRecorder.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want %d", getRecorder.Code, http.StatusOK)
	}

	joinRequest := httptest.NewRequest(http.MethodPost, "/api/v1/lobby/membership/join", nil)
	joinRequest.AddCookie(requestCookie)
	joinRecorder := httptest.NewRecorder()
	handler.ServeHTTP(joinRecorder, joinRequest)
	if joinRecorder.Code != http.StatusOK {
		t.Fatalf("join status = %d, want %d", joinRecorder.Code, http.StatusOK)
	}

	leaveRequest := httptest.NewRequest(http.MethodPost, "/api/v1/lobby/membership/leave", nil)
	leaveRequest.AddCookie(requestCookie)
	leaveRecorder := httptest.NewRecorder()
	handler.ServeHTTP(leaveRecorder, leaveRequest)
	if leaveRecorder.Code != http.StatusOK {
		t.Fatalf("leave status = %d, want %d", leaveRecorder.Code, http.StatusOK)
	}

	assertMembershipPayload(t, getRecorder, membership.StatusNotJoined, true, true)
	assertMembershipPayload(t, joinRecorder, membership.StatusJoined, true, false)
	assertMembershipPayload(t, leaveRecorder, membership.StatusNotJoined, true, true)
}

func TestLobbyMembershipEndpointsMapErrorsClearly(t *testing.T) {
	joinedAt := time.Date(2026, 4, 5, 18, 0, 0, 0, time.UTC)

	for _, testCase := range []struct {
		name       string
		method     string
		path       string
		manager    stubMembershipManager
		wantStatus int
	}{
		{
			name:       "missing member returns not found on read",
			method:     http.MethodGet,
			path:       "/api/v1/lobby/membership",
			manager:    stubMembershipManager{getErr: membership.ErrNotFound},
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "ineligible join returns conflict",
			method:     http.MethodPost,
			path:       "/api/v1/lobby/membership/join",
			manager:    stubMembershipManager{joinErr: membership.IneligibleError{Reason: "visibility_ghost"}},
			wantStatus: http.StatusConflict,
		},
		{
			name:       "repeat join returns conflict",
			method:     http.MethodPost,
			path:       "/api/v1/lobby/membership/join",
			manager:    stubMembershipManager{joinErr: membership.ErrAlreadyJoined},
			wantStatus: http.StatusConflict,
		},
		{
			name:       "repeat leave returns conflict",
			method:     http.MethodPost,
			path:       "/api/v1/lobby/membership/leave",
			manager:    stubMembershipManager{leaveErr: membership.ErrNotJoined},
			wantStatus: http.StatusConflict,
		},
		{
			name:   "unexpected join failure returns internal server error",
			method: http.MethodPost,
			path:   "/api/v1/lobby/membership/join",
			manager: stubMembershipManager{joinErr: errors.New("boom"), joinResponse: membership.LobbyMembership{
				Status:   membership.StatusJoined,
				JoinedAt: &joinedAt,
			}},
			wantStatus: http.StatusInternalServerError,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			handler := NewHandler(Dependencies{
				Auth: stubAuthenticator{
					cookieName: "apollo_session",
					principal:  auth.Principal{UserID: uuid.MustParse("11111111-1111-1111-1111-111111111111")},
				},
				Membership: testCase.manager,
			})

			request := httptest.NewRequest(testCase.method, testCase.path, nil)
			request.AddCookie(&http.Cookie{Name: "apollo_session", Value: "signed"})
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)

			if recorder.Code != testCase.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, testCase.wantStatus)
			}
		})
	}
}

func assertMembershipPayload(t *testing.T, recorder *httptest.ResponseRecorder, wantStatus string, wantJoinedAt bool, wantLeftAt bool) {
	t.Helper()

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload["status"] != wantStatus {
		t.Fatalf("status = %#v, want %q", payload["status"], wantStatus)
	}
	_, hasJoinedAt := payload["joined_at"]
	if hasJoinedAt != wantJoinedAt {
		t.Fatalf("joined_at present = %t, want %t", hasJoinedAt, wantJoinedAt)
	}
	_, hasLeftAt := payload["left_at"]
	if hasLeftAt != wantLeftAt {
		t.Fatalf("left_at present = %t, want %t", hasLeftAt, wantLeftAt)
	}
}
