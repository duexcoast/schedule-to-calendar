package main

import (
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/unidoc/unipdf/v3/common/license"
)

func Test_generateGmailSearchTerm(t *testing.T) {
	t.Run("standard", func(t *testing.T) {
		date := time.Now()
		sc := newSearchCriteria("yolanda.gureckas@starr-restaurant.com", "Server Schedule",
			false, date)

		searchTerm := sc.generateGmailSearchTerm()
		fmt.Println(searchTerm)

		// fmt.Println(searchTerm)
	})

	t.Run("range", func(t *testing.T) {
		date := time.Now()
		dateEnd := time.Now()
		sc := newSearchCriteria("yolanda.gureckas@starr-restaurant.com", "Server Schedule",
			true, date, dateEnd)

		searchTerm := sc.generateGmailSearchTerm()
		fmt.Println(searchTerm)

		// fmt.Println(searchTerm)
	})

}

func Test_findScheduleEmail(t *testing.T) {
	t.Run("find the schedule email sent today", func(t *testing.T) {
		err := godotenv.Load()
		if err != nil {
			log.Fatal(err)
		}
		appConfig, _ := newAppConfig(os.Stdout)
		common, _ := newCommon(appConfig)
		app := newApp(appConfig, common)
		logger := app.logger
		logger.Debug("app initialized")

		// load metered License API key prior to using the Unidoc library
		UNIDOC_API_KEY := os.Getenv("UNIDOC_API_KEY")
		err = license.SetMeteredKey(UNIDOC_API_KEY)
		if err != nil {
			log.Fatal(err)
		}
		g, err := newGmailClient(common)
		if err != nil {
			t.Fatalf(err.Error())
		}
		msg, err := g.findScheduleEmail()
		if err != nil {
			t.Fatalf(err.Error())
		}
		g.downloadAttachment(msg)
		pdfParser := newPDFParser(g.filename, common)
		pdfParser.parse()
		user := newUser("Conor Ney", "conor.ux@gmail.com")
		csvParser := newCSVParser(pdfParser.filename, user, common)
		csvParser.readCSVFile()
		weeklySchedule, err := csvParser.getWeeklyHours()
		if err != nil {
			fmt.Printf("error parsing schedule, err: %v", err)
		}
		for _, v := range weeklySchedule {
			fmt.Printf("[day] %s\n", v.day)
			fmt.Printf("[start] %s\n", v.startTime)
			fmt.Printf("[end] %s\n", v.endTime)
		}
	})
}
