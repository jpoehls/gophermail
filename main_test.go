package gophermail

import (
	"testing"
)

func Test_Bytes(t *testing.T) {
	m := &Message{}
	m.Sender = "sender@domain.com"
	m.To = []string{"to_1@domain.com", "to_2@domain.com"}
	m.Subject = "My Subject"
	m.Body = "My Plain Text Body"
	//m.HTMLBody = "<p>My <b>HTML</b> Body</p>"

	bytes, err := m.Bytes()
	if err != nil {
		t.Log(err)
		t.Fail()
	}

	t.Logf("%s", bytes)
}
