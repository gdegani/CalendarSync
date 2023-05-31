package google

import (
	"errors"
	"time"

	"github.com/inovex/CalendarSync/internal/models"

	"google.golang.org/api/calendar/v3"
)

// calendarEventToEvent maps a *calendar.Event to a sync.Event and ensures that Metadata exists.
// *calendar.Event struct is defined here: https://github.com/googleapis/google-api-go-client/blob/8e67ca4a0f3502e05612b3e1c2a31ff3f3193aff/calendar/v3/calendar-gen.go#L1300
func calendarEventToEvent(e *calendar.Event, adapterSourceID string) models.Event {
	metadata := ensureMetadata(e, adapterSourceID)

	var attendees []models.Attendee
	for _, eventAttendee := range e.Attendees {
		attendees = append(attendees, models.Attendee{
			Email:       eventAttendee.Email,
			DisplayName: eventAttendee.DisplayName,
		})
	}

	var reminders []models.Reminder
	for _, reminder := range e.Reminders.Overrides {
		if reminder.Method == "popup" {
			reminders = append(reminders, models.Reminder{
				Actions: models.ReminderActionDisplay,
				Trigger: models.ReminderTrigger{
					PointInTime: eventDateTimeToTime(e.Start).Add(-(time.Minute * time.Duration(reminder.Minutes))),
				},
			})
		}
	}

	return models.Event{
		ICalUID:     e.ICalUID,
		ID:          e.Id,
		Title:       e.Summary,
		Description: e.Description,
		Location:    e.Location,
		AllDay:      isAllDayEvent(*e),
		StartTime:   eventDateTimeToTime(e.Start),
		EndTime:     eventDateTimeToTime(e.End),
		Metadata:    metadata,
		Attendees:   attendees,
		Reminders:   reminders,
		MeetingLink: e.HangoutLink,
	}
}

// ensureMetadata will return the metadata for a given event.
// If the event has custom metadata stored in EventExtendedProperties, this metadata will be returned.
// Otherwise, new metadata will be derived from the given event.
func ensureMetadata(event *calendar.Event, adapterSourceID string) *models.Metadata {
	var metadata *models.Metadata
	var err error
	if event.ExtendedProperties != nil && len(event.ExtendedProperties.Private) > 0 {
		metadata, err = models.EventMetadataFromMap(event.ExtendedProperties.Private)
		if errors.Is(err, models.ErrMetadataNotFound) {
			metadata = models.NewEventMetadata(event.Id, event.HtmlLink, adapterSourceID)
		}
	} else {
		metadata = models.NewEventMetadata(event.Id, event.HtmlLink, adapterSourceID)
	}

	return metadata
}

// isAllDayEvent returns true if the event is an 'all-day' event.
func isAllDayEvent(event calendar.Event) bool {
	return event.Start.Date != ""
}

// timeToEventDateTime converts an internal event time representation
// to EventDateTime which is required by the Google Calendar API.
// todo: handle timezone
func timeToEventDateTime(allDay bool, t time.Time) *calendar.EventDateTime {
	if allDay {
		return &calendar.EventDateTime{
			Date:     t.Format("2006-01-02"),
			DateTime: "",
			TimeZone: "",
		}
	}

	return &calendar.EventDateTime{
		Date:     "",
		DateTime: t.Format(time.RFC3339),
		TimeZone: "",
	}
}

// eventDateTimeToTime converts an EventDateTime to a basic time.Time which we use internally.
func eventDateTimeToTime(t *calendar.EventDateTime) time.Time {
	if t == nil {
		return time.Time{}
	}

	// the Date field will be set if the event is an all-day event
	if t.Date != "" {
		pt, err := time.Parse("2006-01-02", t.Date)
		if err != nil {
			panic(err) // this should not happen and indicates an issue with the CalendarAPI SDK
		}
		return pt
	}

	// events which are not all-day must have the DateTime field set
	if t.DateTime != "" {
		pt, err := time.Parse(time.RFC3339, t.DateTime)
		if err != nil {
			panic(err) // this should not happen as well, for the same reason
		}
		return pt
	}

	// at this point the event is most likely malformed, but we add a time anyway to remedy the need to fail here
	return time.Now()
}