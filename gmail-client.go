package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

type message struct {
	size    int64
	gmailID string
	date    string
	snippet string
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

	// authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	// fmt.Printf("Go to the following link in your browser then type the "+
	// 	"authorization code: \n%v\n", authURL)
	//
	// var authCode string
	// if _, err := fmt.Scan(&authCode); err != nil {
	// 	log.Fatalf("Unable to read authorization code: %v", err)
	// }
	//
	// tok, err := config.Exchange(context.TODO(), authCode)
	// if err != nil {
	// 	log.Fatalf("Unable to retrieve token from web: %v", err)
	// }
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

func FindScheduleEmail(srv *gmail.Service) {
	msgs := []message{}

	user := "me"
	// TODO: change this to just search for the schedule TODAY.
	req := srv.Users.Messages.List(user).Q("from:(yolanda.gureckas@starr-restaurant.com) subject:(Server Schedule) has:attachment ")
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
			gmailID: msg.Id,
			date:    date,
			snippet: msg.Snippet,
		})
	}

	count := 0
	reader := bufio.NewReader(os.Stdin)

	for _, m := range msgs {
		count++
		fmt.Printf("\nMessage URL: https://mail.google.com/mail/u/0/#all/%v\n", m.gmailID)
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
