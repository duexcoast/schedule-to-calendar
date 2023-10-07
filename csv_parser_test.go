package main

import "testing"

func TestNewShift(t *testing.T) {
	t.Run("generate shift struct", func(t *testing.T) {
		dateString := "10/9/23"
		timeString := "8:30am"

		_, err := newShift(timeString, dateString)
		if err != nil {
			t.Fatalf("newShift could not complete, err=%v", err)
		}
	})

}
