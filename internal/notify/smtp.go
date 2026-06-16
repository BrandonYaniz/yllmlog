package notify

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/smtp"
	"strings"
)

type SMTPConfig struct {
	Host            string
	Port            int
	Username        string
	Password        string
	From            string
	To              string
	RequireSTARTTLS bool
}

type Message struct {
	Subject string
	Body    string
}

type SMTPSender struct {
	config SMTPConfig
	dial   smtpDialer
}

type smtpDialer func(address string) (smtpClient, error)

type smtpClient interface {
	Hello(localName string) error
	Extension(ext string) (bool, string)
	StartTLS(config *tls.Config) error
	Auth(auth smtp.Auth) error
	Mail(from string) error
	Rcpt(to string) error
	Data() (io.WriteCloser, error)
	Quit() error
	Close() error
}

func NewSMTPSender(config SMTPConfig) (SMTPSender, error) {
	if strings.TrimSpace(config.Host) == "" {
		return SMTPSender{}, errors.New("smtp host is required")
	}
	if config.Port <= 0 {
		return SMTPSender{}, errors.New("smtp port is required")
	}
	if strings.TrimSpace(config.From) == "" {
		return SMTPSender{}, errors.New("smtp from address is required")
	}
	if strings.TrimSpace(config.To) == "" {
		return SMTPSender{}, errors.New("smtp to address is required")
	}
	return SMTPSender{
		config: config,
		dial: func(address string) (smtpClient, error) {
			return smtp.Dial(address)
		},
	}, nil
}

func (s SMTPSender) Send(message Message) error {
	if strings.TrimSpace(message.Subject) == "" {
		return errors.New("message subject is required")
	}
	if strings.TrimSpace(message.Body) == "" {
		return errors.New("message body is required")
	}

	address := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	client, err := s.dial(address)
	if err != nil {
		return fmt.Errorf("connect smtp: %w", err)
	}
	defer client.Close()

	if err := client.Hello("localhost"); err != nil {
		return fmt.Errorf("smtp hello: %w", err)
	}
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(&tls.Config{ServerName: s.config.Host, MinVersion: tls.VersionTLS12}); err != nil {
			return fmt.Errorf("smtp starttls: %w", err)
		}
	} else if s.config.RequireSTARTTLS {
		return errors.New("smtp server does not advertise STARTTLS")
	}

	if s.config.Username != "" {
		auth := smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.Host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}
	if err := client.Mail(s.config.From); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}
	if err := client.Rcpt(s.config.To); err != nil {
		return fmt.Errorf("smtp rcpt to: %w", err)
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := writer.Write(formatMessage(s.config.From, s.config.To, message)); err != nil {
		writer.Close()
		return fmt.Errorf("write smtp message: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("close smtp message: %w", err)
	}
	if err := client.Quit(); err != nil {
		return fmt.Errorf("smtp quit: %w", err)
	}
	return nil
}

func formatMessage(from, to string, message Message) []byte {
	headers := []string{
		"From: " + sanitizeHeader(from),
		"To: " + sanitizeHeader(to),
		"Subject: " + sanitizeHeader(message.Subject),
		"Content-Type: text/plain; charset=utf-8",
		"",
		normalizeBody(message.Body),
	}
	return []byte(strings.Join(headers, "\r\n"))
}

func sanitizeHeader(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	return value
}

func normalizeBody(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	return strings.ReplaceAll(value, "\n", "\r\n")
}
