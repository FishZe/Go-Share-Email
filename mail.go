package main

import (
	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	_ "github.com/emersion/go-message/charset"
	"github.com/emersion/go-message/mail"
	"io"
	"log"
	"strconv"
	"time"
)

type Mail struct {
	From        string   `json:"from"`
	To          string   `json:"to"`
	TimeStamp   int      `json:"data"`
	Subject     string   `json:"subject"`
	PlainText   []string `json:"plain_text"`
	HTMLText    []string `json:"html_text"`
	Attachments []string `json:"attachments"`
}

var c *client.Client
var lastCheckSum uint32
var mailUpdateTime time.Time
var connected bool

func connectImap() {
	for {
		if !connected {
			err := initImap()
			if err == nil {
				connected = true
			}
		}
		time.Sleep(time.Second * 5)
	}
}

func initImap() error {
	connected = false
	var err error
	c, err = client.DialTLS(conf.EmailImapHost+":"+strconv.Itoa(conf.EmailImapPort), nil)
	if err != nil {
		log.Println("DialTLS error:", err)
		return err
	}
	if err := c.Login(conf.EmailAccount, conf.EmailPassword); err != nil {
		log.Println("Login error:", err)
		return err
	}
	inBox, err := c.Select("INBOX", false)
	if err != nil {
		lastCheckSum = 0
		log.Println("Select error:", err)
		return err
	}
	lastCheckSum = inBox.Messages
	mailUpdateTime = time.Now()
	connected = true
	return nil
}

func getMessage(mailChan chan Mail) {
	inBox, err := c.Select("INBOX", false)
	if err != nil {
		log.Println("Select error:", err)
		connected = false
		connectImap()
	}
	if inBox.Messages == lastCheckSum {
		return
	}
	var section imap.BodySectionName
	seqSet := new(imap.SeqSet)
	seqSet.AddRange(inBox.Messages, lastCheckSum+1)
	items := []imap.FetchItem{section.FetchItem()}
	messages := make(chan *imap.Message, 1)
	go func() {
		if err := c.Fetch(seqSet, items, messages); err != nil {
			log.Println("Fetch error:", err)
		}
	}()
	for msg := range messages {
		if msg == nil {
			log.Println("Server didn't returned message")
			continue
		}
		r := msg.GetBody(&section)
		if r == nil {
			log.Println("Server didn't returned message body")
			continue
		}
		mr, err := mail.CreateReader(r)
		if err != nil {
			log.Println("CreateReader error:", err)
			continue
		}
		var newMail Mail
		header := mr.Header
		if to, err := header.AddressList("To"); err == nil {
			newMail.To = to[0].Address
		}
		if date, err := header.Date(); err == nil {
			newMail.TimeStamp = int(date.Unix())
		}
		if from, err := header.AddressList("From"); err == nil {
			newMail.From = from[0].Address
		}
		if subject, err := header.Subject(); err == nil {
			newMail.Subject = subject
		}
		for {
			p, err := mr.NextPart()
			if err == io.EOF {
				break
			} else if err != nil {
				log.Fatal(err)
			}
			switch h := p.Header.(type) {
			case *mail.InlineHeader:
				s, err := io.ReadAll(p.Body)
				if err != nil {
					log.Println("ReadAll error:", err)
					break
				}
				if h.Get("Content-Type")[0:9] == "text/html" {
					newMail.HTMLText = append(newMail.HTMLText, string(s))
				} else if h.Get("Content-Type")[0:10] == "text/plain" {
					newMail.PlainText = append(newMail.PlainText, string(s))
				}
			case *mail.AttachmentHeader:
				filename, _ := h.Filename()
				newMail.Attachments = append(newMail.Attachments, filename)
			}
		}
		mailChan <- newMail
	}
	lastCheckSum = inBox.Messages
	mailUpdateTime = time.Now()
}
