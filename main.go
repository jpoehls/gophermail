package gophermail

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/sloonz/go-qprintable"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"net/textproto"
	"path/filepath"
)

// Message Lint: http://tools.ietf.org/tools/msglint/

const crlf = "\r\n"

var ErrMissingRecipient = errors.New("No recipient specified. At one To, Cc, or Bcc recipient is required.")
var ErrMissingFromAddress = errors.New("No from address specified.")

// A Message represents an email message.
// Addresses may be of any form permitted by RFC 5322.
type Message struct {
	// TODO(JPOEHLS): Add support for specifying the Sender header.

	// Technically this could be a list of addresses but we don't support that. See RFC 2822 s3.6.2.
	From string

	// Technically this could be a list of addresses but we don't support that. See RFC 2822 s3.6.2.
	ReplyTo string // optional

	// TODO(JPOEHLS): Make these slices of mail.Address and add AddTo, AddCc, AddBcc funcs that take strings. (Inspired by gopostal's API, https://github.com/pcrawfor/gopostal)
	// At least one of these slices must have a non-zero length.
	To, Cc, Bcc []string

	Subject string // optional

	Body     string       // optional
	HTMLBody bytes.Buffer //optional

	Attachments []Attachment // optional

	// Extra mail headers.
	Headers mail.Header
}

// An Attachment represents an email attachment.
type Attachment struct {
	// Name must be set to a valid file name.
	Name string

	// Optional.
	// Uses mime.TypeByExtension and falls back
	// to application/octet-stream if unknown.
	ContentType string

	Data io.Reader
}

// Bytes gets the encoded MIME message.
func (m *Message) Bytes() ([]byte, error) {
	var buffer = &bytes.Buffer{}

	header := textproto.MIMEHeader{}

	var err error

	// Require To, Cc, or Bcc
	// We'll parse the slices into a list of addresses
	// and then make sure that list isn't empty.
	toAddrs, err := getAddressListString(m.To)
	if err != nil {
		return nil, err
	}
	ccAddrs, err := getAddressListString(m.Cc)
	if err != nil {
		return nil, err
	}
	bccAddrs, err := getAddressListString(m.Bcc)
	if err != nil {
		return nil, err
	}

	var hasTo = toAddrs != ""
	var hasCc = ccAddrs != ""
	var hasBcc = bccAddrs != ""

	if !hasTo && !hasCc && !hasBcc {
		return nil, ErrMissingRecipient
	} else {
		if hasTo {
			header.Add("To", toAddrs)
		}
		if hasCc {
			header.Add("Cc", ccAddrs)
		}
		// BCC header is excluded on purpose.
		// BCC recipients aren't included in the message
		// headers and are only used at the SMTP level.
	}

	// Require From address
	if m.From == "" {
		return nil, ErrMissingFromAddress
	} else {
		parsedAddy, err := mail.ParseAddress(m.From)
		if err != nil {
			return nil, err
		}

		header.Add("From", parsedAddy.String())
	}

	// Optional ReplyTo
	if m.ReplyTo != "" {
		parsedAddy, err := mail.ParseAddress(m.ReplyTo)
		if err != nil {
			return nil, err
		}

		header.Add("Reply-To", parsedAddy.String())
	}

	// Optional Subject
	if m.Subject != "" {
		header.Add("Subject", qEncodeAndWrap(m.Subject, 9 /* len("Subject: ") */))
	}

	for k, v := range m.Headers {
		header[k] = v
	}

	// Top level multipart writer for our `multipart/mixed` body.
	mixedw := multipart.NewWriter(buffer)

	header.Add("MIME-Version", "1.0")
	header.Add("Content-Type", fmt.Sprintf("multipart/mixed;%s boundary=%s", crlf, mixedw.Boundary()))

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
	if m.Body != "" || m.HTMLBody.Len() != 0 {

		// Nested multipart writer for our `multipart/alternative` body.
		altw := multipart.NewWriter(buffer)

		header = textproto.MIMEHeader{}
		header.Add("Content-Type", fmt.Sprintf("multipart/alternative;%s boundary=%s", crlf, altw.Boundary()))
		err := writeHeader(buffer, header)
		if err != nil {
			return nil, err
		}

		if m.Body != "" {
			header = textproto.MIMEHeader{}
			header.Add("Content-Type", "text/plain; charset=utf-8")
			header.Add("Content-Transfer-Encoding", "quoted-printable")
			//header.Add("Content-Transfer-Encoding", "base64")

			partw, err := altw.CreatePart(header)
			if err != nil {
				return nil, err
			}

			bodyBytes := []byte(m.Body)
			//encoder := NewBase64MimeEncoder(partw)
			encoder := qprintable.NewEncoder(qprintable.DetectEncoding(m.Body), partw)
			_, err = encoder.Write(bodyBytes)
			if err != nil {
				return nil, err
			}
			err = encoder.Close()
			if err != nil {
				return nil, err
			}
		}

		if m.HTMLBody.Len() != 0 {
			header = textproto.MIMEHeader{}
			header.Add("Content-Type", "text/html; charset=utf-8")
			//header.Add("Content-Transfer-Encoding", "quoted-printable")
			header.Add("Content-Transfer-Encoding", "base64")

			partw, err := altw.CreatePart(header)
			if err != nil {
				return nil, err
			}

			//encoder := qprintable.NewEncoder(qprintable.DetectEncoding(m.HTMLBody), partw)
			encoder := NewBase64MimeEncoder(partw)
			_, err = io.Copy(encoder, &m.HTMLBody)
			if err != nil {
				return nil, err
			}
			err = encoder.Close()
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
			header.Add("Content-Disposition", fmt.Sprintf(`attachment;%s filename="%s"`, crlf, attachment.Name))
			header.Add("Content-Transfer-Encoding", "base64")

			attachmentPart, err := mixedw.CreatePart(header)
			if err != nil {
				return nil, err
			}

			if attachment.Data != nil {
				encoder := NewBase64MimeEncoder(attachmentPart)
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

	}

	mixedw.Close()

	return buffer.Bytes(), nil
}

// writeHeader writes the specified MIMEHeader to the io.Writer.
// Header values will be trimmed but otherwise left alone.
// Headers with multiple values are not supported and will return an error.
func writeHeader(w io.Writer, header textproto.MIMEHeader) error {
	for k, vs := range header {
		_, err := fmt.Fprintf(w, "%s: ", k)
		if err != nil {
			return err
		}

		for i, v := range vs {
			v = textproto.TrimString(v)

			_, err := fmt.Fprintf(w, "%s", v)
			if err != nil {
				return err
			}

			if i < len(vs)-1 {
				return errors.New("Multiple header values are not supported.")
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

// qEncode encodes a string with Q encoding defined as an 'encoded-word' in RFC 2047.
// The maximum encoded word length of 75 characters is not accounted for.
// Use qEncodeAndWrap if you need that.
//
// Inspired by https://gist.github.com/andelf/5004821
func qEncode(input string) string {
	// use mail's rfc2047 to encode any string
	addr := mail.Address{input, ""}
	s := addr.String()
	return s[:len(s)-3]
}

// qEncodeAndWrap encodes the input as potentially multiple 'encoded-words'
// with CRLF SPACE line breaks between them to (as best as possible)
// guarantee that each encoded-word is no more than 75 characters
// and, padding included, each line is no longer than 76 characters.
// See RFC 2047 s2.
func qEncodeAndWrap(input string, padding int) string {

	// Split at any whitespace but prefer "; " or  ", " or " >" or "> " which
	// denotes a clear semantic break.
	// Remember that the qEncoded input isn't guaranteed to have the same
	// length as the unencoded input (obvious). Example: http://play.golang.org/p/dXA5IJnL22

	// Increase the padding to account for
	// the encoded-word 'envelop' tokens.
	// "?" charset (utf-8 is always assumed) "?" encoding "?" encoded-text "?="
	padding += 11

	// Tokenization included, the encoded word must not
	// be longer than 75 characters.
	const maxEncodedWordLength = 75

	var firstTry = qEncode(input)
	if len(firstTry) > maxEncodedWordLength-padding {

		// TODO(JPOEHLS): Implement an algorithm to break the input into multiple encoded-words.

		return firstTry
	} else {
		return firstTry
	}
}

// getAddressListString encodes a slice of email addresses into
// a string value suitable for a MIME header. Each address is
// Q encoded and wrapped onto its own line to help ensure that
// the header line doesn't cross the 78 character maximum.
func getAddressListString(addresses []string) (string, error) {
	// Short circuit for empty or nil slices.
	if addresses == nil || len(addresses) == 0 {
		return "", nil
	}

	buffer := bytes.NewBuffer([]byte{})

	for i, v := range addresses {
		// Ignore empty addresses
		if v != "" {
			parsedAddy, err := mail.ParseAddress(v)
			if err != nil {
				return "", err
			}

			_, err = fmt.Fprint(buffer, parsedAddy.String())
			if err != nil {
				return "", err
			}

			// TODO(ANYONE): Currently we are splitting each address onto its own line
			//               because this is the easiest way to ensure lines don't exceed
			//               the max length. Ideally we wouldn't split until we actually
			//               needed to.

			// Separate multiple addresses with a comma
			// and wrap them to their own line using whitespace folding.
			// See RFC 2822 s2.2.3.
			if i < len(addresses)-1 {
				_, err := fmt.Fprint(buffer, ","+crlf+" ")
				if err != nil {
					return "", err
				}
			}
		}
	}

	return buffer.String(), nil
}
