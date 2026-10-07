package notify

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

// SMTPSender sends through an SMTP server: Mailpit locally, the email provider's relay in
// production.
// ASSUMPTION: the provider (Brevo, Postmark, ...) is still to be picked; they all take SMTP with
// STARTTLS and a username and password, so only the settings change.
type SMTPSender struct {
	Addr     string // host:port
	From     string // "Loadout <noreply@loadout.bg>"
	Username string // empty: no AUTH (Mailpit)
	Password string
}

const smtpTimeout = 30 * time.Second

func (s *SMTPSender) Send(ctx context.Context, m Message) error {
	from, err := mail.ParseAddress(s.From)
	if err != nil {
		return fmt.Errorf("notify: from address: %w", err)
	}
	to, err := mail.ParseAddress(m.To)
	if err != nil {
		return fmt.Errorf("notify: to address: %w", err)
	}
	msg := buildMessage(from, to, m, time.Now())

	ctx, cancel := context.WithTimeout(ctx, smtpTimeout)
	defer cancel()
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", s.Addr)
	if err != nil {
		return fmt.Errorf("notify: dial %s: %w", s.Addr, err)
	}
	deadline, _ := ctx.Deadline()
	_ = conn.SetDeadline(deadline)
	host, _, _ := net.SplitHostPort(s.Addr)
	c, err := smtp.NewClient(conn, host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("notify: smtp: %w", err)
	}
	defer c.Close()

	if ok, _ := c.Extension("STARTTLS"); ok {
		if err := c.StartTLS(&tls.Config{ServerName: host}); err != nil {
			return fmt.Errorf("notify: starttls: %w", err)
		}
	}
	if s.Username != "" {
		// PlainAuth refuses to send the password without TLS, except to localhost.
		if err := c.Auth(smtp.PlainAuth("", s.Username, s.Password, host)); err != nil {
			return fmt.Errorf("notify: smtp auth: %w", err)
		}
	}
	if err := c.Mail(from.Address); err != nil {
		return fmt.Errorf("notify: smtp mail from: %w", err)
	}
	if err := c.Rcpt(to.Address); err != nil {
		return fmt.Errorf("notify: smtp rcpt to: %w", err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("notify: smtp data: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("notify: smtp write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("notify: smtp send: %w", err)
	}
	if err := c.Quit(); err != nil && !errors.Is(err, net.ErrClosed) {
		return fmt.Errorf("notify: smtp quit: %w", err)
	}
	return nil
}

// buildMessage formats m as a UTF-8 plain-text email. Non-ASCII headers (a Bulgarian subject)
// are RFC 2047 encoded and the body is quoted-printable, so it survives any mail server.
func buildMessage(from, to *mail.Address, m Message, now time.Time) []byte {
	var b bytes.Buffer
	header := func(k, v string) { fmt.Fprintf(&b, "%s: %s\r\n", k, v) }
	header("From", from.String())
	header("To", to.String())
	// One encoded word per line: lines shouldn't pass 78 characters.
	header("Subject", strings.ReplaceAll(mime.BEncoding.Encode("utf-8", m.Subject), " =?", "\r\n =?"))
	header("Date", now.Format(time.RFC1123Z))
	header("Message-ID", messageID(from.Address))
	header("MIME-Version", "1.0")
	header("Content-Type", `text/plain; charset="utf-8"`)
	header("Content-Transfer-Encoding", "quoted-printable")
	b.WriteString("\r\n")

	qp := quotedprintable.NewWriter(&b)
	_, _ = qp.Write(bytes.ReplaceAll([]byte(m.Text), []byte("\n"), []byte("\r\n")))
	_ = qp.Close()
	return b.Bytes()
}

func messageID(fromAddr string) string {
	domain := "localhost"
	if i := strings.LastIndexByte(fromAddr, '@'); i >= 0 {
		domain = fromAddr[i+1:]
	}
	id := make([]byte, 16)
	_, _ = rand.Read(id) // crypto/rand.Read never returns an error
	return "<" + hex.EncodeToString(id) + "@" + domain + ">"
}
