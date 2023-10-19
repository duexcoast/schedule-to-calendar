package main

import (
	"bytes"
	"log"
	"os"
)

func main() {
	app := Initialize(os.Stdout)
	gClient, err := setupGoogleClient()
	if err != nil {
		log.Fatal(err)
	}

	gmSrv, err := newGmailService(app.Common, gClient)
	if err != nil {
		log.Fatal("Could not initialize Gmail Service API", err)
	}

	pdf, err := gmSrv.FindAndDownloadSchedule()
	if err != nil {
		log.Fatal("Could not get schedule attachment", err)
	}

	readerPDF := bytes.NewReader(pdf)
	csv, err := ParseSchedPDF(readerPDF)
	if err != nil {
		log.Fatal("Could not parse PDF", err)
	}

	readerCSV := bytes.NewReader(csv)
	weeklySched, err := ParseSchedCSV(readerCSV, app.user)
	if err != nil {
		log.Fatal("Could not parse CSV", err)
	}

	gcSrv, err := newGoogleCalendarService(app.Common, gClient)
	if err != nil {
		log.Fatal("Could not initialize Google Calendar Service API", err)
	}

	if err = gcSrv.AddWeeklySchedule(weeklySched); err != nil {
		log.Fatal("Could not add schedule to Google Calendar", err)
	}
}
