---
layout: default
title: SMTP
parent: Notifier
grand_parent: Configuration
nav_order: 2
---

# SMTP
**Authelia** can send emails to users through an SMTP server.
It can be configured as described below.

```yaml
# Configuration of the notification system.
#
# Notifications are sent to users when they require a password reset, a u2f
# registration or a TOTP registration.
# Use only an available configuration: filesystem, smtp.
notifier:
  # You can disable the notifier startup check by setting this to true.
  disable_startup_check: false

  # For testing purpose, notifications can be sent in a file
  ## filesystem:
  ##   filename: /config/notification.txt

  # Use a SMTP server for sending notifications. Authelia uses PLAIN or LOGIN method to authenticate.
  # [Security] By default Authelia will:
  #   - force all SMTP connections over TLS including unauthenticated connections
  #      - use the disable_require_tls boolean value to disable this requirement (only works for unauthenticated connections)
  #   - validate the SMTP server x509 certificate during the TLS handshake against the hosts trusted certificates (configure in tls section)
  smtp:
    username: test
    # Password can also be set using a secret: https://docs.authelia.com/configuration/secrets.html
    password: password
    host: 127.0.0.1
    port: 1025
    sender: admin@example.com
    # HELO/EHLO Identifier. Some SMTP Servers may reject the default of localhost.
    identifier: localhost
    # Subject configuration of the emails sent.
    # {title} is replaced by the text from the notifier
    subject: "[Authelia] {title}"
    # This address is used during the startup check to verify the email configuration is correct. It's not important what it is except if your email server only allows local delivery.
    startup_check_address: test@authelia.com
    disable_require_tls: false
    disable_html_emails: false

    tls:
      # Server Name for certificate validation (in case you are using the IP or non-FQDN in the host option).
      # server_name: smtp.example.com

      # Skip verifying the server certificate (to allow a self-signed certificate).
      skip_verify: false

      # Minimum TLS version for either StartTLS or SMTPS.
      minimum_version: TLS1.2

  # Sending an email using a Gmail account is as simple as the next section.
  # You need to create an app password by following: https://support.google.com/accounts/answer/185833?hl=en
  ## smtp:
  ##   username: myaccount@gmail.com
  ##   # Password can also be set using a secret: https://docs.authelia.com/configuration/secrets.html
  ##   password: yourapppassword
  ##   sender: admin@example.com
  ##   host: smtp.gmail.com
  ##   port: 587
```

## Configuration options
Most configuration options are self-explanatory, however here is an explanation of the ones that may not
be as obvious.

### host
If utilising an IPv6 literal address it must be enclosed by square brackets and quoted:

```yaml
host: "[fd00:1111:2222:3333::1]"
```

### identifier
The name to send to the SMTP server as the identifier with the HELO/EHLO command. Some SMTP providers like Google Mail
reject the message if it's localhost.

### subject
This is the subject Authelia will use in the email, it has a single placeholder at present `{title}` which should
be included in all emails as it is the internal descriptor for the contents of the email.

### disable_require_tls
For security reasons the default settings for Authelia require the SMTP connection is encrypted by TLS. See [security] for
more information. This option disables this measure (not recommended).

### disable_html_emails
This option forces Authelia to only send plain text email via the notifier. This is the default for the file based 
notifier, but some users may wish to use plain text for security reasons.

### TLS (section)
The key `tls` is a map of options for tuning TLS options. You can see how to configure the tls section [here](../index.md#tls-configuration).

## Using Gmail
You need to generate an app password in order to use Gmail SMTP servers. The process is
described [here](https://support.google.com/accounts/answer/185833?hl=en)

```yaml
notifier:
  smtp:
    username: myaccount@gmail.com
    # Password can also be set using a secret: https://docs.authelia.com/configuration/secrets.html
    password: yourapppassword
    sender: admin@example.com
    host: smtp.gmail.com
    port: 587
```

## Loading a password from a secret instead of inside the configuration
Password can also be defined using a [secret](../secrets.md).

[security]: ../../security/measures.md#notifier-security-measures-smtp