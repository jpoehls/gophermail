package gophermail

import (
	"bytes"
	"net/mail"
	"testing"
	"time"
)

func Test_Bytes(t *testing.T) {
	m := &Message{}
	m.From = "sender@domain.com"
	m.To = []string{"to_1@domain.com", "to_2@domain.com"}
	m.Subject = "My Subject (abcdefghijklmnop qrstuvwxyz0123456789 abcdefghijklmnopqrstuvwxyz0123456789_567890)"
	m.Body = "My Plain Text Body"
	m.HTMLBody = *bytes.NewBufferString("<p>My <b>HTML</b> Body</p>")
	m.Headers = mail.Header{}
	m.Headers["Date"] = []string{time.Now().UTC().Format(time.RFC822)}

	bytes, err := m.Bytes()
	if err != nil {
		t.Log(err)
		t.Fail()
	}

	t.Logf("%s", bytes)
}
