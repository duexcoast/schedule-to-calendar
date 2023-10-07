package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

type csvParser struct {
	*Common
	user      user
	inputPath string
	records   csvSchedRecords
}

func newCSVParser(inputPath string, user user, common *Common) *csvParser {
	csvParser := csvParser{
		Common:    common,
		inputPath: inputPath,
		user:      user,
	}

	return &csvParser
}

type csvSchedRecords [][]string

func (c *csvParser) readCSVFile() {
	f, err := os.Open(c.inputPath)
	if err != nil {
		log.Fatalf("unable to read input file %q err=%v", c.inputPath, err)
	}
	defer f.Close()

	csvReader := csv.NewReader(f)
	records, err := csvReader.ReadAll()
	if err != nil {
		log.Fatalf("unable to parse file as a CSV. %q err=%v", c.inputPath, err)
	}
	c.records = records
	// fmt.Printf("%v", c.records)
}

// getWeeklyHours converts an [][]string containing the entire schedule, and
// returns []shift for csvParser.user
func (c *csvParser) getWeeklyHours() (weeklySchedule, error) {
	// map key is index and value is date, taken from top row of
	// csv records
	dateMap := map[int]string{}

	fmt.Printf("LEN: %d\n", len(c.records[0]))

	for i := 0; i < len(c.records[0]); i++ {
		dateMap[i] = c.records[0][i]
	}
	// for k, v := range dateMap {
	// fmt.Printf("key: %d\tvalue: %s\n", k, v)
	// }

	// this will be populated with the index of the employees hours.
	// their hours are contained here and employeeIndex + 1
	var employeeIndex int
	// set match to true if we locate the employee by name
	match := false

	// loop over records and get the index of the employee
	for i := 0; i < len(c.records); i++ {
		fmt.Println(c.records[i][0])

		// TODO: this shouldn't be hard coded. get name from user struct
		// and convert it to the correct "Last, First" format
		if c.records[i][0] == "Ney,Conor" {
			match = true
			employeeIndex = i
		}
	}
	fmt.Printf("Employee Index: %d\n", employeeIndex)

	if !match {
		return nil, fmt.Errorf("Could not find the employee in the csv file. %q", c.inputPath)
	}

	// create slice to store []shift.
	weeklySchedule := []shift{}

	for i := employeeIndex; i < employeeIndex+2; i++ {
		for j := 1; j < len(c.records[i]); j++ {
			if c.records[i][j] != "" {
				startTime := c.records[i][j]
				date := dateMap[j]

				shift, err := newShift(startTime, date)
				if err != nil {
					fmt.Println(err)
				}
				weeklySchedule = append(weeklySchedule, shift)
			}
		}
	}
	return weeklySchedule, nil
}

func newShift(startTime string, date string) (shift, error) {
	// used for time.Parse, defines how to interpret string being parsed.
	// we now need to concatenate the input strings into this format
	// and parse them
	dateLayout := "1/2/2006 3:04pm MST"

	start := dateString(startTime, date)

	shift := shift{}

	parsedStartTime, err := time.Parse(dateLayout, start)
	if err != nil {
		fmt.Println(err)
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

	return shift, nil
}

func dateString(time, date string) string {
	return strings.Join([]string{date, time, "EDT"}, " ")
}
