package main

import "time"

type user struct {
	name     string
	email    string
	schedule weeklySchedule
}

type weeklySchedule []shift

type shift struct {
	startTime time.Time
	endTime   time.Time
}
