package main

import "time"

type user struct {
	name     string
	email    string
	schedule weeklySchedule
}

type weeklySchedule []shift

type shift struct {
	day       time.Weekday
	startTime time.Time
	endTime   time.Time
}
