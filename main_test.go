package gophermail

import (
	"bytes"
	"net/mail"
	"net/textproto"
	"testing"
	"time"
)

func simpleMessage() *Message {
	m := &Message{}
	m.SetFrom("Domain Sender <sender@domain.com>")
	m.AddTo("First person <to_1@domain.com>")
	m.Subject = "My Subject"
	m.Body = "My Plain Text Body"

	return m
}

func Test_Bytes(t *testing.T) {
	m := &Message{}
	m.SetFrom("Domain Sender <sender@domain.com>")
	m.SetReplyTo("Don't reply <noreply@domain.com>")
	m.AddTo("First person <to_1@domain.com>")
	m.AddTo("to_2@domain.com")
	m.AddCc("Less important person <to_3@domain.com>")
	m.AddBcc("Sneaky person <to_4@domain.com>")

	m.Subject = "My Subject (abcdefghijklmnop qrstuvwxyz0123456789 abcdefghijklmnopqrstuvwxyz0123456789_567890)"
	m.Body = "My Plain Text Body"
	m.HTMLBody = "<p>My <b>HTML</b> Body</p>"
	m.Headers = mail.Header{}
	m.Headers["Date"] = []string{time.Now().UTC().Format(time.RFC822)}

	bytes, err := m.Bytes()
	if err != nil {
		t.Log(err)
		t.Fail()
	}

	t.Logf("%s", bytes)
}

func TestSubjectHeaderWithSimpleQuoting(t *testing.T) {
	m := simpleMessage()
	m.Subject = "My Subject"
	buf := new(bytes.Buffer)
	header := textproto.MIMEHeader{}

	_, err := m.bytes(buf, header)
	if err != nil {
		t.Log(err)
		t.Fail()
	}

	expected := "My Subject"
	if sub := header.Get("Subject"); sub != expected {
		t.Logf(`Expected Subject to be "%s" but got "%s"`, expected, sub)
		t.Fail()
	}
}

func TestSubjectHeaderWithExistingQuotes(t *testing.T) {
	m := simpleMessage()
	m.Subject = `"Hi World"`
	buf := new(bytes.Buffer)
	header := textproto.MIMEHeader{}

	_, err := m.bytes(buf, header)
	if err != nil {
		t.Log(err)
		t.Fail()
	}

	expected := `\"Hi World\"`
	if sub := header.Get("Subject"); sub != expected {
		t.Logf(`Expected Subject to be "%s" but got "%s"`, expected, sub)
		t.Fail()
	}
}
