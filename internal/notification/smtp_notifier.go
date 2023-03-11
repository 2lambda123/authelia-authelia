package notification

import (
	"context"
	"fmt"
	"net/mail"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	gomail "github.com/wneessen/go-mail"

	"github.com/authelia/authelia/v4/internal/configuration/schema"
	"github.com/authelia/authelia/v4/internal/logging"
	"github.com/authelia/authelia/v4/internal/random"
	"github.com/authelia/authelia/v4/internal/templates"
	"github.com/authelia/authelia/v4/internal/trust"
)

// NewSMTPNotifier creates a SMTPNotifier using the notifier configuration.
func NewSMTPNotifier(config *schema.SMTPNotifierConfiguration, trustProvider trust.CertificateProvider) *SMTPNotifier {
	var domain string

	at := strings.LastIndex(config.Sender.Address, "@")

	if at >= 0 {
		domain = config.Sender.Address[at+1:]
	} else {
		domain = "localhost.localdomain"
	}

	return &SMTPNotifier{
		config: config,
		domain: domain,
		random: &random.Cryptographical{},
		trust:  trustProvider,
		log:    logging.Logger(),
	}
}

// SMTPNotifier a notifier to send emails to SMTP servers.
type SMTPNotifier struct {
	config *schema.SMTPNotifierConfiguration

	random random.Provider
	trust  trust.CertificateProvider

	domain string

	log *logrus.Logger
}

func (n *SMTPNotifier) opts() (opts []gomail.Option) {
	opts = []gomail.Option{
		gomail.WithPort(n.config.Port),
		gomail.WithHELO(n.config.Identifier),
		gomail.WithTimeout(n.config.Timeout),
		gomail.WithoutNoop(),
	}

	if n.config.TLS != nil {
		opts = append(opts, gomail.WithTLSConfig(n.trust.GetTLSConfig(n.config.TLS)))
	}

	ssl := n.config.Port == smtpPortSUBMISSIONS

	if ssl {
		opts = append(opts, gomail.WithSSL())
	}

	switch {
	case ssl:
		break
	case n.config.DisableStartTLS:
		opts = append(opts, gomail.WithTLSPolicy(gomail.NoTLS))
	case n.config.DisableRequireTLS:
		opts = append(opts, gomail.WithTLSPolicy(gomail.TLSOpportunistic))
	default:
		opts = append(opts, gomail.WithTLSPolicy(gomail.TLSMandatory))
	}

	return opts
}

// StartupCheck implements model.StartupCheck to perform startup check operations.
func (n *SMTPNotifier) StartupCheck() (err error) {
	var client *gomail.Client

	if client, err = gomail.NewClient(n.config.Host, n.opts()...); err != nil {
		return fmt.Errorf("failed to establish client: %w", err)
	}

	ctx := context.Background()

	if err = client.DialWithContext(ctx); err != nil {
		return fmt.Errorf("failed to dial connection: %w", err)
	}

	if err = client.Close(); err != nil {
		return fmt.Errorf("failed to close connection: %w", err)
	}

	return nil
}

// Send a notification via the SMTPNotifier.
func (n *SMTPNotifier) Send(ctx context.Context, recipient mail.Address, subject string, et *templates.EmailTemplate, data any) (err error) {
	msg := gomail.NewMsg(
		gomail.WithMIMEVersion(gomail.Mime10),
		gomail.WithBoundary(n.random.StringCustom(30, random.CharSetAlphaNumeric)),
	)

	n.setMessageID(msg)

	if err = msg.From(n.config.Sender.String()); err != nil {
		return fmt.Errorf("notifier: smtp: failed to set from address: %w", err)
	}

	if err = msg.AddTo(recipient.String()); err != nil {
		return fmt.Errorf("notifier: smtp: failed to set to address: %w", err)
	}

	msg.Subject(strings.ReplaceAll(n.config.Subject, "{title}", subject))

	switch {
	case n.config.DisableHTMLEmails:
		if err = msg.SetBodyTextTemplate(et.Text, data); err != nil {
			return fmt.Errorf("notifier: smtp: failed to set body: text template errored: %w", err)
		}
	default:
		if err = msg.AddAlternativeHTMLTemplate(et.HTML, data); err != nil {
			return fmt.Errorf("notifier: smtp: failed to set body: html template errored: %w", err)
		}

		if err = msg.AddAlternativeTextTemplate(et.Text, data); err != nil {
			return fmt.Errorf("notifier: smtp: failed to set body: text template errored: %w", err)
		}
	}

	var client *gomail.Client

	if client, err = gomail.NewClient(n.config.Host, n.opts()...); err != nil {
		return fmt.Errorf("notifier: smtp: failed to establish client: %w", err)
	}

	if auth := NewOpportunisticSMTPAuth(n.config); auth != nil {
		client.SetSMTPAuthCustom(auth)
	}

	if err = client.DialWithContext(ctx); err != nil {
		return fmt.Errorf("notifier: smtp: failed to dial connection: %w", err)
	}

	if err = client.Send(msg); err != nil {
		return fmt.Errorf("notifier: smtp: failed to send message: %w", err)
	}

	if err = client.Close(); err != nil {
		return fmt.Errorf("notifier: smtp: failed to close connection: %w", err)
	}

	return nil
}

func (n *SMTPNotifier) setMessageID(msg *gomail.Msg) {
	rm := n.random.Intn(10000)

	msg.SetMessageIDWithValue(fmt.Sprintf("%d.%d%d.%s@%s", os.Getpid()+rm, n.random.Intn(100000000), rm, n.random.StringCustom(17, random.CharSetAlphaNumeric), n.domain))
}
