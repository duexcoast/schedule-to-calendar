package main

import (
	"fmt"
	"log"
	"path"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func Test_newShift(t *testing.T) {
	t.Run("generate shift struct", func(t *testing.T) {
		dateString := "10/14/2023"
		timeString := "8:30am"

		shift, err := newShift(timeString, dateString)
		t.Logf("%v\t%v", shift.StartTime, shift.EndTime)
		if err != nil {
			t.Fatalf("newShift could not complete, err=%v", err)
		}
	})

}

func Test_readCSVFile(t *testing.T) {
	tc := map[string]struct {
		filename string
		expected csvSchedRecords
	}{
		"correctly parse properly formatted CSV file": {
			filename: "ServerSchedule10.9-10.15",
			expected: [][]string{
				[]string{"", "10/9/2023", "10/10/2023", "10/11/2023", "10/12/2023", "10/13/2023", "10/14/2023", "10/15/2023"},
				[]string{"Servers", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"},
				[]string{"Bodkin,Danielle", "", "", "", "", "", "", ""},
				[]string{"267-592-1859", "", "3:45pm", "5:00pm", "", "3:45pm", "5:00pm", ""},
				[]string{"Boucher,Alicia", "", "", "", "", "", "8:30am", "R/O"},
				[]string{"973-440-0003", "5:00pm", "3:45pm", "", "", "3:45pm", "R/O", "5:00pm"},
				[]string{"Burnett,Ryan", "", "", "", "", "", "", ""},
				[]string{"215-694-6139", "", "5:00pm", "", "3:45pm", "5:00pm", "4:00pm", ""},
				[]string{"Curiale,Kevin", "", "", "", "", "", "", ""},
				[]string{"856-419-0409", "", "", "3:45pm", "", "5:00pm", "4:00pm", "4:00pm"},
				[]string{"Diaz,Rain", "", "", "", "", "", "8:30am", ""},
				[]string{"610-724-9029", "", "4:00pm", "", "", "", "", "4:00pm"},
				[]string{"Esteves-Baez,Ivanna", "", "", "SHIFT", "SHIFT", "", "SHIFT", "SHIFT"},
				[]string{"267-515-3395", "", "", "LEAD", "LEAD", "3:45pm", "LEAD", "LEAD"},
				[]string{"Freeman,Brooke", "", "", "", "", "", "", "8:30am"},
				[]string{"860-268-4767", "3:45pm", "", "", "", "", "", ""},
				[]string{"Gardner,Alana", "", "", "", "", "", "R/O", "11:00am"},
				[]string{"267-721-4801", "3:45pm", "", "", "5:00pm", "3:45pm", "", "4:00pm"},
				[]string{"Guzman,Brendan", "", "", "", "", "", "11:00am", "8:30am"},
				[]string{"609-458-0220", "", "", "3:45pm", "3:45pm", "", "4:00pm", ""},
				[]string{"Hann,Spencer", "", "", "", "", "", "11:00am", ""},
				[]string{"860-772-7230", "5:00pm", "3:45pm", "", "", "3:45pm", "4:00pm", "4:00pm"},
				[]string{"Karmilowicz,Macey", "", "", "", "", "", "R/O", "8:30am"},
				[]string{"215-439-4671", "oncall", "", "3:45pm", "5:00pm", "3:45pm", "", ""},
				[]string{"Lynch,Danielle", "", "", "", "", "", "", ""},
				[]string{"484-547-8913", "", "", "", "3:45pm", "3:45pm", "", ""},
				[]string{"Mekdaschi,Lamiesse", "R/O", "", "", "", "", "R/O", "11:00am"},
				[]string{"210-884-7270", "", "oncall", "", "3:45pm", "", "5:00pm", "4:00pm"},
				[]string{"Ney,Conor", "", "", "", "", "", "8:30am", ""},
				[]string{"650-815-9480", "3:45pm", "5:00pm", "", "", "", "4:00pm", "5:00pm"},
				[]string{"Palatajko,Alex", "", "", "", "", "", "", ""},
				[]string{"609-408-7539", "", "", "5:00pm", "3:45pm", "3:45pm", "5:00pm", ""},
				[]string{"SalazarMendez,Javier", "", "", "", "", "", "", ""},
				[]string{"267-709-5068", "", "5:00pm", "3:45pm", "5:00pm", "", "4:00pm", ""},
				[]string{"Sargent-Boone,Starr", "", "", "", "", "", "11:00am", ""},
				[]string{"267-290-7316", "5:00pm", "", "", "", "3:45pm", "4:00pm", "4:00pm"},
				[]string{"Thai,Valerie", "", "", "", "", "R/O", "8:30am", ""},
				[]string{"408-634-3961", "", "3:45pm", "5:00pm", "", "", "4:00pm", "5:00pm"},
				[]string{"Troutman,Imani", "", "", "", "", "", "", "11:00am"},
				[]string{"856-656-9097", "", "", "3:45pm", "3:45pm", "5:00pm", "4:00pm", ""},
			},
		},
	}

	// Test setup
	sharedDir := path.Join("testdata", "csv_parser")
	app := setupAppForTest(t, sharedDir)

	for name, test := range tc {
		t.Run(name, func(t *testing.T) {

			csvParser := newCSVParser(test.filename, app.Common)
			records := csvParser.readCSVFile()

			if !cmp.Equal(records, test.expected) {
				t.Logf("\n\nGOT:\n\n%v", records)
				t.Logf("\n\nEXPECTED:\n\n%v", test.expected)
				t.Logf("\n\nDIFF:\n\n%s", cmp.Diff(records, test.expected))
				t.Fatalf("Records did not match the expected output")
			}

		})

	}
}

func Test_getWeeklyHours(t *testing.T) {
	tc := map[string]struct {
		filename string
		expected weeklySchedule
	}{
		"get weekly schedule for Conor Ney": {
			filename: "ServerSchedule10.9-10.15",
			expected: []shift{
				shift{
					Day:       time.Saturday,
					StartTime: parseDateForTest("2023-10-14 08:30:00 -0400 EDT"),
					EndTime:   parseDateForTest("2023-10-14 15:00:00 -0400 EDT"),
				},
				shift{
					Day:       time.Monday,
					StartTime: parseDateForTest("2023-10-09 15:45:00 -0400 EDT"),
					EndTime:   parseDateForTest("2023-10-09 22:00:00 -0400 EDT"),
				},
				shift{
					Day:       time.Tuesday,
					StartTime: parseDateForTest("2023-10-10 17:00:00 -0400 EDT"),
					EndTime:   parseDateForTest("2023-10-10 22:15:00 -0400 EDT"),
				},
				shift{
					Day:       time.Saturday,
					StartTime: parseDateForTest("2023-10-14 16:00:00 -0400 EDT"),
					EndTime:   parseDateForTest("2023-10-14 22:00:00 -0400 EDT"),
				},
				shift{
					Day:       time.Sunday,
					StartTime: parseDateForTest("2023-10-15 17:00:00 -0400 EDT"),
					EndTime:   parseDateForTest("2023-10-15 22:15:00 -0400 EDT"),
				},
			},
		},
	}

	// Test setup
	sharedDir := path.Join("testdata", "csv_parser")
	app := setupAppForTest(t, sharedDir)

	for name, test := range tc {
		t.Run(name, func(t *testing.T) {
			csvParser := newCSVParser(test.filename, app.Common)
			records := csvParser.readCSVFile()
			weeklySchedule, err := csvParser.getWeeklyHours(records)
			if err != nil {
				t.Fatalf("Could not generate weeklySchedule struct from records. Err: %v", err)
			}
			if !cmp.Equal(weeklySchedule, test.expected) {
				t.Logf("\n\nDIFF:\n\n%s", cmp.Diff(weeklySchedule, test.expected))
			}
		})
	}

}

func printWeeklySchedule(ws weeklySchedule) {
	for _, v := range ws {
		fmt.Printf("day: %v\n", v.Day)
		fmt.Printf("startTime: %v\n", v.StartTime)
		fmt.Printf("endTime: %v\n\n", v.EndTime)

	}
}

func parseDateForTest(date string) time.Time {
	layout := "2006-01-02 15:04:05 -0700 MST"
	myTime, err := time.Parse(layout, date)
	if err != nil {
		log.Fatal(err)
	}
	return myTime
}
