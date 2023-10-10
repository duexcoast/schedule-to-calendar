package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

const pdfRegex = `^Server Schedule`
const dateRegex = `^\d.*\d$`

var rePDF = regexp.MustCompile(pdfRegex)
var reDate = regexp.MustCompile(dateRegex)

type gmailService struct {
	*Common

	googleClient *http.Client
	srv          *gmail.Service
	searchCriteria
	// does not include extension
	filename string
	outPath  string
}

func newGmailService(common *Common, googleClient *http.Client) (*gmailService, error) {
	gmailSrvc := &gmailService{
		Common:       common,
		googleClient: googleClient,
	}
	ctx := context.Background()
	srv, err := gmail.NewService(ctx, option.WithHTTPClient(gmailSrvc.googleClient))
	if err != nil {
		err = fmt.Errorf("Unable to retrieve Gmail client: %v", err)
		return nil, err
	}
	gmailSrvc.srv = srv
	return gmailSrvc, nil
}

type searchCriteria struct {
	// sender's email
	email string
	// if false then we will ignore the endDate
	rangeSearch bool
	// date to begin searching
	after string
	// date to end searching
	before string
	// email subject
	subject string
	// will default to true because obviously we're looking for a schedule PDF
	hasAttachment bool
	// name of the attachment, not used in search but used to confirm we have
	// the correct attachment
	attachmentName regexp.Regexp
}

// dates is a variadic parameter, it can include 0-2 values: the first being the
// startTime for the search and teh second being the endTime for the search.
func newSearchCriteria(email, subject string, rangeSearch bool, dates ...time.Time) searchCriteria {
	// get the current time
	currentTime := time.Now()
	oneDay := 24 * time.Hour

	// we want the default search to be within 1 day.
	yesterday := currentTime.Add(-oneDay)
	startDate := timeToGmailFormat(yesterday)
	if len(dates) == 1 {
		startDate = timeToGmailFormat(dates[0])
	}
	// check if just the start date value is included, if so
	var endDate string
	if rangeSearch {
		startDate, endDate = timeToGmailFormat(dates[0]), timeToGmailFormat(dates[1])
	}

	return searchCriteria{
		email:          email,
		subject:        subject,
		rangeSearch:    rangeSearch,
		after:          startDate,
		before:         endDate,
		hasAttachment:  true,
		attachmentName: *rePDF,
	}
}

// generateGmailSearchTerm takes the fields of a searchCriteria object and
// constructs a string in the correct format to search a gmail inbox, it returns
// the string
func (sc searchCriteria) generateGmailSearchTerm() string {
	// "from:(yolanda.gureckas@starr-restaurant.com) subject:(Server Schedule) has:attachment"
	var b strings.Builder
	fmt.Fprintf(&b, "from:(%s) subject:(%s) has:attachment after:%s", sc.email, sc.subject, sc.after)
	if sc.rangeSearch {
		fmt.Fprintf(&b, " before: %s", sc.before)
	}
	return b.String()
}

func timeToGmailFormat(date time.Time) string {
	layout := "2006/1/2"
	return date.Format(layout)
}

type message struct {
	size       int64
	msgId      string
	date       string
	subject    string
	snippet    string
	attachment attachment
}

type attachment struct {
	filename     string
	attachmentID string
}

func (g *gmailService) findAllScheduleEmails(srv *gmail.Service) {
	msgs := []message{}

	// "from:(yolanda.gureckas@starr-restaurant.com) subject:(Server Schedule) has:attachment"
	before := time.Date(2023, time.January, 1, 0, 0, 0, 0, time.Local)
	sc := newSearchCriteria("yolanda.gureckas@starr-restaurant", "Server Schedule",
		true, time.Now(), before)
	g.searchCriteria = sc

	user := "me"
	searchString := g.generateGmailSearchTerm()
	req := srv.Users.Messages.List(user).Q(searchString)
	r, err := req.Do()
	if err != nil {
		log.Fatalf("Unable to retrieve messages: %v", err)
	}

	log.Printf("Processing %v messages...\n", len(r.Messages))
	for _, m := range r.Messages {
		msg, err := srv.Users.Messages.Get("me", m.Id).Do()
		if err != nil {
			log.Fatalf("Unable to retrieve message %v: %v", m.Id, err)
		}
		date := ""
		for _, h := range msg.Payload.Headers {
			if h.Name == "Date" {
				date = h.Value
				break
			}
		}
		msgs = append(msgs, message{
			msgId:   msg.Id,
			date:    date,
			snippet: msg.Snippet,
		})
	}

	count := 0
	reader := bufio.NewReader(os.Stdin)

	for _, m := range msgs {
		count++
		fmt.Printf("\nMessage URL: https://mail.google.com/mail/u/0/#all/%v\n", m.msgId)
		fmt.Printf("Size: %v, Date: %v, Snippet: %q\n", m.size, m.date, m.snippet)
		fmt.Printf("Options: (r)ead, (s)kip, (q)uit: [s] ")
		val := ""
		if val, err = reader.ReadString('\n'); err != nil {
			log.Fatalf("unable to scan input: %v", err)
		}
		val = strings.TrimSpace(val)
		switch val {
		case "q": // quit
			log.Printf("Done.  %v messages processed\n", count)
			os.Exit(0)
		default:
		}
	}

}

func (g *gmailService) findScheduleEmail(date ...time.Time) (message, error) {
	user := "me"
	var sc searchCriteria
	switch len(date) {
	case 0:
		sc = newSearchCriteria("yolanda.gureckas@starr-restaurant.com",
			"Server Schedule", false)
	case 1:
		sc = newSearchCriteria("yolanda.gureckas@starr-restaurant.com",
			"Server Schedule", false, date[0])

	case 2:
		sc = newSearchCriteria("yolanda.gureckas@starr-restaurant.com",
			"Server Schedule", true, date[0], date[1])
	}
	g.searchCriteria = sc
	req := g.srv.Users.Messages.List(user).Q(g.generateGmailSearchTerm())
	r, err := req.Do()
	if err != nil {
		log.Printf("Unable to retrieve messages: %v", err)
		return message{}, err
	}
	log.Printf("Processing %v messages...\n", len(r.Messages))
	m := r.Messages[0]

	msg, err := g.srv.Users.Messages.Get("me", m.Id).Do()
	if err != nil {
		log.Fatalf("Unable to retrieve message %v: %v", m.Id, err)
	}
	msgDate := ""
	for _, h := range msg.Payload.Headers {
		if h.Name == "Date" {
			msgDate = h.Value
			break
		}
	}

	var schedAttachment attachment

	for _, msg := range msg.Payload.Parts {
		if g.attachmentName.MatchString(msg.Filename) {
			fmt.Println(msg.Filename)
			schedAttachment.filename = msg.Filename
			schedAttachment.attachmentID = msg.Body.AttachmentId
		}

	}

	message := message{
		msgId:      msg.Id,
		date:       msgDate,
		snippet:    msg.Snippet,
		attachment: schedAttachment,
	}

	return message, nil
}

func (g *gmailService) downloadAttachment(msg message) {
	req := g.srv.Users.Messages.Attachments.Get("me", msg.msgId,
		msg.attachment.attachmentID)
	attachment, err := req.Do()
	if err != nil {
		g.logger.Debug("Unable to retrieve attachment %v: %v", msg.attachment.filename, err)
		return
	}
	decoded, err := base64.URLEncoding.DecodeString(attachment.Data)
	if err != nil {
		g.logger.Debug("Unable to decode attachment %v: %v", msg.attachment.filename, err)
		return
	}
	if g.outPath == "" {
		filename := makeFilename(msg.attachment.filename)
		g.filename = filename
		fullFilename := strings.Join([]string{g.filename, ".pdf"}, "")
		g.outPath = path.Join(g.Common.sharedDirectory, "pdf", fullFilename)
	}

	f, err := os.Create(g.outPath)
	if err != nil {
		g.logger.Debug("Could not create file: %q err: %v", msg.attachment.filename, err)
		return
		// return nil, err
	}
	defer f.Close()
	_, err = f.Write(decoded)
	if err != nil {
		g.logger.Debug("could not write attachment to file: %q err: %v", msg.attachment.filename, err)
	}
	// return f, nil
}

// makeFilename removes spaces and extension from the filename
func makeFilename(name string) string {
	noSpaces := strings.Replace(name, " ", "", -1)
	fileName := strings.Replace(noSpaces, ".pdf", "", -1)
	return fileName
}
