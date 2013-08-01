package gophermail

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/sloonz/go-qprintable"
	"io"
	"mime/multipart"
	"net/smtp"
	"net/textproto"
	"strings"
)

// TODO(JPOEHLS): Figure out how we should encode header values (Q encoding?)
// TODO(JPOEHLS): Refactor writeHeader() to accept a textproto.MIMEHeader
// TODO(JPOEHLS): Add support for attachments
// TODO(JPOEHLS): Add a SendMessage() SMTP function

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

	// TODO(JPOEHLS): Support extra mail headers? Things like On-Behalf-Of, In-Reply-To, List-Unsubscribe, etc.
}

// An Attachment represents an email attachment.
type Attachment struct {
	// Name must be set to a valid file name.
	Name string
	Data []byte

	// TODO(JPOEHLS): Does it make sense to support an io.Reader instead of (or in addition to?) []byte so that data can be streamed in to save memory?
}

// Gets the encoded message data bytes.
func (m *Message) Bytes() ([]byte, error) {
	var buffer = &bytes.Buffer{}

	// Require To, Cc, or Bcc
	var hasTo = m.To != nil && len(m.To) > 0
	var hasCc = m.Cc != nil && len(m.Cc) > 0
	var hasBcc = m.Bcc != nil && len(m.Bcc) > 0

	if !hasTo && !hasCc && !hasBcc {
		return nil, ErrMissingRecipient
	} else {
		if hasTo {
			err := writeHeader(buffer, "To", strings.Join(m.To, ","))
			if err != nil {
				return nil, err
			}
		}
		if hasCc {
			err := writeHeader(buffer, "Cc", strings.Join(m.Cc, ","))
			if err != nil {
				return nil, err
			}
		}
		if hasBcc {
			err := writeHeader(buffer, "Bcc", strings.Join(m.Bcc, ","))
			if err != nil {
				return nil, err
			}
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
		err := writeHeader(buffer, "From", m.Sender)
		if err != nil {
			return nil, err
		}
	}

	// Optional ReplyTo
	if m.ReplyTo != "" {
		err := writeHeader(buffer, "Reply-To", m.ReplyTo)
		if err != nil {
			return nil, err
		}
	}

	// Optional Subject
	if m.Subject != "" {
		err := writeHeader(buffer, "Subject", m.Subject)
		if err != nil {
			return nil, err
		}
	}

	mixedw := multipart.NewWriter(buffer)

	var err error

	err = writeHeader(buffer, "MIME-Version", "1.0")
	if err != nil {
		return nil, err
	}
	err = writeHeader(buffer, "Content-Type", fmt.Sprintf("multipart/mixed; boundary=%s", mixedw.Boundary()))
	if err != nil {
		return nil, err
	}

	// Add a spacer between our header and the first part.
	_, err = fmt.Fprint(buffer, crlf)
	if err != nil {
		return nil, err
	}

	var header textproto.MIMEHeader

	if m.Body != "" || m.HTMLBody != "" {

		altw := multipart.NewWriter(buffer)
		err := writeHeader(buffer, "Content-Type", fmt.Sprintf("multipart/alternative; boundary=%s", altw.Boundary()))
		if err != nil {
			return nil, err
		}

		// Add a spacer between our header and the first part.
		_, err = fmt.Fprint(buffer, crlf)
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
		// TODO(JPOEHLS): Write the attachment parts.
	}

	mixedw.Close()

	return buffer.Bytes(), nil
}

var headerNewlineToSpace = strings.NewReplacer("\n", " ", "\r", " ")

func writeHeader(w io.Writer, key, value string) error {
	// TODO(JPOEHLS): Do we need to worry about escaping certain characters in the value or wrapping long values to multiple lines? For example a really long recipient list.

	// Clean key and value like http.Header.Write() does.
	key = textproto.CanonicalMIMEHeaderKey(key)
	value = headerNewlineToSpace.Replace(value)
	value = textproto.TrimString(value)

	_, err := fmt.Fprintf(w, "%s: %s%s", key, value, crlf)
	return err
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

// SendMessage connects to the server at addr, switches to TLS if possible,
// authenticates with mechanism a if possible, and then sends an email from
// address from, to addresses to, with message msg.
func SendMessage(addr string, a smtp.Auth, msg *Message) error {

	// TODO(JPOEHLS): Make this work. Add support for RCPT BCC and RCPT CC commands. GAH!
	return nil

	/*	c, err := smtp.Dial(addr)
		if err != nil {
			return err
		}
		if err := c.hello(); err != nil {
			return err
		}
		if ok, _ := c.Extension("STARTTLS"); ok {
			if err = c.StartTLS(nil); err != nil {
				return err
			}
		}
		if a != nil && c.ext != nil {
			if _, ok := c.ext["AUTH"]; ok {
				if err = c.Auth(a); err != nil {
					return err
				}
			}
		}
		if err = c.Mail(from); err != nil {
			return err
		}
		for _, addr := range to {
			if err = c.Rcpt(addr); err != nil {
				return err
			}
		}
		w, err := c.Data()
		if err != nil {
			return err
		}
		_, err = w.Write(msg.Bytes())
		if err != nil {
			return err
		}
		err = w.Close()
		if err != nil {
			return err
		}
		return c.Quit()*/
}
