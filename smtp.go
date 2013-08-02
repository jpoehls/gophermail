package gophermail

import (
	"net/smtp"
)

// SendMail connects to the server at addr, switches to TLS if possible,
// authenticates with mechanism a if possible, and then sends an email from
// address from, to addresses to, with message msg.
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

	// TODO(JPOEHLS): hello() is private and it looks like other things call Hello implicitly, do we really need this?
	// if err := c.hello(); err != nil {
	// 	return err
	// }
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
	if err = c.Mail(msg.Sender); err != nil {
		return err
	}
	for _, rcptAddr := range msg.To {
		if err = c.Rcpt(rcptAddr); err != nil {
			return err
		}
	}
	for _, rcptAddr := range msg.Cc {
		if err = c.Rcpt(rcptAddr); err != nil {
			return err
		}
	}
	for _, rcptAddr := range msg.Bcc {
		if err = c.Rcpt(rcptAddr); err != nil {
			return err
		}
	}
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
