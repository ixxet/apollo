package schedule

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ixxet/apollo/internal/store"
)

const (
	isoDateLayout = "2006-01-02"
	hhmmLayout    = "15:04"
)

func resourceFromStore(row store.ApolloScheduleResource) Resource {
	return Resource{
		ResourceKey:  row.ResourceKey,
		FacilityKey:  row.FacilityKey,
		ZoneKey:      row.ZoneKey,
		ResourceType: row.ResourceType,
		DisplayName:  row.DisplayName,
		PublicLabel:  row.PublicLabel,
		Bookable:     row.Bookable,
		Active:       row.Active,
		CreatedAt:    timeFromTimestamptz(row.CreatedAt),
		UpdatedAt:    timeFromTimestamptz(row.UpdatedAt),
	}
}

func edgeFromStore(row store.ApolloScheduleResourceEdge) ResourceEdge {
	return ResourceEdge{
		ResourceKey:        row.ResourceKey,
		RelatedResourceKey: row.RelatedResourceKey,
		EdgeType:           row.EdgeType,
		CreatedAt:          timeFromTimestamptz(row.CreatedAt),
		UpdatedAt:          timeFromTimestamptz(row.UpdatedAt),
	}
}

func blockFromStore(row store.ApolloScheduleBlock) Block {
	block := Block{
		ID:                           row.ID,
		FacilityKey:                  row.FacilityKey,
		ZoneKey:                      row.ZoneKey,
		ResourceKey:                  row.ResourceKey,
		Scope:                        row.Scope,
		ScheduleType:                 row.ScheduleType,
		Kind:                         row.Kind,
		Effect:                       row.Effect,
		Visibility:                   row.Visibility,
		Status:                       row.Status,
		Version:                      int(row.Version),
		Weekday:                      int16PtrToInt(row.Weekday),
		StartTime:                    timePtrFromPgTime(row.StartTime),
		EndTime:                      timePtrFromPgTime(row.EndTime),
		Timezone:                     row.Timezone,
		RecurrenceStartDate:          datePtrFromPgDate(row.RecurrenceStartDate),
		RecurrenceEndDate:            datePtrFromPgDate(row.RecurrenceEndDate),
		StartAt:                      timePtrFromTimestamptz(row.StartAt),
		EndAt:                        timePtrFromTimestamptz(row.EndAt),
		CreatedByUserID:              row.CreatedByUserID,
		CreatedBySessionID:           row.CreatedBySessionID,
		CreatedByRole:                row.CreatedByRole,
		CreatedByCapability:          row.CreatedByCapability,
		CreatedTrustedSurfaceKey:     row.CreatedTrustedSurfaceKey,
		CreatedTrustedSurfaceLabel:   row.CreatedTrustedSurfaceLabel,
		UpdatedByUserID:              row.UpdatedByUserID,
		UpdatedBySessionID:           row.UpdatedBySessionID,
		UpdatedByRole:                row.UpdatedByRole,
		UpdatedByCapability:          row.UpdatedByCapability,
		UpdatedTrustedSurfaceKey:     row.UpdatedTrustedSurfaceKey,
		UpdatedTrustedSurfaceLabel:   row.UpdatedTrustedSurfaceLabel,
		CreatedAt:                    timeFromTimestamptz(row.CreatedAt),
		UpdatedAt:                    timeFromTimestamptz(row.UpdatedAt),
		CancelledAt:                  timePtrFromTimestamptz(row.CancelledAt),
		CancelledByUserID:            uuidPtrFromPgUUID(row.CancelledByUserID),
		CancelledBySessionID:         uuidPtrFromPgUUID(row.CancelledBySessionID),
		CancelledByRole:              row.CancelledByRole,
		CancelledByCapability:        row.CancelledByCapability,
		CancelledTrustedSurfaceKey:   row.CancelledTrustedSurfaceKey,
		CancelledTrustedSurfaceLabel: row.CancelledTrustedSurfaceLabel,
	}
	return block
}

func exceptionFromStore(row store.ApolloScheduleBlockException) string {
	return dateStringFromPgDate(row.ExceptionDate)
}

func timeFromTimestamptz(value pgtype.Timestamptz) time.Time {
	if !value.Valid {
		return time.Time{}
	}

	return value.Time.UTC()
}

func timePtrFromTimestamptz(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}

	converted := value.Time.UTC()
	return &converted
}

func timePtrFromPgTime(value pgtype.Time) *string {
	if !value.Valid {
		return nil
	}

	microseconds := value.Microseconds
	hours := microseconds / 3_600_000_000
	minutes := (microseconds % 3_600_000_000) / 60_000_000
	converted := fmt.Sprintf("%02d:%02d", hours, minutes)
	return &converted
}

func datePtrFromPgDate(value pgtype.Date) *string {
	if !value.Valid {
		return nil
	}

	converted := value.Time.UTC().Format(isoDateLayout)
	return &converted
}

func dateStringFromPgDate(value pgtype.Date) string {
	if !value.Valid {
		return ""
	}

	return value.Time.UTC().Format(isoDateLayout)
}

func int16PtrToInt(value *int16) *int {
	if value == nil {
		return nil
	}

	converted := int(*value)
	return &converted
}

func intPtr(value int) *int {
	converted := value
	return &converted
}

func stringPtr(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func uuidPtrFromPgUUID(value pgtype.UUID) *uuid.UUID {
	if !value.Valid {
		return nil
	}

	converted := uuid.UUID(value.Bytes)
	return &converted
}

func pgTimeFromString(raw string) (pgtype.Time, error) {
	parsed, err := time.Parse(hhmmLayout, raw)
	if err != nil {
		return pgtype.Time{}, fmt.Errorf("parse time %q: %w", raw, err)
	}

	return pgtype.Time{
		Valid:        true,
		Microseconds: int64((parsed.Hour()*60 + parsed.Minute()) * 60 * 1_000_000),
	}, nil
}

func pgDateFromString(raw string) (pgtype.Date, error) {
	parsed, err := time.Parse(isoDateLayout, raw)
	if err != nil {
		return pgtype.Date{}, fmt.Errorf("parse date %q: %w", raw, err)
	}

	return pgtype.Date{
		Valid: true,
		Time:  time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, time.UTC),
	}, nil
}

func pgTimestamptzFromTime(raw time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Valid: true, Time: raw.UTC()}
}

func pgUUIDFromPtr(value *uuid.UUID) pgtype.UUID {
	if value == nil {
		return pgtype.UUID{}
	}

	return pgtype.UUID{Valid: true, Bytes: *value}
}

func alignToWeekday(date time.Time, weekday int, location *time.Location) time.Time {
	if location == nil {
		location = time.UTC
	}

	local := date.In(location)
	offset := (weekday - int(local.Weekday()) + 7) % 7
	aligned := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location).AddDate(0, 0, offset)
	return aligned
}

func alignToWeekdayOnOrBefore(date time.Time, weekday int, location *time.Location) time.Time {
	if location == nil {
		location = time.UTC
	}

	local := date.In(location)
	offset := (int(local.Weekday()) - weekday + 7) % 7
	aligned := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location).AddDate(0, 0, -offset)
	return aligned
}
