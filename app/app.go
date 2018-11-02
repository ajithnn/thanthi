package app

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
)

type Mailer struct {
	Service  *gmail.Service
	User     string
	Messages map[string]string
}

func NewMailer(creds []byte) (*Mailer, error) {
	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(creds, gmail.MailGoogleComScope)
	if err != nil {
		return &Mailer{}, err
	}

	client := getClient(config)
	srv, err := gmail.New(client)
	if err != nil {
		return &Mailer{}, err
	}

	return &Mailer{
		srv,
		"me",
		make(map[string]string),
	}, nil
}

func (mailer *Mailer) DeleteAll(labels []string) error {
	err := mailer.Service.Users.Messages.List(mailer.User).LabelIds(labels...).MaxResults(500).Pages(context.Background(), mailer.deleteMessages)
	if err != nil {
		return err
	}
	return nil
}

func (mailer *Mailer) SendMail(to, sub, cc, msg string) error {
	mesg := &gmail.Message{}
	// RFC2822 Format for EMAIL Messages
	// Headers \r\n\r\n
	// Body
	mesg.Raw = base64.URLEncoding.EncodeToString([]byte("From: 'me'\r\n" +
		"reply-to: ajith@amagi.com\r\n" +
		"To: " + to + "\r\n" +
		"CC: " + cc + "\r\n" +
		"Subject: " + sub + " \r\n" +
		"Content-Type: text/html;\r\n\r\n" +
		msg))
	_, err := mailer.Service.Users.Messages.Send(mailer.User, mesg).Do()
	if err == nil {
		fmt.Println("Sent Message")
	}
	return err
}

func (mailer *Mailer) ListMail(labels []string) error {
	resp, err := mailer.Service.Users.Threads.List(mailer.User).LabelIds(labels...).MaxResults(10).Q("is:unread").Do()
	if err != nil {
		return err
	}

	msgs := make(map[string]string)
	for _, thread := range resp.Threads {
		resp, err := mailer.Service.Users.Threads.Get(mailer.User, thread.Id).Format("metadata").Do()
		if err != nil {
			return err
		}
		for _, msg := range resp.Messages {
			curSnippet := msg.Snippet
			curThread := msg.ThreadId
			curSub := ""
			curFrom := ""
			for _, header := range msg.Payload.Headers {
				if header.Name == "Subject" {
					curSub = header.Value
				}
				if header.Name == "From" {
					curFrom = header.Value
				}
			}
			msgs[curThread] += "   " + curSub + ":"
			msgs[curThread] += curFrom + ",\n\n"
			msgs[curThread] += curSnippet + "\n\n"
		}
	}
	mailer.Messages = msgs
	return nil
}

func (mailer *Mailer) deleteMessages(r *gmail.ListMessagesResponse) error {
	msgIds := make([]string, 0)
	for _, l := range r.Messages {
		msgIds = append(msgIds, l.Id)
	}
	err := mailer.Service.Users.Messages.BatchDelete(mailer.User, &gmail.BatchDeleteMessagesRequest{Ids: msgIds}).Do()
	return err
}

// Retrieve a token, saves the token, then returns the generated client.
func getClient(config *oauth2.Config) *http.Client {
	tokFile := "configs/token.json"
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokFile, tok)
	}
	return config.Client(context.Background(), tok)
}

// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return tok
}

// Retrieves a token from a local file.
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

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}
