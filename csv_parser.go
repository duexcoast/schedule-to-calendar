package main

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"time"
)

const csvDateRegex = `(\d\d?/\d\d?/\d\d\d\d)`
const repairCSVRegex = `(\d\d?:\d\d[pam]{2})|(oncall)|(REQUESTOFF)|(SHIFT|LEAD)`

var ErrOnCallShift = errors.New("On call shift")

func ParseSchedCSV(data io.Reader, user user) (weeklySchedule, error) {
	schedCSV := readCSV(data)
	fmt.Println("%+v", schedCSV)
	weeklySched, err := getWeeklyHours(schedCSV, user)
	fmt.Println("%v", weeklySched)
	if err != nil {
		return nil, err
	}
	return weeklySched, nil
}

type csvSchedRecords [][]string

// method readCSVFile reads the CSV file stored at c.inputPath using a csvReader,
// returning the parsed data.
func readCSV(data io.Reader) csvSchedRecords {
	csvReader := csv.NewReader(data)
	records, err := csvReader.ReadAll()
	if err != nil {
		log.Fatalf("unable to parse file as a CSV. %q err=%v", data, err)
	}
	return records
}

// getWeeklyHours converts the parsed csv data provided by the records arg into
// []shift, for the employee specified in csvParser.user.
func getWeeklyHours(records csvSchedRecords, user user) (weeklySchedule, error) {
	// map key is index and value is date, taken from top row of
	// csv records
	dateMap := map[int]string{}

	for i := 0; i < len(records[0]); i++ {
		dateMap[i] = records[0][i]
	}
	fmt.Println(dateMap)

	// this will be populated with the index of the employees hours.
	// their hours are contained here and employeeIndex + 1
	var employeeIndex int
	// set match to true if we locate the employee by name
	match := false

	// this is the name of the employee as it appears on the schedule.
	// We will compare against this to find the rows that belong to the
	// employee
	schedName := user.nameSchedFormat()

	// loop over records and get the index of the employee
	for i := 0; i < len(records); i++ {

		if records[i][0] == schedName {
			match = true
			employeeIndex = i
		}
	}

	if !match {
		return nil, fmt.Errorf("Could not find the employee in the csv input. Looking for: %s", schedName)
	}

	// create slice to store []shift.
	weeklySchedule := []shift{}

	for i := employeeIndex; i <= employeeIndex+1; i++ {
		for j := 1; j < len(records[i]); j++ {
			if records[i][j] != "" {
				startTime := records[i][j]
				if onCallCheck(startTime) {
					continue
				}
				date := dateMap[j]

				shift, err := newShift(startTime, date)
				if err != nil {
					return nil, err
				}
				weeklySchedule = append(weeklySchedule, shift)
			}
		}
	}
	return weeklySchedule, nil
}

// TODO: decide how to handle on call shifts
func onCallCheck(startTime string) bool {
	if startTime == "on call" {
		return true
	}
	return false
}

func newShift(startTime string, date string) (shift, error) {
	// used for time.Parse, defines how to interpret string being parsed.
	// We now need to concatenate the input strings into this format
	// and parse them
	dateLayout := "1/2/2006 3:04pm MST"

	start := dateString(startTime, date)

	shift := shift{}

	parsedStartTime, err := time.Parse(dateLayout, start)
	if err != nil {
		return shift, err
	}
	fmt.Println("TEST")

	shift.StartTime = parsedStartTime
	shift.Day = parsedStartTime.Weekday()

	var endTime string
	// convert "3:04 pm" into "3:04pm". Not sure if the input wil always be one
	// way or the other, so normalizing it just in case
	startTimeNormalized := strings.ReplaceAll(startTime, " ", "")

	// switch case to set endTime based on startTimeNormalized
	switch startTimeNormalized {
	case "8:30am":
		endTime = "3:00pm"
	case "11:00am":
		endTime = "4:00pm"
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

	shift.EndTime = parsedEndTime

	return shift, nil
}

func dateString(time, date string) string {
	return strings.Join([]string{date, time, "EDT"}, " ")
}
