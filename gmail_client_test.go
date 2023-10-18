package main

import (
	"fmt"
	"log"
	"os"
	"path"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/unidoc/unipdf/v3/common/license"
)

func Test_generateGmailSearchTerm(t *testing.T) {
	t.Run("standard", func(t *testing.T) {
		date := time.Now()
		sc := newSearchCriteria("yolanda.gureckas@starr-restaurant.com", "Server Schedule",
			date)

		searchTerm := sc.generateGmailSearchTerm()
		fmt.Println(searchTerm)

		// fmt.Println(searchTerm)
	})

	t.Run("range", func(t *testing.T) {
		date := time.Now()
		dateEnd := time.Now()
		sc := newSearchCriteria("yolanda.gureckas@starr-restaurant.com", "Server Schedule",
			date, dateEnd)

		searchTerm := sc.generateGmailSearchTerm()
		fmt.Println(searchTerm)

		// fmt.Println(searchTerm)
	})

}

func Test_findScheduleEmail(t *testing.T) {
	t.Run("find the schedule email sent October 8, 2023", func(t *testing.T) {
		err := godotenv.Load()
		if err != nil {
			log.Fatal(err)
		}
		appConfig, _ := newAppConfig(os.Stdout)

		sharedDir := path.Join("testdata", "gmail_client")
		user := newUser("Conor Ney", "conor.ux@gmail.com")
		common, _ := newCommon(appConfig, sharedDir, user)
		app := newApp(appConfig, common)

		// load metered License API key prior to using the Unidoc library
		UNIDOC_API_KEY := os.Getenv("UNIDOC_API_KEY")
		err = license.SetMeteredKey(UNIDOC_API_KEY)
		if err != nil {
			log.Fatal(err)
		}
		gClient, err := setupGoogleClient()
		if err != nil {
			t.Fatal(err)
		}
		g, err := newGmailService(app.Common, gClient)
		if err != nil {
			t.Fatal(err)
		}
		// construct date for 10/8/2023, when a schedule email was sent
		correctDate := time.Date(2023, time.October, 8, 23, 59, 59, 59, time.Local)
		// msg, err := g.findAllScheduleEmails()
		// for i, v := range msg {
		// 	fmt.Println("\tSUBJECT: %s\tDATE: %s", v.subject, v.date)
		// }
		msg, err := g.findScheduleEmail(correctDate)
		if err != nil {
			t.Fatalf(err.Error())
		}
		_, err = g.downloadAttachment(msg)
		if err != nil {
			t.Fatalf("Could not download the attachment, err:%v", err)
		}
		// pdfParser := newPDFParser(g.filename, common)
		// pdfParser.PDFtoCSV()
		// csvParser := newCSVParser(pdfParser.filename, common)
		// records := csvParser.readCSVFile()
		// weeklySchedule, err := csvParser.getWeeklyHours(records)
		// if err != nil {
		// 	fmt.Printf("error parsing schedule, err: %v", err)
		// }
		// for _, v := range weeklySchedule {
		// 	fmt.Printf("[day] %s\n", v.Day)
		// 	fmt.Printf("[start] %s\n", v.StartTime)
		// 	fmt.Printf("[end] %s\n", v.EndTime)
		// }
	})
}
