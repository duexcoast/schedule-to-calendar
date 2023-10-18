package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
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
	attachmentName *regexp.Regexp
}

// newSearchCriteria returns a searchCriteria struct for use in searching a
// gmail inbox. The dates parameter is variadic and can handle three conditions:
//  1. No dates provided. This will return a searchCriteria that searches the
//     inbox for messages sent today only.
//  2. One date provided. This will return a searchCriteria that searches the
//     inbox for messages sent on this specific date.
//  3. Two dates provided. This indicates a range search and will search the
//     inbox for messages sent between the two dates. The second date is non-
//     inclusive (messages sent on that date will not be included in the results)
//
// If more than two dates are provided, then the rest will be ignored.
func newSearchCriteria(email, subject string, dates ...time.Time) searchCriteria {
	// declare start and end date variables
	var startDate, endDate string

	// get the current time, and one day in advance
	oneDay := 24 * time.Hour

	// if there is only one date provided, then we want to perform a search for
	// that day specifically.
	switch len(dates) {
	case 0:
		// we want the default search to be within 1 day. that means we want the
		// after field to be today, and the before field to be one day in the future.
		today := time.Now()
		tomorrow := today.Add(oneDay)

		startDate = timeToGmailFormat(today)
		endDate = timeToGmailFormat(tomorrow)
	case 1:
		startDate = timeToGmailFormat(dates[0])
		endDate = timeToGmailFormat(dates[0].Add(oneDay * 2))
	default:
		startDate, endDate = timeToGmailFormat(dates[0]), timeToGmailFormat(dates[1])
	}

	return searchCriteria{
		email:          email,
		subject:        subject,
		after:          startDate,
		before:         endDate,
		hasAttachment:  true,
		attachmentName: rePDF,
	}
}

// generateGmailSearchTerm takes the fields of a searchCriteria object and
// constructs a string in the correct format to search a gmail inbox, it returns
// the string
func (sc searchCriteria) generateGmailSearchTerm() string {
	// "from:(yolanda.gureckas@starr-restaurant.com) subject:(Server Schedule) has:attachment"
	var b strings.Builder
	fmt.Fprintf(&b, "from:(%s) subject:(%s) has:attachment before:%s after:%s", sc.email, sc.subject, sc.before, sc.after)
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

// findAllScheduleEmails will return a []message containing all schedule emails
// sent after 1/1/2023.
func (g *gmailService) findAllScheduleEmails() ([]message, error) {
	msgs := []message{}

	before := time.Date(2023, time.January, 1, 0, 0, 0, 0, time.Local)
	sc := newSearchCriteria("yolanda.gureckas@starr-restaurant", "Server Schedule",
		time.Now(), before)
	g.searchCriteria = sc

	// user is special value for gmail API: used for userID and indicates the
	// currently authenticated user
	user := "me"
	searchString := g.generateGmailSearchTerm()
	r, err := g.srv.Users.Messages.List(user).Q(searchString).Do()
	if err != nil {
		// TODO: I still need to figure out the best way to log things and where
		// to handle errors. Should I still log here if I'm returning the error?
		// Should I log the error message like this? I think more structured
		// information would be better
		newErr := fmt.Errorf("Unable to retrieve message list from gmail: %w", err)
		g.logger.Error(newErr.Error())
		return nil, newErr
	}

	g.logger.Info("Processing messages from gmail inbox", "numMsgs", len(r.Messages))
	for _, m := range r.Messages {
		msg, err := g.srv.Users.Messages.Get(user, m.Id).Do()
		if err != nil {
			// If we can't retrieve a particular message, we'll just log it
			// and continue to the next message
			g.logger.Error("Unable to retrieve message", "msgID", m.Id)
			continue
		}
		date := ""
		for _, h := range msg.Payload.Headers {
			if h.Name == "Date" {
				date = h.Value
				break
			}
		}
		// TODO: I forget if this will initialize to nil or zero values. This is
		// important for the check below to see if we found an attachment
		var schedAttachment attachment

		for _, msg := range msg.Payload.Parts {
			if g.attachmentName.MatchString(msg.Filename) {
				fmt.Println(msg.Filename)
				schedAttachment.filename = msg.Filename
				schedAttachment.attachmentID = msg.Body.AttachmentId
			}

		}
		if schedAttachment.attachmentID == "" {
			// log and continue to next iteration
			g.logger.Error("This message did not contain a schedule attachment", "msgId", m.Id)
			continue
		}

		msgs = append(msgs, message{
			msgId:      msg.Id,
			date:       date,
			snippet:    msg.Snippet,
			attachment: schedAttachment,
		})
	}
	return msgs, nil

}

// findScheduleEmail will search for a schedule email and return the first
// message matching the date provided. The dates parameter is variadic and can
// handle three cases:
//  1. No dates provided. In this case we will search for emails sent TODAY.
//  2. One date provided. We will search for messages sent on this specific date.
//  3. Two dates provided. This indicates a range search, we will search for
//     schedule emails sent within this range and return the first result found.
//
// If more than two dates are provided, then the rest will be ignored.
//
// The function finds the attachment containing the schedule and saves it in the
// message.attachment field
func (g *gmailService) findScheduleEmail(date ...time.Time) (message, error) {
	// TODO: The searchCriteria information is hardcoded in this function, and
	// if the format of Server Schedule emails changes, that will need to
	// change here. This is not the ideal way of doing things, and should be
	// restructured.

	user := "me"
	var sc searchCriteria
	switch len(date) {
	// There is no date provided. This means we want to search for schedule emails
	// that were sent TODAY.
	case 0:
		sc = newSearchCriteria("yolanda.gureckas@starr-restaurant.com",
			"Server Schedule")
	// There is one date provided. This means that we want to search for schedule
	// emails sent on a specific day.
	case 1:
		sc = newSearchCriteria("yolanda.gureckas@starr-restaurant.com",
			"Server Schedule", date[0])

	case 2:
		sc = newSearchCriteria("yolanda.gureckas@starr-restaurant.com",
			"Server Schedule", date[0], date[1])
	}
	g.searchCriteria = sc
	searchTerm := g.generateGmailSearchTerm()
	g.logger.Debug("generated search term", "searchTerm", searchTerm)
	req := g.srv.Users.Messages.List(user).Q(searchTerm)
	r, err := req.Do()
	if err != nil {
		log.Printf("Unable to retrieve messages: %v", err)
		return message{}, err
	}
	log.Printf("Search result contains %d messages.\n", len(r.Messages))
	if len(r.Messages) == 0 {
		return message{}, fmt.Errorf("Did not find any messages matching these search terms:\n%s\nMessageResponse:\n%+v\n", searchTerm, r)
	}
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

	// Handle case where a matching attachment is not found
	if schedAttachment.attachmentID == "" {
		return message{}, fmt.Errorf("This message did not contain an attachment matching the schedule PDF")
	}

	message := message{
		msgId:      msg.Id,
		date:       msgDate,
		snippet:    msg.Snippet,
		attachment: schedAttachment,
	}

	return message, nil
}

// downloadAttachment takes the message provided and downloads the attachment.
// It returns []byte representing the downloaded file.
func (g *gmailService) downloadAttachment(msg message) ([]byte, error) {
	req := g.srv.Users.Messages.Attachments.Get("me", msg.msgId,
		msg.attachment.attachmentID)
	attachment, err := req.Do()
	if err != nil {
		g.logger.Error("Unable to retrieve attachment", "attachmentName",
			msg.attachment.filename, "attachmentID",
			msg.attachment.attachmentID, "err", err.Error())
		return nil, err
	}
	decoded, err := base64.URLEncoding.DecodeString(attachment.Data)
	if err != nil {
		g.logger.Error("Unable to decode attachment", "attachmentName",
			msg.attachment.filename, "attachmentID",
			msg.attachment.attachmentID, "err", err.Error())
		return decoded, err
	}
	return decoded, nil
}

// func (g *gmailService) downloadAttachmentsSlice(msgs []message) {
// 	for _, msg := range msgs {
// 		if data, err := g.downloadAttachment(msg); err != nil {
// 			// log and continue
// 			g.logger.Error("Could not download attachment", "msgID", msg.msgId,
// 				"attachmentName", msg.attachment.filename, "attachmentID",
// 				msg.attachment.attachmentID)
// 			continue
// 		}
// 	}
// }

// makeFilename removes spaces and extension from the filename
func makeFilename(name string) string {
	noSpaces := strings.Replace(name, " ", "", -1)
	filename := strings.Replace(noSpaces, ".pdf", "", -1)
	return filename
}
