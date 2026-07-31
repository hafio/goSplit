package mail

import (
	"bufio"
	"context"
	"net"
	"strings"
	"testing"

	"github.com/hafio/gosplit/internal/config"
)

func TestNewLogMailerWhenSMTPUnset(t *testing.T) {
	m := New(&config.Config{FromEmail: "no-reply@x.test"})
	lm, ok := m.(*LogMailer)
	if !ok {
		t.Fatalf("New without SMTP host = %T, want *LogMailer", m)
	}
	if lm.From != "no-reply@x.test" {
		t.Fatalf("LogMailer.From = %q", lm.From)
	}
}

func TestNewSMTPMailerWhenHostSet(t *testing.T) {
	m := New(&config.Config{EmailServerHost: "smtp.x.test", EmailServerPort: 587, FromEmail: "f@x.test"})
	if _, ok := m.(*SMTPMailer); !ok {
		t.Fatalf("New with SMTP host = %T, want *SMTPMailer", m)
	}
}

func TestLogMailerSend(t *testing.T) {
	if err := (&LogMailer{From: "f@x.test"}).Send(context.Background(), "to@x.test", "subj", "body"); err != nil {
		t.Fatalf("LogMailer.Send: %v", err)
	}
}

func TestBuildMessage(t *testing.T) {
	msg := buildMessage("from@x.test", "to@x.test", "Hello", "the body")
	for _, want := range []string{
		"From: from@x.test\r\n", "To: to@x.test\r\n", "Subject: Hello\r\n",
		"MIME-Version: 1.0\r\n", "Content-Type: text/plain; charset=UTF-8\r\n",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("message missing %q; got:\n%s", want, msg)
		}
	}
	if !strings.HasSuffix(msg, "\r\n\r\nthe body") {
		t.Errorf("body should follow a blank line; got:\n%s", msg)
	}
}

func TestSMTPMailerDialError(t *testing.T) {
	// Bind then immediately release a port so the address is guaranteed refused.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	m := &SMTPMailer{cfg: &config.Config{
		EmailServerHost: "127.0.0.1", EmailServerPort: port, FromEmail: "f@x.test",
	}}
	err = m.Send(context.Background(), "to@x.test", "s", "b")
	if err == nil || !strings.Contains(err.Error(), "mail: dial") {
		t.Fatalf("Send to a closed port err = %v, want a dial error", err)
	}
}

func TestSMTPMailerSendHappyPath(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	go serveFakeSMTP(ln)

	port := ln.Addr().(*net.TCPAddr).Port
	m := &SMTPMailer{cfg: &config.Config{
		// A loopback server name lets net/smtp send PLAIN auth over plaintext.
		EmailServerHost: "127.0.0.1", EmailServerPort: port,
		EmailServerUser: "user", EmailServerPassword: "pass",
		FromEmail: "from@x.test", EmailTLSRejectUnauthorized: true,
	}}
	if err := m.Send(context.Background(), "to@x.test", "Subject line", "Message body"); err != nil {
		t.Fatalf("SMTP Send happy path: %v", err)
	}
}

// serveFakeSMTP accepts one connection and speaks just enough ESMTP for
// net/smtp's client to greet, authenticate (PLAIN), and deliver a message over
// plaintext. It advertises AUTH but not STARTTLS, matching the plaintext path.
func serveFakeSMTP(ln net.Listener) {
	conn, err := ln.Accept()
	if err != nil {
		return // listener closed at test end
	}
	defer func() { _ = conn.Close() }()
	r := bufio.NewReader(conn)
	w := func(s string) { _, _ = conn.Write([]byte(s)) }

	w("220 mock ESMTP ready\r\n")
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		switch {
		case strings.HasPrefix(line, "EHLO"), strings.HasPrefix(line, "HELO"):
			w("250-mock greets you\r\n250 AUTH PLAIN\r\n")
		case strings.HasPrefix(line, "AUTH"):
			w("235 2.7.0 authenticated\r\n")
		case strings.HasPrefix(line, "MAIL"):
			w("250 2.1.0 ok\r\n")
		case strings.HasPrefix(line, "RCPT"):
			w("250 2.1.5 ok\r\n")
		case strings.HasPrefix(line, "DATA"):
			w("354 end data with <CR><LF>.<CR><LF>\r\n")
			for {
				dl, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if dl == ".\r\n" || dl == ".\n" {
					break
				}
			}
			w("250 2.0.0 queued\r\n")
		case strings.HasPrefix(line, "QUIT"):
			w("221 2.0.0 bye\r\n")
			return
		default:
			w("250 ok\r\n")
		}
	}
}
