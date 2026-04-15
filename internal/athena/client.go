package athena

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

var (
	ErrBaseURLRequired          = errors.New("athena base url is required")
	ErrBaseURLInvalid           = errors.New("athena base url is invalid")
	ErrTimeoutInvalid           = errors.New("athena timeout must be greater than zero")
	ErrMalformedResponse        = errors.New("athena occupancy response is malformed")
	ErrAnalyticsMalformed       = errors.New("athena analytics response is malformed")
	ErrRequestTimeout           = errors.New("athena request timed out")
	ErrRequestFailed            = errors.New("athena request failed")
	ErrAnalyticsFacilityMissing = errors.New("athena analytics facility is required")
	ErrAnalyticsWindowInvalid   = errors.New("athena analytics window is invalid")
	ErrAnalyticsBucketInvalid   = errors.New("athena analytics bucket_minutes must be greater than zero")
	ErrAnalyticsLimitInvalid    = errors.New("athena analytics session_limit must be greater than zero")
)

type UpstreamStatusError struct {
	StatusCode int
	Message    string
}

func (e *UpstreamStatusError) Error() string {
	if strings.TrimSpace(e.Message) == "" {
		return fmt.Sprintf("athena request failed with status %d", e.StatusCode)
	}

	return fmt.Sprintf("athena request failed with status %d: %s", e.StatusCode, e.Message)
}

type OccupancySnapshot struct {
	FacilityID   string
	ZoneID       string
	CurrentCount int
	ObservedAt   time.Time
}

type AnalyticsFilter struct {
	FacilityID    string
	ZoneID        string
	NodeID        string
	Since         time.Time
	Until         time.Time
	BucketMinutes int
	SessionLimit  int
}

type AnalyticsReport struct {
	FacilityID         string             `json:"facility_id"`
	ZoneID             string             `json:"zone_id,omitempty"`
	NodeID             string             `json:"node_id,omitempty"`
	Since              time.Time          `json:"since"`
	Until              time.Time          `json:"until"`
	BucketMinutes      int                `json:"bucket_minutes"`
	ObservationSummary ObservationSummary `json:"observation_summary"`
	SessionSummary     SessionSummary     `json:"session_summary"`
	FlowBuckets        []FlowBucket       `json:"flow_buckets"`
	NodeBreakdown      []NodeBreakdown    `json:"node_breakdown"`
	Sessions           []SessionFact      `json:"sessions"`
}

type ObservationSummary struct {
	Total         int `json:"total"`
	Pass          int `json:"pass"`
	Fail          int `json:"fail"`
	CommittedPass int `json:"committed_pass"`
}

type SessionSummary struct {
	OpenCount              int   `json:"open_count"`
	ClosedCount            int   `json:"closed_count"`
	UnmatchedExitCount     int   `json:"unmatched_exit_count"`
	UniqueVisitors         int   `json:"unique_visitors"`
	AverageDurationSeconds int64 `json:"average_duration_seconds"`
	MedianDurationSeconds  int64 `json:"median_duration_seconds"`
	OccupancyAtEnd         int   `json:"occupancy_at_end"`
}

type FlowBucket struct {
	StartedAt    time.Time `json:"started_at"`
	EndedAt      time.Time `json:"ended_at"`
	PassIn       int       `json:"pass_in"`
	PassOut      int       `json:"pass_out"`
	FailIn       int       `json:"fail_in"`
	FailOut      int       `json:"fail_out"`
	OccupancyEnd int       `json:"occupancy_end"`
}

type NodeBreakdown struct {
	NodeID        string `json:"node_id"`
	Total         int    `json:"total"`
	Pass          int    `json:"pass"`
	Fail          int    `json:"fail"`
	CommittedPass int    `json:"committed_pass"`
}

type SessionFact struct {
	SessionID       string     `json:"session_id"`
	State           string     `json:"state"`
	EntryEventID    string     `json:"entry_event_id,omitempty"`
	EntryNodeID     string     `json:"entry_node_id,omitempty"`
	EntryAt         *time.Time `json:"entry_at,omitempty"`
	ExitEventID     string     `json:"exit_event_id,omitempty"`
	ExitNodeID      string     `json:"exit_node_id,omitempty"`
	ExitAt          *time.Time `json:"exit_at,omitempty"`
	DurationSeconds *int64     `json:"duration_seconds,omitempty"`
}

type Client struct {
	baseURL    *url.URL
	httpClient *http.Client
}

type occupancyResponse struct {
	FacilityID   string `json:"facility_id"`
	ZoneID       string `json:"zone_id,omitempty"`
	CurrentCount int    `json:"current_count"`
	ObservedAt   string `json:"observed_at"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func NewClient(baseURL string, timeout time.Duration) (*Client, error) {
	trimmed := strings.TrimSpace(baseURL)
	if trimmed == "" {
		return nil, ErrBaseURLRequired
	}
	if timeout <= 0 {
		return nil, ErrTimeoutInvalid
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBaseURLInvalid, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, ErrBaseURLInvalid
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return nil, ErrBaseURLInvalid
	}

	return &Client{
		baseURL: parsed,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}, nil
}

func (c *Client) CurrentOccupancy(ctx context.Context, facilityID string) (OccupancySnapshot, error) {
	query := make(url.Values)
	query.Set("facility", strings.TrimSpace(facilityID))

	response, err := c.doGET(ctx, "/api/v1/presence/count", query)
	if err != nil {
		return OccupancySnapshot{}, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return OccupancySnapshot{}, decodeUpstreamStatus(response)
	}

	var payload occupancyResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return OccupancySnapshot{}, fmt.Errorf("%w: %v", ErrMalformedResponse, err)
	}
	if strings.TrimSpace(payload.FacilityID) == "" || strings.TrimSpace(payload.ObservedAt) == "" || payload.CurrentCount < 0 {
		return OccupancySnapshot{}, ErrMalformedResponse
	}

	observedAt, err := time.Parse(time.RFC3339, payload.ObservedAt)
	if err != nil {
		return OccupancySnapshot{}, fmt.Errorf("%w: %v", ErrMalformedResponse, err)
	}

	return OccupancySnapshot{
		FacilityID:   strings.TrimSpace(payload.FacilityID),
		ZoneID:       strings.TrimSpace(payload.ZoneID),
		CurrentCount: payload.CurrentCount,
		ObservedAt:   observedAt.UTC(),
	}, nil
}

func (c *Client) OccupancyAnalytics(ctx context.Context, filter AnalyticsFilter) (AnalyticsReport, error) {
	if err := validateAnalyticsFilter(filter); err != nil {
		return AnalyticsReport{}, err
	}

	query := make(url.Values)
	query.Set("facility", strings.TrimSpace(filter.FacilityID))
	if strings.TrimSpace(filter.ZoneID) != "" {
		query.Set("zone", strings.TrimSpace(filter.ZoneID))
	}
	if strings.TrimSpace(filter.NodeID) != "" {
		query.Set("node", strings.TrimSpace(filter.NodeID))
	}
	query.Set("since", filter.Since.UTC().Format(time.RFC3339))
	query.Set("until", filter.Until.UTC().Format(time.RFC3339))
	if filter.BucketMinutes > 0 {
		query.Set("bucket_minutes", fmt.Sprintf("%d", filter.BucketMinutes))
	}
	if filter.SessionLimit > 0 {
		query.Set("session_limit", fmt.Sprintf("%d", filter.SessionLimit))
	}

	response, err := c.doGET(ctx, "/api/v1/presence/analytics", query)
	if err != nil {
		return AnalyticsReport{}, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return AnalyticsReport{}, decodeUpstreamStatus(response)
	}

	var payload AnalyticsReport
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return AnalyticsReport{}, fmt.Errorf("%w: %v", ErrAnalyticsMalformed, err)
	}
	if err := validateAnalyticsReport(payload); err != nil {
		return AnalyticsReport{}, err
	}

	return payload, nil
}

func validateAnalyticsFilter(filter AnalyticsFilter) error {
	if strings.TrimSpace(filter.FacilityID) == "" {
		return ErrAnalyticsFacilityMissing
	}
	if filter.Since.IsZero() || filter.Until.IsZero() || filter.Until.Before(filter.Since) {
		return ErrAnalyticsWindowInvalid
	}
	if filter.BucketMinutes < 0 {
		return ErrAnalyticsBucketInvalid
	}
	if filter.SessionLimit < 0 {
		return ErrAnalyticsLimitInvalid
	}

	return nil
}

func validateAnalyticsReport(report AnalyticsReport) error {
	if strings.TrimSpace(report.FacilityID) == "" || report.Since.IsZero() || report.Until.IsZero() || report.Until.Before(report.Since) {
		return ErrAnalyticsMalformed
	}
	if report.BucketMinutes <= 0 {
		return ErrAnalyticsMalformed
	}
	for _, bucket := range report.FlowBuckets {
		if bucket.StartedAt.IsZero() || bucket.EndedAt.IsZero() || bucket.EndedAt.Before(bucket.StartedAt) {
			return ErrAnalyticsMalformed
		}
	}

	return nil
}

func (c *Client) doGET(ctx context.Context, endpoint string, query url.Values) (*http.Response, error) {
	requestURL := *c.baseURL
	requestURL.Path = path.Join(c.baseURL.Path, endpoint)
	requestURL.RawQuery = query.Encode()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build athena request: %w", err)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, fmt.Errorf("%w: %v", ErrRequestTimeout, err)
		}

		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return nil, fmt.Errorf("%w: %v", ErrRequestTimeout, err)
		}

		return nil, fmt.Errorf("%w: %v", ErrRequestFailed, err)
	}

	return response, nil
}

func decodeUpstreamStatus(response *http.Response) error {
	var upstreamError errorResponse
	if err := json.NewDecoder(response.Body).Decode(&upstreamError); err != nil {
		return &UpstreamStatusError{StatusCode: response.StatusCode}
	}

	return &UpstreamStatusError{
		StatusCode: response.StatusCode,
		Message:    strings.TrimSpace(upstreamError.Error),
	}
}
