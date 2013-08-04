package gophermail

import (
	"net/mail"
	"net/smtp"
)

// SendMail connects to the server at addr, switches to TLS if possible,
// authenticates with mechanism a if possible, and then sends the given Message.
//
// Based heavily on smtp.SendMail().
func SendMail(addr string, a smtp.Auth, msg *Message) error {

	msgBytes, err := msg.Bytes()
	if err != nil {
		return err
	}

	c, err := smtp.Dial(addr)
	if err != nil {
		return err
	}

	if ok, _ := c.Extension("STARTTLS"); ok {
		if err = c.StartTLS(nil); err != nil {
			return err
		}
	}
	if a != nil {
		if ok, _ := c.Extension("AUTH"); ok {
			if err = c.Auth(a); err != nil {
				return err
			}
		}
	}

	// Sender
	parsedAddr, err := parseAddress(msg.From)
	if err != nil {
		return err
	}
	if err = c.Mail(parsedAddr); err != nil {
		return err
	}

	// To
	err = rcpt(c, msg.To)
	if err != nil {
		return err
	}

	// CC
	err = rcpt(c, msg.Cc)
	if err != nil {
		return err
	}

	// BCC
	w, err := c.Data()
	if err != nil {
		return err
	}
	_, err = w.Write(msgBytes)
	if err != nil {
		return err
	}
	err = w.Close()
	if err != nil {
		return err
	}
	return c.Quit()
}

// rcpt parses the specified list of RFC 5322 addresses
// and calls smtp.Client.Rcpt() with each one.
func rcpt(c *smtp.Client, addresses []string) error {
	for _, rcptAddr := range addresses {
		parsedAddr, err := parseAddress(rcptAddr)
		if err != nil {
			return err
		}

		if err = c.Rcpt(parsedAddr); err != nil {
			return err
		}
	}

	return nil
}

// parseAddress parses a single RFC 5322 address and returns the
// e-mail address portion.
// e.g. "Barry Gibbs <bg@example.com>" would return "bg@example.com".
func parseAddress(address string) (string, error) {
	a, err := mail.ParseAddress(address)
	if err != nil {
		return "", err
	}
	return a.Address, err
}
