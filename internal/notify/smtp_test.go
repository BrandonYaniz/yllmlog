package notify

import (
	"bytes"
	"crypto/tls"
	"io"
	"net/smtp"
	"strings"
	"testing"
)

func TestSMTPSenderSend(t *testing.T) {
	fake := &fakeSMTP{startTLS: true}
	sender, err := NewSMTPSender(SMTPConfig{
		Host:            "smtp.example.test",
		Port:            587,
		Username:        "user",
		Password:        "secret",
		From:            "yllmlog@example.test",
		To:              "admin@example.test",
		RequireSTARTTLS: true,
	})
	if err != nil {
		t.Fatalf("NewSMTPSender returned error: %v", err)
	}
	sender.dial = func(address string) (smtpClient, error) {
		if address != "smtp.example.test:587" {
			t.Fatalf("address = %q", address)
		}
		return fake, nil
	}

	if err := sender.Send(Message{Subject: "Daily report", Body: "Everything is fine.\n"}); err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if !fake.didStartTLS {
		t.Fatal("StartTLS was not called")
	}
	if !fake.didAuth {
		t.Fatal("Auth was not called")
	}
	if !strings.Contains(fake.data.String(), "Subject: Daily report") {
		t.Fatalf("message missing subject:\n%s", fake.data.String())
	}
	if !strings.Contains(fake.data.String(), "Everything is fine.") {
		t.Fatalf("message missing body:\n%s", fake.data.String())
	}
}

func TestSMTPSenderRequiresSTARTTLS(t *testing.T) {
	fake := &fakeSMTP{}
	sender, err := NewSMTPSender(SMTPConfig{
		Host:            "smtp.example.test",
		Port:            25,
		From:            "yllmlog@example.test",
		To:              "admin@example.test",
		RequireSTARTTLS: true,
	})
	if err != nil {
		t.Fatalf("NewSMTPSender returned error: %v", err)
	}
	sender.dial = func(string) (smtpClient, error) {
		return fake, nil
	}

	if err := sender.Send(Message{Subject: "Report", Body: "Body"}); err == nil {
		t.Fatal("Send succeeded without STARTTLS")
	}
}

func TestFormatMessageSanitizesHeaders(t *testing.T) {
	message := string(formatMessage("from@example.test", "to@example.test", Message{
		Subject: "Hello\r\nBcc: attacker@example.test",
		Body:    "Line 1\nLine 2",
	}))
	if strings.Contains(message, "\r\nBcc:") || strings.Contains(message, "\nBcc:") {
		t.Fatalf("header injection survived:\n%s", message)
	}
	if !strings.Contains(message, "Line 1\r\nLine 2") {
		t.Fatalf("body line endings not normalized:\n%s", message)
	}
}

type fakeSMTP struct {
	startTLS    bool
	didStartTLS bool
	didAuth     bool
	data        bytes.Buffer
}

func (f *fakeSMTP) Hello(string) error { return nil }

func (f *fakeSMTP) Extension(ext string) (bool, string) {
	if ext == "STARTTLS" && f.startTLS {
		return true, ""
	}
	return false, ""
}

func (f *fakeSMTP) StartTLS(*tls.Config) error {
	f.didStartTLS = true
	return nil
}

func (f *fakeSMTP) Auth(smtp.Auth) error {
	f.didAuth = true
	return nil
}

func (f *fakeSMTP) Mail(string) error { return nil }
func (f *fakeSMTP) Rcpt(string) error { return nil }

func (f *fakeSMTP) Data() (io.WriteCloser, error) {
	return nopWriteCloser{Writer: &f.data}, nil
}

func (f *fakeSMTP) Quit() error  { return nil }
func (f *fakeSMTP) Close() error { return nil }

type nopWriteCloser struct {
	io.Writer
}

func (n nopWriteCloser) Close() error { return nil }
