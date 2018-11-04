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
	"gitlab.com/golang-commonmark/markdown"
)

func main() {
	mode := flag.String("m", "send", "send|delete-all|read")
	subject := flag.String("s", "subject", "Mail Subject if send")
	to := flag.String("t", "to", "Mail To, comma separated list")
	cc := flag.String("cc", "", "Mail cc comma separated list")
	file := flag.String("f", "", "Mail Body file")
	label := flag.String("l", "IMPORTANT", "comma separated Labels for delete-all and read")

	flag.Parse()

	box := packr.NewBox("../configs/")

	creds, err := box.Find("credentials.json")
	//creds, err := ioutil.ReadFile("configs/credentials.json")
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	mailer, err := app.NewMailer(creds, *label)
	if err != nil {
		log.Fatalf("Unable to create client handler: %v", err)
	}

	switch *mode {
	case "delete-all":
		labels := strings.Split(*label, ",")
		err = mailer.DeleteAll(labels)
	case "send":
		msg := ""
		if *file != "" {
			msg = readMailFromFile(*file)
		} else {
			msg = readMailBody()
		}
		md := markdown.New(markdown.XHTMLOutput(true))
		err = mailer.SendMail(*to, *subject, *cc, md.RenderToString([]byte(msg)))
	case "ui":
		//		r, err := render.NewRenderer()
		//		if err == nil {
		//			defer r.Close()
		//			r.Show()
		//		}
	case "test":
		r, err := render.NewRenderer(mailer)
		err = mailer.ListMail("init")
		if err == nil {
			defer r.Close()
			r.Show()
		}
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
