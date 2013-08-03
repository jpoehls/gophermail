package gophermail

import (
	"bytes"
	"fmt"
	"testing"
)

func TestBase64MimeEncoder(t *testing.T) {
	dest := bytes.NewBufferString("")
	encoder := NewBase64MimeEncoder(dest)

	_, err := fmt.Fprint(encoder, "Hello World!")
	if err != nil {
		t.Log(err)
		t.FailNow()
	}
	err = encoder.Close()
	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	output := dest.String()
	t.Log(output)

	if output != "SGVsbG8gV29ybGQh" {
		t.Fail()
	}
}
