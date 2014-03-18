package gophermail

import (
	"net/mail"
	"testing"
	"time"
)

func Test_Bytes(t *testing.T) {
	m := &Message{}
	m.SetFrom("Doman Sender <sender@domain.com>")
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
