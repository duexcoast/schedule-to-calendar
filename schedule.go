package main

import (
	"strings"
	"time"
)

type user struct {
	name     string
	email    string
	schedule weeklySchedule
}

func newUser(name, email string) *user {
	user := &user{
		name:  name,
		email: email,
	}
	return user
}

func (u *user) nameSchedFormat() string {
	splitName := strings.Split(u.name, " ")
	first := splitName[0]
	last := splitName[1]
	//
	return strings.Join([]string{last, first}, ",")
}

type weeklySchedule []shift

type shift struct {
	Day       time.Weekday
	StartTime time.Time
	EndTime   time.Time
}
