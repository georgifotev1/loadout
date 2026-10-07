package notify

import (
	"bytes"
	"context"
	"io"
	"mime"
	"mime/quotedprintable"
	"net/mail"
	"strings"
	"testing"
	"time"
)

type recorder struct{ sent []Message }

func (r *recorder) Send(_ context.Context, m Message) error {
	r.sent = append(r.sent, m)
	return nil
}

func TestLinks(t *testing.T) {
	r := &recorder{}
	s := NewService(r, "https://loadout.bg/")
	ctx := context.Background()
	if err := s.VerifyEmail(ctx, "a@b.bg", "tok+/="); err != nil {
		t.Fatal(err)
	}
	if err := s.ResetPassword(ctx, "a@b.bg", "abc"); err != nil {
		t.Fatal(err)
	}
	if err := s.AccountExists(ctx, "a@b.bg"); err != nil {
		t.Fatal(err)
	}

	for i, want := range []string{
		"https://loadout.bg/verify-email?token=tok%2B%2F%3D\n",
		"https://loadout.bg/reset-password?token=abc\n",
		"https://loadout.bg/forgot-password\n",
	} {
		if !strings.Contains(r.sent[i].Text, want) {
			t.Errorf("email %d doesn't contain %q:\n%s", i, want, r.sent[i].Text)
		}
	}
}

func TestBuildMessage(t *testing.T) {
	from := &mail.Address{Name: "Loadout", Address: "noreply@loadout.bg"}
	to := &mail.Address{Address: "иван@example.bg"}
	text := "Здравейте,\n\n" + strings.Repeat("дълъг ред ", 20) + "\n"
	raw := buildMessage(from, to, Message{Subject: "Потвърдете имейла си", Text: text}, time.Unix(0, 0))

	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	subject, err := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject"))
	if err != nil || subject != "Потвърдете имейла си" {
		t.Errorf("subject %q, %v", subject, err)
	}
	if !strings.HasSuffix(msg.Header.Get("Message-ID"), "@loadout.bg>") {
		t.Errorf("Message-ID %q", msg.Header.Get("Message-ID"))
	}
	body, err := io.ReadAll(quotedprintable.NewReader(msg.Body))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.ReplaceAll(string(body), "\r\n", "\n"); got != text {
		t.Errorf("body %q, want %q", got, text)
	}
	for _, line := range strings.Split(string(raw), "\r\n") {
		if len(line) > 78 {
			t.Errorf("line longer than 78 bytes: %q", line)
		}
	}
}
