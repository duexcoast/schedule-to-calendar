package main

import (
	"os"
	"testing"
)

func TestNewShift(t *testing.T) {
	t.Run("generate shift struct", func(t *testing.T) {
		dateString := "10/14/2023"
		timeString := "8:30am"

		shift, err := newShift(timeString, dateString)
		t.Logf("%v\t%v", shift.startTime, shift.endTime)
		if err != nil {
			t.Fatalf("newShift could not complete, err=%v", err)
		}
	})

}

func TestGetWeeklyHours(t *testing.T) {
	t.Run("parse csv file and get weekly schedule", func(t *testing.T) {
		cfg, _ := newAppConfig(os.Stdout)
		common, _ := newCommon(cfg)
		user := newUser("Conor Ney", "conor.ux@gmail.com")
		parser := newCSVParser("testdata/schedule.csv", user, common)
		parser.readCSVFile()
		parser.getWeeklyHours()

	})

}
