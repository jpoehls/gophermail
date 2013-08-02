package gophermail

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/sloonz/go-qprintable"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"net/textproto"
	"path/filepath"
	"strings"
)

// TODO(JPOEHLS): Find out if we need to split headers > 76 chars into multiple lines.
// TODO(JPOEHLS): Play with using base64 (split into 76 character lines) instead of quoted-printable. Benefit being removal of a non-core dependency, downside being a non-human readable mail encoding.
// TODO(JPOEHLS): Split base64 encoded attachments into lines of 76 chars
// TODO(JPOEHLS): Fix CC and BCC recipients - they are shown publically in the message and shouldn't be...
// TODO(JPOEHLS): Gmail says there is an encoding problem with the email when I receive it.

const crlf = "\r\n"

var ErrMissingRecipient = errors.New("No recipient specified. At one To, Cc, or Bcc recipient is required.")
var ErrMissingSender = errors.New("No sender specified.")

// A Message represents an email message.
// Addresses may be of any form permitted by RFC 822.
type Message struct {
	Sender  string
	ReplyTo string // optional

	// At least one of these slices must have a non-zero length.
	To, Cc, Bcc []string

	Subject string // optional

	Body     string // optional
	HTMLBody string // optional

	Attachments []Attachment // optional

	// TODO(JPOEHLS): Support extra mail headers? Things like On-Behalf-Of, In-Reply-To, List-Unsubscribe, etc.
}

// An Attachment represents an email attachment.
type Attachment struct {
	// Name must be set to a valid file name.
	Name        string
	ContentType string // optional
	Data        io.Reader
}

// Gets the encoded message data bytes.
func (m *Message) Bytes() ([]byte, error) {
	var buffer = &bytes.Buffer{}

	header := textproto.MIMEHeader{}

	// Require To, Cc, or Bcc
	var hasTo = m.To != nil && len(m.To) > 0
	var hasCc = m.Cc != nil && len(m.Cc) > 0
	var hasBcc = m.Bcc != nil && len(m.Bcc) > 0

	if !hasTo && !hasCc && !hasBcc {
		return nil, ErrMissingRecipient
	} else {
		if hasTo {
			toAddrs, err := getAddressListString(m.To)
			if err != nil {
				return nil, err
			}
			header.Add("To", toAddrs)
		}
		if hasCc {
			ccAddrs, err := getAddressListString(m.Cc)
			if err != nil {
				return nil, err
			}
			header.Add("Cc", ccAddrs)
		}
		if hasBcc {
			bccAddrs, err := getAddressListString(m.Bcc)
			if err != nil {
				return nil, err
			}
			header.Add("Bcc", bccAddrs)
		}
	}

	// Require Sender
	if m.Sender == "" {
		return nil, ErrMissingSender
	} else {
		header.Add("From", m.Sender)
	}

	// Optional ReplyTo
	if m.ReplyTo != "" {
		header.Add("Reply-To", m.ReplyTo)
	}

	// Optional Subject
	if m.Subject != "" {
		header.Add("Subject", encodeRFC2047(m.Subject))
	}

	// Top level multipart writer for our `multipart/mixed` body.
	mixedw := multipart.NewWriter(buffer)

	var err error

	header.Add("MIME-Version", "1.0")
	header.Add("Content-Type", fmt.Sprintf("multipart/mixed; boundary=%s", mixedw.Boundary()))

	err = writeHeader(buffer, header)
	if err != nil {
		return nil, err
	}

	// Write the start of our `multipart/mixed` body.
	_, err = fmt.Fprintf(buffer, "--%s%s", mixedw.Boundary(), crlf)
	if err != nil {
		return nil, err
	}

	// Does the message have a body?
	if m.Body != "" || m.HTMLBody != "" {

		// Nested multipart writer for our `multipart/alternative` body.
		altw := multipart.NewWriter(buffer)

		header = textproto.MIMEHeader{}
		header.Add("Content-Type", fmt.Sprintf("multipart/alternative; boundary=%s", altw.Boundary()))
		err := writeHeader(buffer, header)
		if err != nil {
			return nil, err
		}

		if m.Body != "" {
			header = textproto.MIMEHeader{}
			header.Add("Content-Type", "text/plain; charset=UTF8")
			header.Add("Content-Transfer-Encoding", "quoted-printable")

			partw, err := altw.CreatePart(header)
			if err != nil {
				return nil, err
			}

			bodyBytes := []byte(m.Body)
			encoder := qprintable.NewEncoder(qprintable.DetectEncoding(m.Body), partw)
			_, err = encoder.Write(bodyBytes)
			if err != nil {
				return nil, err
			}
		}

		if m.HTMLBody != "" {
			header = textproto.MIMEHeader{}
			header.Add("Content-Type", "text/html; charset=UTF8")
			header.Add("Content-Transfer-Encoding", "quoted-printable")

			partw, err := altw.CreatePart(header)
			if err != nil {
				return nil, err
			}

			htmlBodyBytes := []byte(m.HTMLBody)
			encoder := qprintable.NewEncoder(qprintable.DetectEncoding(m.HTMLBody), partw)
			_, err = encoder.Write(htmlBodyBytes)
			if err != nil {
				return nil, err
			}
		}

		altw.Close()
	}

	if m.Attachments != nil && len(m.Attachments) > 0 {

		for _, attachment := range m.Attachments {

			contentType := attachment.ContentType
			if contentType == "" {
				contentType = mime.TypeByExtension(filepath.Ext(attachment.Name))
				if contentType == "" {
					contentType = "application/octet-stream"
				}
			}

			header := textproto.MIMEHeader{}
			header.Add("Content-Type", contentType)
			header.Add("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, attachment.Name))
			header.Add("Content-Transfer-Encoding", "base64")

			attachmentPart, err := mixedw.CreatePart(header)
			if err != nil {
				return nil, err
			}

			encoder := base64.NewEncoder(base64.StdEncoding, attachmentPart)
			_, err = io.Copy(encoder, attachment.Data)
			if err != nil {
				return nil, err
			}
			err = encoder.Close()
			if err != nil {
				return nil, err
			}
		}

	}

	mixedw.Close()

	return buffer.Bytes(), nil
}

var headerNewlineToSpace = strings.NewReplacer("\n", " ", "\r", " ")

func writeHeader(w io.Writer, header textproto.MIMEHeader) error {
	for k, vs := range header {
		_, err := fmt.Fprintf(w, "%s: ", k)
		if err != nil {
			return err
		}

		for i, v := range vs {
			// Clean the value like http.Header.Write() does.
			v = headerNewlineToSpace.Replace(v)
			v = textproto.TrimString(v)

			_, err := fmt.Fprintf(w, "%s", v)
			if err != nil {
				return err
			}

			// Separate multiple header values with a semicolon.
			if i < len(vs)-1 {
				_, err := fmt.Fprintf(w, "; ", v)
				if err != nil {
					return err
				}
			}
		}

		_, err = fmt.Fprint(w, crlf)
		if err != nil {
			return err
		}
	}

	// Write a blank line as a spacer
	_, err := fmt.Fprint(w, crlf)
	if err != nil {
		return err
	}

	return nil
}

// Inspired by https://gist.github.com/andelf/5004821
func encodeRFC2047(input string) string {
	// use mail's rfc2047 to encode any string
	addr := mail.Address{input, ""}
	s := addr.String()
	return s[:len(s)-3]
}

// Converts a list of mail.Address objects into a comma-delimited string.
func getAddressListString(addresses []string) (string, error) {
	return strings.Join(addresses, ","), nil
}
