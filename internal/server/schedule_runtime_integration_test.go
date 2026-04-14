package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ixxet/apollo/internal/authz"
	"github.com/ixxet/apollo/internal/schedule"
)

func TestScheduleRuntimeHTTPRoundTripSupportsCalendarExceptionsAndCancel(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	ownerCookie, owner := createVerifiedSessionViaHTTP(t, env, "student-schedule-runtime-001", "schedule-runtime-001@example.com")
	setUserRole(t, env, owner.ID, authz.RoleOwner)

	createResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/schedule/blocks", `{
		"facility_key":"ashtonbee",
		"scope":"facility",
		"kind":"operating_hours",
		"effect":"informational",
		"visibility":"public_labeled",
		"weekly":{
			"weekday":1,
			"start_time":"09:00",
			"end_time":"10:00",
			"timezone":"America/Toronto",
			"recurrence_start_date":"2026-04-06",
			"recurrence_end_date":"2026-04-20"
		}
	}`, ownerCookie)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("createResponse.Code = %d, want %d body=%s", createResponse.Code, http.StatusCreated, createResponse.Body.String())
	}
	created := decodeScheduleBlock(t, createResponse)

	calendarResponse := env.doRequest(t, http.MethodGet, "/api/v1/schedule/calendar?facility_key=ashtonbee&from=2026-04-06T00:00:00Z&until=2026-04-27T00:00:00Z", nil, ownerCookie)
	if calendarResponse.Code != http.StatusOK {
		t.Fatalf("calendarResponse.Code = %d, want %d body=%s", calendarResponse.Code, http.StatusOK, calendarResponse.Body.String())
	}
	occurrences := decodeScheduleOccurrences(t, calendarResponse)
	if len(occurrences) != 3 {
		t.Fatalf("len(occurrences) = %d, want 3", len(occurrences))
	}

	exceptionResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/schedule/blocks/"+created.ID.String()+"/exceptions", `{"expected_version":1,"exception_date":"2026-04-13"}`, ownerCookie)
	if exceptionResponse.Code != http.StatusOK {
		t.Fatalf("exceptionResponse.Code = %d, want %d body=%s", exceptionResponse.Code, http.StatusOK, exceptionResponse.Body.String())
	}
	updated := decodeScheduleBlock(t, exceptionResponse)
	if updated.Version != 2 {
		t.Fatalf("updated.Version = %d, want 2", updated.Version)
	}

	calendarResponse = env.doRequest(t, http.MethodGet, "/api/v1/schedule/calendar?facility_key=ashtonbee&from=2026-04-06T00:00:00Z&until=2026-04-27T00:00:00Z", nil, ownerCookie)
	if calendarResponse.Code != http.StatusOK {
		t.Fatalf("calendarResponse(after exception).Code = %d, want %d body=%s", calendarResponse.Code, http.StatusOK, calendarResponse.Body.String())
	}
	occurrences = decodeScheduleOccurrences(t, calendarResponse)
	if len(occurrences) != 2 {
		t.Fatalf("len(occurrences) = %d, want 2", len(occurrences))
	}

	staleCancelResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/schedule/blocks/"+created.ID.String()+"/cancel", `{"expected_version":1}`, ownerCookie)
	if staleCancelResponse.Code != http.StatusConflict {
		t.Fatalf("staleCancelResponse.Code = %d, want %d body=%s", staleCancelResponse.Code, http.StatusConflict, staleCancelResponse.Body.String())
	}

	cancelResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/schedule/blocks/"+created.ID.String()+"/cancel", `{"expected_version":2}`, ownerCookie)
	if cancelResponse.Code != http.StatusOK {
		t.Fatalf("cancelResponse.Code = %d, want %d body=%s", cancelResponse.Code, http.StatusOK, cancelResponse.Body.String())
	}
	cancelled := decodeScheduleBlock(t, cancelResponse)
	if cancelled.Status != schedule.StatusCancelled {
		t.Fatalf("cancelled.Status = %q, want %q", cancelled.Status, schedule.StatusCancelled)
	}
}

func TestScheduleCalendarHTTPUsesRFC3339WindowsAndBlockTimezoneOccurrences(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	ownerCookie, owner := createVerifiedSessionViaHTTP(t, env, "student-schedule-runtime-002", "schedule-runtime-002@example.com")
	setUserRole(t, env, owner.ID, authz.RoleOwner)

	createResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/schedule/blocks", `{
		"facility_key":"ashtonbee",
		"scope":"facility",
		"kind":"event",
		"effect":"informational",
		"visibility":"internal",
		"weekly":{
			"weekday":7,
			"start_time":"23:30",
			"end_time":"23:50",
			"timezone":"America/Toronto",
			"recurrence_start_date":"2026-04-05",
			"recurrence_end_date":"2026-04-12"
		}
	}`, ownerCookie)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("createResponse.Code = %d, want %d body=%s", createResponse.Code, http.StatusCreated, createResponse.Body.String())
	}

	windowedResponse := env.doRequest(t, http.MethodGet, "/api/v1/schedule/calendar?facility_key=ashtonbee&from=2026-04-06T03:00:00Z&until=2026-04-06T04:00:00Z", nil, ownerCookie)
	if windowedResponse.Code != http.StatusOK {
		t.Fatalf("windowedResponse.Code = %d, want %d body=%s", windowedResponse.Code, http.StatusOK, windowedResponse.Body.String())
	}
	occurrences := decodeScheduleOccurrences(t, windowedResponse)
	if len(occurrences) != 1 {
		t.Fatalf("len(occurrences) = %d, want 1", len(occurrences))
	}
	if got := occurrences[0].OccurrenceDate; got != "2026-04-05" {
		t.Fatalf("occurrences[0].OccurrenceDate = %q, want %q", got, "2026-04-05")
	}
	if got := occurrences[0].StartsAt.UTC().Format("2006-01-02T15:04:05Z07:00"); got != "2026-04-06T03:30:00Z" {
		t.Fatalf("occurrences[0].StartsAt = %q, want %q", got, "2026-04-06T03:30:00Z")
	}

	tooNarrowResponse := env.doRequest(t, http.MethodGet, "/api/v1/schedule/calendar?facility_key=ashtonbee&from=2026-04-06T04:00:00Z&until=2026-04-06T05:00:00Z", nil, ownerCookie)
	if tooNarrowResponse.Code != http.StatusOK {
		t.Fatalf("tooNarrowResponse.Code = %d, want %d body=%s", tooNarrowResponse.Code, http.StatusOK, tooNarrowResponse.Body.String())
	}
	occurrences = decodeScheduleOccurrences(t, tooNarrowResponse)
	if len(occurrences) != 0 {
		t.Fatalf("len(occurrences) = %d, want 0", len(occurrences))
	}
}

func decodeScheduleBlock(t *testing.T, response *httptest.ResponseRecorder) schedule.Block {
	t.Helper()

	var payload schedule.Block
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal(schedule block) error = %v", err)
	}
	return payload
}

func decodeScheduleOccurrences(t *testing.T, response *httptest.ResponseRecorder) []schedule.Occurrence {
	t.Helper()

	var payload []schedule.Occurrence
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal(schedule occurrences) error = %v", err)
	}
	return payload
}
