package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

type googleCalendarService struct {
	*Common
	googleClient *http.Client
	srv          *calendar.Service
}

func newGoogleCalendarService(common *Common, googleClient *http.Client) (googleCalendarService, error) {
	calendarSrvc := googleCalendarService{
		Common:       common,
		googleClient: googleClient,
	}

	ctx := context.Background()
	srv, err := calendar.NewService(ctx, option.WithHTTPClient(googleClient))
	if err != nil {
		return calendarSrvc, fmt.Errorf("Unable to retrieve Calendar client: %v", err)
	}
	calendarSrvc.srv = srv

	return calendarSrvc, nil
}

func (gc googleCalendarService) addEvent(event *calendar.Event) error {
	event, err := gc.srv.Events.Insert("primary", event).Do()
	if err != nil {
		return fmt.Errorf("Unable to add event to calendar. Err: %v", err)
	}
	fmt.Printf("Event created: %s", event.HtmlLink)
	return nil
}

func (gc googleCalendarService) AddWeeklySchedule(ws weeklySchedule) error {
	for _, v := range ws {
		event := gc.makeEvent(v)
		if err := gc.addEvent(event); err != nil {
			return fmt.Errorf("Couldn't add shift to calendar: Start Time: %v Err: %v", v.StartTime, err)
		}
	}
	return nil
}

func (gc googleCalendarService) makeEvent(shift shift) *calendar.Event {
	startTime := &calendar.EventDateTime{
		DateTime: shift.StartTime.Format(time.RFC3339),
	}
	endTime := &calendar.EventDateTime{
		DateTime: shift.EndTime.Format(time.RFC3339),
	}

	event := &calendar.Event{
		Start: startTime,
		End:   endTime,
		// TODO: use ColorId field
		// ColorId: ,
		Location: "1739 N Front St.\nPhiladelphia, PA 19123",
		Summary:  "Work LMNO",
	}

	return event
}
