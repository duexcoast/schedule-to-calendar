package main

import (
	"path"
	"testing"
)

func Test_addEvent(t *testing.T) {
	sharedDir := path.Join("testdata", "gcalendar")
	app := setupAppForTest(t, sharedDir)

	t.Run("add a simple event", func(t *testing.T) {
		user := newUser("Conor Ney", "conor.ux@gmail.com")
		csvParser := newCSVParser("ServerSchedule10.9-10.15", user, app.Common)
		records := csvParser.readCSVFile()
		weeklySchedule, err := csvParser.getWeeklyHours(records)
		if err != nil {
			t.Fatalf("Unable to parse CSV file into weeklySchedule struct. Err: %v", err)
		}

		client, err := setupGoogleClient()
		if err != nil {
			t.Fatalf("Error setting up google client. Err: %v", err)
		}
		gc, err := newGoogleCalendarService(app.Common, client)
		if err != nil {
			t.Fatalf("Error creating Calendar Service through Google. Err: %v", err)
		}

		if err := gc.AddWeeklySchedule(weeklySchedule); err != nil {
			t.Fatalf("Error adding weeklySchedule to Google Calendar. Err: %v", err)
		}
	})
}
