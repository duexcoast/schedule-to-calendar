package main

import (
	"strings"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
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

// santizes name and returns it in the way it appears on the schedule: "Ney,Conor"
func (u *user) nameSchedFormat() string {
	lowerName := strings.ToLower(u.name)
	caser := cases.Title(language.AmericanEnglish)
	titleCased := caser.String(lowerName)

	splitName := strings.Split(titleCased, " ")
	first := splitName[0]
	last := splitName[1]
	return strings.Join([]string{last, first}, ", ")
}

type weeklySchedule []shift

type shift struct {
	Day       time.Weekday
	StartTime time.Time
	EndTime   time.Time
}
