package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

func readCSVFile(filePath string) [][]string {
	f, err := os.Open(filePath)
	if err != nil {
		log.Fatalf("unable to read input file %q err=%v", filePath, err)
	}
	defer f.Close()

	csvReader := csv.NewReader(f)
	records, err := csvReader.ReadAll()
	if err != nil {
		log.Fatalf("unable to parse file as a CSV. %q err=%v", filePath, err)
	}
	return records
}

// getWeeklyHours takes the records array returned by readCSVFile and puts the
// hours into
// func getWeeklyHours()

func newShift(startTime string, date string) (shift, error) {
	// used for time.Parse, defines how to interpret string being parsed.
	// we now need to concatenate the input strings into this format
	// and parse them
	dateLayout := "1/2/06 3:04pm MST"

	start := dateString(startTime, date)

	shift := shift{}

	parsedStartTime, err := time.Parse(dateLayout, start)
	if err != nil {
		return shift, err
	}

	shift.startTime = parsedStartTime
	shift.day = parsedStartTime.Weekday()

	var endTime string
	// convert "3:04 pm" into "3:04pm". Not sure if the input wil always be one
	// way or the other, so normalizing it just in case
	startTimeNormalized := strings.ReplaceAll(startTime, " ", "")

	// switch case to set endTime based on startTimeNormalized
	switch startTimeNormalized {
	case "8:30am":
		endTime = "3:00pm"
	case "11:00am":

	case "3:45pm", "4:00pm":
		endTime = "10:00pm"
	case "5:00pm":
		// if it's a Friday or Saturday, then the end time is 11:30 pm,
		// otherwise 10:15
		weekDay := parsedStartTime.Weekday()
		if weekDay == 5 || weekDay == 6 {
			endTime = "11:30pm"
		} else {
			endTime = "10:15pm"
		}
	}

	end := dateString(endTime, date)
	parsedEndTime, err := time.Parse(dateLayout, end)
	if err != nil {
		return shift, err
	}

	shift.endTime = parsedEndTime
	fmt.Println(parsedEndTime)

	return shift, nil
}

func dateString(time, date string) string {
	return strings.Join([]string{date, time, "EDT"}, " ")
}
