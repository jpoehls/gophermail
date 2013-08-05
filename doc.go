/*
gophermail is a simple package for sending mail using net/smtp.

Features:
	- Poviding both plain text and HTML message bodies
	- Attachments with data fed from an io.Reader
	- Reply-To header
	- To, Cc, and Bcc recipients

Notes:
	- UTF-8 encoding is always assumed.
	- Message bodies are base64 encoded instead of
	  the more readable quoted-printable encoding.

Known Issues:
	- `Subject: ` headers longer than 75 characters are not
	  wrapped into multiple encoded-words as per RFC 2047.
	  Use short subjects.

TODO:
	- Use quoted-printable encoding for message bodies.
	- Properly wrap `Subject:` headers longer than 75 characters.
	- Add support for `Sender:` header.
	- Add support for multiple `From:` and `Reply-To:` addresses.
	- Auto-add a `Date:` header. i.e. "Date: " + time.Now().UTC().Format(time.RFC822)
*/
package gophermail
