package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/textproto"
	"strings"
)

const crlf = "\r\n"

var ErrMissingRecipient = errors.New("No recipient specified. At one To, Cc, or Bcc recipient is required.")
var ErrMissingBody = errors.New("No body specified.")
var ErrMissingSender = errors.New("No sender specified.")

// A Message represents an email message.
// Addresses may be of any form permitted by RFC 822.
type Message struct {
	Sender  string
	ReplyTo string // optional

	// At least one of these slices must have a non-zero length.
	To, Cc, Bcc []string

	Subject string // optional

	// At least one of Body or HTMLBody must be non-empty.
	Body     string
	HTMLBody string

	Attachments []Attachment // optional

	// TODO(JPOEHLS): Support extra mail headers? Things on On-Behalf-Of, In-Reply-To, List-Unsubscribe, etc.
}

// An Attachment represents an email attachment.
type Attachment struct {
	// Name must be set to a valid file name.
	Name string
	Data []byte

	// TODO(JPOEHLS): Does it make sense to support an io.Reader instead of (or in addition to?) []byte so that data can be streamed in to save memory?
}

func (m *Message) Build() ([]byte, error) {
	var buffer = &bytes.Buffer{}

	// Require To, Cc, or Bcc
	var hasTo = m.To != nil && len(m.To) > 0
	var hasCc = m.Cc != nil && len(m.Cc) > 0
	var hasBcc = m.Bcc != nil && len(m.Bcc) > 0

	if !hasTo && !hasCc && !hasBcc {
		return nil, ErrMissingRecipient
	} else {
		if hasTo {
			writeHeader(buffer, "To", strings.Join(m.To, ","))
		}
		if hasCc {
			writeHeader(buffer, "Cc", strings.Join(m.Cc, ","))
		}
		if hasBcc {
			writeHeader(buffer, "Bcc", strings.Join(m.Bcc, ","))
		}
	}

	// Require Body or HTMLBody
	// TODO(JPOEHLS): Is a body is technically required by MIME? If not, we shouldn't require it either.
	if m.Body == "" && m.HTMLBody == "" {
		return nil, ErrMissingBody
	}

	// Require Sender
	if m.Sender == "" {
		return nil, ErrMissingSender
	} else {
		writeHeader(buffer, "From", m.Sender)
	}

	// Optional ReplyTo
	if m.ReplyTo != "" {
		writeHeader(buffer, "Reply-To", m.ReplyTo)
	}

	// Optional Subject
	if m.Subject != "" {
		writeHeader(buffer, "Subject", m.Subject)
	}

	mixedw := multipart.NewWriter(buffer)

	writeHeader(buffer, "MIME-Version", "1.0")
	writeHeader(buffer, "Content-Type", fmt.Sprintf("multipart/mixed; boundary=%s", mixedw.Boundary()))

	// Add a spacer between our header and the first part.
	write(buffer, crlf)

	var header textproto.MIMEHeader

	if m.Body != "" || m.HTMLBody != "" {

		altw := multipart.NewWriter(buffer)
		writeHeader(buffer, "Content-Type", fmt.Sprintf("multipart/alternative; boundary=%s", altw.Boundary()))

		// Add a spacer between our header and the first part.
		write(buffer, crlf)

		if m.Body != "" {
			header = textproto.MIMEHeader{}
			header.Add("Content-Type", "text/plain; charset=UTF8")
			// TODO(JPOEHLS): Do we need a Content-Transfer-Encoding: quoted-printable header? What does that mean?

			partw, err := altw.CreatePart(header)
			if err != nil {
				return nil, err
			}

			err = write(
				partw,
				// TODO(JPOEHLS): Do we need to escape any specific characters in the body or wrap long lines?
				m.Body,
			)
			if err != nil {
				return nil, err
			}
		}

		if m.HTMLBody != "" {
			header = textproto.MIMEHeader{}
			header.Add("Content-Type", "text/html; charset=UTF8")
			// TODO(JPOEHLS): Do we need a Content-Transfer-Encoding: quoted-printable header? What does that mean?

			partw, err := altw.CreatePart(header)
			if err != nil {
				return nil, err
			}

			err = write(
				partw,
				// TODO(JPOEHLS): Do we need to escape any specific characters in the body or wrap long lines?
				m.HTMLBody,
			)
			if err != nil {
				return nil, err
			}
		}

		altw.Close()
	}

	if m.Attachments != nil && len(m.Attachments) > 0 {
		// TODO(JPOEHLS): Write the attachment parts.
	}

	mixedw.Close()

	return buffer.Bytes(), nil
}

func writeHeader(w io.Writer, key, value string) {
	// TODO(JPOEHLS): Do we need to worry about escaping certain characters in the value or wrapping long values to multiple lines? For example a really long recipient list.
	write(w, fmt.Sprintf("%s: %s%s", key, value, crlf))
}

// Helper method to make writing to an io.Writer over and over nicer.
func write(w io.Writer, data ...string) error {
	for _, part := range data {
		_, err := w.Write([]byte(part))
		if err != nil {
			return err
		}
	}
	return nil
}
