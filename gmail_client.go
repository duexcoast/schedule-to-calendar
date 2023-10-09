package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

const pdfRegex = `^Server Schedule`
const dateRegex = `^\d.*\d$`

var rePDF = regexp.MustCompile(pdfRegex)
var reDate = regexp.MustCompile(dateRegex)

type gmailClient struct {
	*Common
	srv *gmail.Service
	searchCriteria
	// does not include extension
	filename string
	outPath  string
}

func newGmailClient(common *Common) (*gmailClient, error) {
	srv, err := setupGmailService()
	if err != nil {
		return nil, err
	}
	return &gmailClient{
		Common: common,
		srv:    srv,
	}, err
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

// Retrieve a token, saves the token, then returns the generated client.
func getClient(config *oauth2.Config) *http.Client {
	// The file token.json stores the user's access and refresh tokens, and is
	// created automatically when the authorization flow completes for the first
	// time
	tokFile := "token.json"
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokFile, tok)
	}

	return config.Client(context.Background(), tok)
}

// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	ch := make(chan string)
	randState := fmt.Sprintf("st%d", time.Now().UnixNano())
	ts := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/favicon.ico" {
			http.Error(rw, "", 404)
			return
		}
		if req.FormValue("state") != randState {
			log.Printf("State doesn't match: req = %#v", req)
			http.Error(rw, "", 500)
			return
		}
		if code := req.FormValue("code"); code != "" {
			fmt.Fprintf(rw, "<h1>Success</h1>Authorized.")
			rw.(http.Flusher).Flush()
			ch <- code
			return
		}
		log.Printf(" no code")
		http.Error(rw, "", 500)
	}))
	defer ts.Close()

	config.RedirectURL = ts.URL
	authURL := config.AuthCodeURL(randState)
	go openURL(authURL)
	log.Printf("Authorize this app at: %s", authURL)
	code := <-ch
	log.Printf("Got code: %s", code)

	token, err := config.Exchange(context.TODO(), code)
	if err != nil {
		log.Fatalf("Token exchange error: %v", err)
	}
	return token
}

func openURL(url string) {
	try := []string{"xdg-open", "google chrome", "open"}
	for _, bin := range try {
		err := exec.Command(bin, url).Run()
		if err != nil {
			return
		}
	}
}

// Retrieves token from local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %q\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func setupGmailService() (*gmail.Service, error) {
	ctx := context.Background()
	b, err := os.ReadFile("credentials.json")
	if err != nil {
		err = fmt.Errorf("Unable to read client secret file: %v", err)
		return nil, err
	}

	// If modifying these scopes, delete your previously saved token.json
	config, err := google.ConfigFromJSON(b, gmail.GmailReadonlyScope)
	if err != nil {
		err = fmt.Errorf("Unable to parse client secret file to config: %v", err)
		return nil, err
	}
	client := getClient(config)

	srv, err := gmail.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		err = fmt.Errorf("Unable to retrieve Gmail client: %v", err)
		return nil, err
	}
	return srv, nil
	//
	// user := "me"
	// r, err := srv.Users.Labels.List(user).Do()
	// if err != nil {
	// 	err = fmt.Errorf("Unable to retrieve labels: %v", err)
	// 	return nil, err
	// }
	// if len(r.Labels) != 0 {
	// 	fmt.Println("No labels found.")
	// 	return
	// }
	// fmt.Println("Labels:")
	// for _, l := range r.Labels {
	// 	fmt.Printf("- %s\n", l)
	// }
}

// TODO: Provide date to search for email. Going to search daily at 11:59pm for
// a schedule email sent that day. Function accepts date as param, returns the
// message. A separate function can then take the message and download the attachment
// that is the schedule PDF. We can then update the DB with the scheudle attachment
// data. This way, we can alawys correct schedules that were updated.

func (g *gmailClient) findAllScheduleEmails(srv *gmail.Service) {
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

func (g *gmailClient) findScheduleEmail(date ...time.Time) (message, error) {
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

func (g *gmailClient) downloadAttachment(msg message) {
	req := g.srv.Users.Messages.Attachments.Get("me", msg.msgId,
		msg.attachment.attachmentID)
	attachment, err := req.Do()
	fmt.Println("HERE1")
	if err != nil {
		g.logger.Debug("Unable to retrieve attachment %v: %v", msg.attachment.filename, err)
		fmt.Println("HERE1.5")
		return
	}
	fmt.Println("HERE2")
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

// makeFilename removes spaces from the filename
func makeFilename(name string) string {
	noSpaces := strings.Replace(name, " ", "", -1)
	// fmt.Printf("[noSpaces] %s", noSpaces)
	fileName := strings.Replace(noSpaces, ".pdf", "", -1)
	return fileName
}
