package app

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/ajithnn/thanthi/logger"
	"gitlab.com/golang-commonmark/markdown"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"jaytaylor.com/html2text"
)

const MAXREAD int64 = 5

type Thread struct {
	ID       string
	Subject  string
	Snippet  string
	Messages []*Message
}

type Message struct {
	From      string
	CC        string
	BCC       string
	Reply     string
	Body      string
	MessageID string
}

type ComposeParams struct {
	Mode     string
	To       string
	Bcc      string
	Cc       string
	Subject  string
	Body     string
	ThreadID string
}

type Mailer struct {
	Service          *gmail.Service
	User             string
	Threads          []*Thread
	Labels           []string
	Pages            []string
	CurrentPageIndex int
}

func NewMailer(creds []byte, label string) (*Mailer, error) {
	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(creds, gmail.MailGoogleComScope)
	if err != nil {
		return &Mailer{}, err
	}

	client, err := getClient(config)
	if err != nil {
		return &Mailer{}, err
	}

	srv, err := gmail.New(client)
	if err != nil {
		return &Mailer{}, err
	}

	resp, err := srv.Users.GetProfile("me").Do()
	if err != nil {
		return &Mailer{}, err
	}

	return &Mailer{
		Service: srv,
		User:    resp.EmailAddress,
		Labels:  strings.Split(label, ","),
		Pages:   []string{""},
	}, nil
}

func (mailer *Mailer) DeleteAll(labels []string) error {
	err := mailer.Service.Users.Messages.List(mailer.User).LabelIds(labels...).MaxResults(500).Pages(context.Background(), mailer.deleteMessages)
	if err != nil {
		return err
	}
	return nil
}

func (mailer *Mailer) ListLabels() error {
	resp, err := mailer.Service.Users.Labels.List(mailer.User).Do()
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.TabIndent)
	for _, label := range resp.Labels {
		fmt.Fprintf(w, "ID: %s \t\t\t\t\t\t\t\t\t Name: %s\n", label.Id, label.Name)
	}
	w.Flush()
	return nil
}

func (mailer *Mailer) ListMail(mode string) error {

	var err error
	var resp *gmail.ListThreadsResponse

	mailer.Threads = make([]*Thread, 0)
	switch mode {
	case "init":
		resp, err = mailer.Service.Users.Threads.List(mailer.User).LabelIds(mailer.Labels...).MaxResults(MAXREAD).Q("is:unread").Do()
		if err == nil {
			mailer.CurrentPageIndex = 0
			mailer.Pages = append(mailer.Pages, resp.NextPageToken)
		}
	case "next":
		resp, err = mailer.Service.Users.Threads.List(mailer.User).LabelIds(mailer.Labels...).MaxResults(MAXREAD).PageToken(mailer.Pages[mailer.CurrentPageIndex+1]).Q("is:unread").Do()
		if err == nil {
			mailer.Pages = append(mailer.Pages, resp.NextPageToken)
			mailer.CurrentPageIndex += 1
		}
	case "prev":
		if mailer.CurrentPageIndex == 0 {
			mailer.CurrentPageIndex = 1
		}
		resp, err = mailer.Service.Users.Threads.List(mailer.User).LabelIds(mailer.Labels...).MaxResults(MAXREAD).PageToken(mailer.Pages[mailer.CurrentPageIndex-1]).Q("is:unread").Do()
		mailer.CurrentPageIndex -= 1
	case "reload":
		resp, err = mailer.Service.Users.Threads.List(mailer.User).LabelIds(mailer.Labels...).MaxResults(MAXREAD).PageToken(mailer.Pages[mailer.CurrentPageIndex]).Q("is:unread").Do()
		if len(resp.Threads) == 0 {
			// Call Previous Page if current Page is empty upon reload
			return mailer.ListMail("prev")
		}
	}

	if err != nil {
		return err
	}

	for _, thread := range resp.Threads {
		resp, err := mailer.Service.Users.Threads.Get(mailer.User, thread.Id).Format("full").Do()
		if err != nil {
			return err
		}
		curThread := &Thread{ID: thread.Id, Snippet: thread.Snippet}
		for _, msg := range resp.Messages {
			curMsg := &Message{}
			for _, header := range msg.Payload.Headers {
				switch header.Name {
				case "Subject":
					if curThread.Subject == "" {
						curThread.Subject = header.Value
					}
				case "From":
					curMsg.From = header.Value
				case "Cc":
					curMsg.CC = header.Value
				case "Bcc":
					curMsg.BCC = header.Value
				case "Reply-To":
					curMsg.Reply = header.Value
				case "Message-ID":
					curMsg.MessageID = header.Value
				}
			}
			curMsg.ExtractMessage(msg)
			curThread.Messages = append(curThread.Messages, curMsg)
		}
		mailer.Threads = append(mailer.Threads, curThread)
	}
	return err
}

func (mailer *Mailer) MarkAsRead(thread *Thread) error {
	modReq := &gmail.ModifyThreadRequest{
		RemoveLabelIds: []string{"UNREAD"},
	}
	_, err := mailer.Service.Users.Threads.Modify(mailer.User, thread.ID, modReq).Do()
	if err != nil {
		return err
	}
	return nil
}

func (mailer *Mailer) ComposeAndSend(params *ComposeParams, replyID string) error {

	var headers string
	md := markdown.New(markdown.XHTMLOutput(true))
	msg := md.RenderToString([]byte(params.Body))

	// RFC2822 Format for EMAIL Messages
	// Headers \r\n\r\n
	// Body
	logger.NewLogger().Infof("Sending Email with params: %v", params)
	switch params.Mode {
	case "forward":
		fallthrough
	case "new":
		headers = "From: " + mailer.User + "\r\n" +
			"Reply-To: " + mailer.User + "\r\n" +
			"To: " + params.To + "\r\n" +
			"Cc: " + params.Cc + "\r\n" +
			"Bcc: " + params.Bcc + "\r\n" +
			"Subject: " + params.Subject + " \r\n" +
			"Content-Type: text/html;\r\n\r\n"
	case "reply":
		reply := strings.Split(replyID, " ")
		headers = "From: " + mailer.User + "\r\n" +
			"Reply-To: " + mailer.User + "\r\n" +
			"To: " + params.To + "\r\n" +
			"Cc: " + params.Cc + "\r\n" +
			"Bcc: " + params.Bcc + "\r\n" +
			"Subject: " + params.Subject + " \r\n" +
			"In-Reply-To: " + reply[len(reply)-1] + " \r\n" +
			"References: " + replyID + " \r\n" +
			"Content-Type: text/html;\r\n\r\n"
	default:
	}
	return mailer.sendMail(base64.URLEncoding.EncodeToString([]byte(headers+msg)), params.ThreadID)
}

func (mailer *Mailer) sendMail(msg, threadId string) error {
	mesg := &gmail.Message{}
	mesg.Raw = msg
	mesg.ThreadId = threadId
	_, err := mailer.Service.Users.Messages.Send(mailer.User, mesg).Do()
	return err
}

func (mailer *Mailer) deleteMessages(r *gmail.ListMessagesResponse) error {
	msgIds := make([]string, 0)
	for _, l := range r.Messages {
		msgIds = append(msgIds, l.Id)
	}
	err := mailer.Service.Users.Messages.BatchDelete(mailer.User, &gmail.BatchDeleteMessagesRequest{Ids: msgIds}).Do()
	return err
}

func (m *Message) ExtractMessage(msg *gmail.Message) {
	var body []byte
	mimeType := strings.Split(msg.Payload.MimeType, "/")
	switch mimeType[0] {
	case "multipart":
		curParts := make([]*gmail.MessagePart, 0)
		for _, part := range msg.Payload.Parts {
			if len(part.Parts) > 0 {
				curParts = append(curParts, part.Parts...)
			}
		}
		if len(curParts) == 0 {
			curParts = append(curParts, msg.Payload.Parts...)
		}
		for _, part := range curParts {
			var cType string
			for _, header := range part.Headers {
				if header.Name == "Content-Type" {
					cType = header.Value
				}
			}
			if strings.Contains(cType, "text/plain") {
				body, _ = base64.URLEncoding.DecodeString(part.Body.Data)
				m.Body += strings.Replace(strings.Replace(strings.Replace(string(body), "<p>", "", -1), "</p>", "", -1), "\r", "", -1)
			}
		}
	case "text":
		body, _ = base64.URLEncoding.DecodeString(msg.Payload.Body.Data)
		text, _ := html2text.FromString(string(body), html2text.Options{PrettyTables: true})
		m.Body += text
	default:
	}
}

func FetchToken(creds []byte) error {
	tokFile := "configs/token.json"
	os.Remove(tokFile)
	config, err := google.ConfigFromJSON(creds, gmail.MailGoogleComScope)
	if err != nil {
		return err
	}

	tok := getTokenFromWeb(config)
	saveToken(tokFile, tok)
	return nil
}

// Retrieve a token, saves the token, then returns the generated client.
func getClient(config *oauth2.Config) (*http.Client, error) {
	tokFile := "configs/token.json"
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		return &http.Client{}, err
	}
	return config.Client(context.Background(), tok), nil
}

// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Open the following link in your browser: \n%v\n", authURL)

	var authCode string
	fmt.Printf("Enter Auth Code: ")
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
