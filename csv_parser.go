package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"path"
	"regexp"
	"strings"
	"time"
)

const csvDateRegex = `(\d\d?/\d\d?/\d\d\d\d)`
const repairCSVRegex = `(\d\d?:\d\d[pam]{2})|(oncall)|(REQUESTOFF)|(SHIFT|LEAD)`

var reCSVDate = regexp.MustCompile(csvDateRegex)
var reRepairCSV = regexp.MustCompile(repairCSVRegex)

type csvParser struct {
	*Common
	user      *user
	filename  string
	inputPath string
}

func newCSVParser(filename string, user *user, common *Common) *csvParser {
	// add the correct extension to the filename
	fullFilename := strings.Join([]string{filename, ".csv"}, "")
	in := path.Join(common.sharedDirectory, "csv", fullFilename)
	csvParser := csvParser{
		Common:    common,
		filename:  filename,
		inputPath: in,
		user:      user,
	}

	return &csvParser
}

type csvSchedRecords [][]string

// method readCSVFile reads the CSV file stored at c.inputPath using a csvReader,
// returning the parsed data.
func (c *csvParser) readCSVFile() csvSchedRecords {
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
	return records
}

func (c *csvParser) repairCSV(records csvSchedRecords) csvSchedRecords {
	if len(records[0]) == 8 {
		c.logger.Debug("[repairCSV] Correct amount of rows.")
		return nil
	}
	newRecords := make([][]string, 0, 42)

	var friSatColumnBroken bool

	// Use a regular expression to determine whether any of the first row dates
	// contain multiple dates merged together
	for i, v := range records[0] {
		matches := reCSVDate.FindAllString(v, -1)
		// if the column hasn't been split correctly, that will be indicated by
		// more than 1 match.
		if len(matches) > 1 {
			if i == 5 {
				friSatColumnBroken = true
				dates := []string{matches[0], matches[1]}
				// append fixed dates row
				newRow := c.fixRow(records, dates, i, 0)
				newRecords = append(newRecords, newRow)
				// append Fixed Weekday row
				days := []string{"Friday", "Saturday"}
				newRow = c.fixRow(records, days, i, 1)
				newRecords = append(newRecords, newRow)
			} else {
				c.logger.Debug("[repairCSV] The records are broken on unfixable columns: %v", matches)
				return nil
			}
		}
	}
	// Don't return any data if we have an irrepairable row
	if !friSatColumnBroken {
		return nil
	}
	for i := 2; i < len(records); i++ {
		data, err := splitBrokenColumn(records[i][5], 5, i)
		if err != nil {
			fmt.Println("[repairCSV] Could not split column")
		}
		fixedRow := c.fixRow(records, data, 5, i)
		newRecords = append(newRecords, fixedRow)
	}
	return newRecords
}

func (c csvParser) fixRow(records csvSchedRecords, data []string, index, row int) []string {
	// allocate the new row that we will be appending to
	newRow := make([]string, 0, 8)
	prevRow := records[row]

	newRow = append(newRow, prevRow[0:index]...)
	newRow = append(newRow, data...)
	newRow = append(newRow, prevRow[index+1:]...)
	return newRow
}

func splitBrokenColumn(cell string, col, row int) ([]string, error) {
	// as of right now, I can only figure out how to fix a combined column
	// that straddles Friday and Saturday, thankfully, that's the only issue
	// I've experienced so far.
	// Check to confirm that the broken column is friday/saturday
	if col == 5 {
		// if row == 1 {
		// 	// date
		// 	data := []string{"Friday", "Saturday"}
		// 	return data, nil
		// }
		matches := reRepairCSV.FindAllString(cell, -1)
		switch len(matches) {
		case 0:
			data := []string{"", ""}
			return data, nil
		case 1:
			if strings.Contains(matches[0], "am") {
				data := []string{"", matches[0]}
				return data, nil
			} else if strings.Contains(matches[0], "pm") {
				data := []string{matches[0], ""}
				return data, nil
			} else if strings.Contains(matches[0], "oncall") {
				// I DONT KNOW WHAT TO DO IN THIS SITUATION!
				// There's a couple possibilities:
				//   - oncall friday
				//   - oncall saturday morning (we can tell it's a morning row) using
				//     modulo math (even or odd)
				//   - oncall saturday evening
				//   - so basically, I'm unable to determine between oncall
				//     friday pm and oncall saturday pm: so just mark it as both
				//     I guess?
				return []string{matches[0], matches[0]}, nil
			} else {
				return []string{"", ""}, nil
			}
		default:
			return []string{matches[0], matches[1]}, nil
		}
	}

	// shouldn't be calling this function if index isn't 5
	return nil, fmt.Errorf("Can't fix broken column. At splitBrokenColumn")
}

// getWeeklyHours converts the parsed csv data provided by the records arg into
// []shift, for the employee specified in csvParser.user.
func (c *csvParser) getWeeklyHours(records csvSchedRecords) (weeklySchedule, error) {
	// map key is index and value is date, taken from top row of
	// csv records
	dateMap := map[int]string{}

	for i := 0; i < len(records[0]); i++ {
		dateMap[i] = records[0][i]
	}

	// this will be populated with the index of the employees hours.
	// their hours are contained here and employeeIndex + 1
	var employeeIndex int
	// set match to true if we locate the employee by name
	match := false

	// this is the name of the employee as it appears on the schedule.
	// We will compare against this to find the rows that belong to the
	// employee
	schedName := c.user.nameSchedFormat()

	// loop over records and get the index of the employee
	for i := 0; i < len(records); i++ {

		if records[i][0] == schedName {
			match = true
			employeeIndex = i
		}
	}

	if !match {
		return nil, fmt.Errorf("Could not find the employee in the csv file. %q", c.inputPath)
	}

	// create slice to store []shift.
	weeklySchedule := []shift{}

	for i := employeeIndex; i < employeeIndex+2; i++ {
		for j := 1; j < len(records[i]); j++ {
			if records[i][j] != "" {
				startTime := records[i][j]
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

	shift.StartTime = parsedStartTime
	shift.Day = parsedStartTime.Weekday()

	var endTime string
	// convert "3:04 pm" into "3:04pm". Not sure if the input wil always be one
	// way or the other, so normalizing it just in case
	startTimeNormalized := strings.ReplaceAll(startTime, " ", "")

	// TODO: Handle the case where start time is "oncall", this means different
	// things in the morning than it does in the evening, so we need to take the
	// row and/or column into account

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

	shift.EndTime = parsedEndTime

	return shift, nil
}

func dateString(time, date string) string {
	return strings.Join([]string{date, time, "EDT"}, " ")
}
