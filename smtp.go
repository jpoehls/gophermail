package gophermail

import (
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

	var to []string
	for _, address := range msg.To {
		to = append(to, address.Address)
	}

	for _, address := range msg.Cc {
		to = append(to, address.Address)
	}

	for _, address := range msg.Bcc {
		to = append(to, address.Address)
	}

	return smtp.SendMail(addr, a, msg.From.Address, to, msgBytes)
}
