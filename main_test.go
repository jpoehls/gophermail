package main

import (
	"testing"
)

func Test_Build(t *testing.T) {
	m := &Message{}
	m.Sender = "sender@domain.com"
	m.To = []string{"to_1@domain.com", "to_2@domain.com"}
	m.Subject = "My Subject"
	m.Body = "My Plain Tex Body"
	m.HTMLBody = "<p>My <b>HTML</b> Body</p>"

	bytes, err := m.Build()
	if err != nil {
		t.Log(err)
		t.Fail()
	}

	t.Logf("%s", bytes)
}
