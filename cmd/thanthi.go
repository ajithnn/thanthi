package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/ajithnn/thanthi/app"
	"github.com/ajithnn/thanthi/render"
	"github.com/gobuffalo/packr"
)

func main() {
	mode := flag.String("m", "labels", "send - To send Emails|read - Read emails|clear - Clear all for given labels|labels - List valid labels")
	subject := flag.String("s", "subject", "EMail Subject for send mode")
	to := flag.String("t", "to", "comma separated 'TO' list for send mode")
	cc := flag.String("cc", "", "comma separated 'CC' list for send mode")
	bcc := flag.String("bcc", "", "comma separated 'BCC' list for send mode")
	file := flag.String("f", "", "File containing EMail body in md format for send mode")
	label := flag.String("l", "IMPORTANT", "comma separated Labels needed for clear and read modes")
	configure := flag.Bool("configure", false, "Used configure oauth creds for account.Re-run to change account.")

	flag.Parse()

	box := packr.NewBox("../configs/")
	creds, err := box.Find("credentials.json")
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	if *configure {
		err := app.FetchToken(creds)
		if err != nil {
			log.Fatalf("Unable to configure Token. %v", err)
		}
		fmt.Println("Successfully configured")
		os.Exit(0)
	}

	mailer, err := app.NewMailer(creds, *label)
	if err != nil {
		log.Fatalf("Unable to create client handler: %v", err)
	}

	switch *mode {
	case "clear":
		labels := strings.Split(*label, ",")
		err = mailer.DeleteAll(labels)
	case "send":
		msg := ""
		if *file != "" {
			msg = readMailFromFile(*file)
		} else {
			msg = readMailBody()
		}
		err = mailer.SendMail(*to, *subject, *cc, *bcc, msg)
	case "read":
		r, err := render.NewRenderer(mailer)
		err = mailer.ListMail("init")
		if err == nil {
			defer r.Close()
			r.Show()
		}
	case "labels":
		err = mailer.ListLabels()
	default:
		log.Fatalf("Unknown mode: Usage thanthi -m send|delete-all|read [options]")
	}

	if err != nil {
		log.Fatalf("Command Failed: %v", err)
	}
}

func readMailFromFile(filePath string) string {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return ""
	}
	return string(data)
}

func readMailBody() string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Enter the Mail body")
	text, _ := reader.ReadString('\n')
	txt := text
	fmt.Println("end with <<<EOM")
	for {
		txt, _ = reader.ReadString('\n')
		if txt == "<<<EOM\n" {
			break
		}
		text += txt
	}
	return text
}
