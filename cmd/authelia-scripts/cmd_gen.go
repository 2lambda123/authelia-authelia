package main

import (
	"fmt"
	"net/mail"
	"net/url"
	"os"
	"reflect"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/spf13/cobra"

	"github.com/authelia/authelia/v4/internal/configuration/schema"
)

// NewRunGenCmd implements the code generation cobra command.
func NewRunGenCmd() (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use:  "gen",
		RunE: runGenE,
	}

	return cmd
}

func runGenE(cmd *cobra.Command, args []string) (err error) {
	if err = genConfigurationKeys(); err != nil {
		return err
	}

	return nil
}

func genConfigurationKeys() (err error) {
	data := loadKeysTemplate()

	f, err := os.Create("./internal/configuration/schema/keys.go")
	if err != nil {
		return err
	}

	return keysTemplate.Execute(f, data)
}

var keysTemplate = template.Must(template.New("keys").Parse(`// Code generated by go generate. DO NOT EDIT.
//
// Run the following command to generate this file:
// 		go run ./cmd/authelia-scripts gen
//

package schema

// Keys represents the detected schema keys.
var Keys = []string{
{{- range .Keys }}
	{{ printf "%q" . }},
{{- end }}
}
`))

type keysTemplateStruct struct {
	Timestamp time.Time
	Keys      []string
}

func loadKeysTemplate() keysTemplateStruct {
	config := schema.Configuration{
		Storage: schema.StorageConfiguration{
			Local:      &schema.LocalStorageConfiguration{},
			MySQL:      &schema.MySQLStorageConfiguration{},
			PostgreSQL: &schema.PostgreSQLStorageConfiguration{},
		},
		Notifier: schema.NotifierConfiguration{
			FileSystem: &schema.FileSystemNotifierConfiguration{},
			SMTP: &schema.SMTPNotifierConfiguration{
				TLS: &schema.TLSConfig{},
			},
		},
		AuthenticationBackend: schema.AuthenticationBackendConfiguration{
			File: &schema.FileAuthenticationBackendConfiguration{
				Password: &schema.PasswordConfiguration{},
			},
			LDAP: &schema.LDAPAuthenticationBackendConfiguration{
				TLS: &schema.TLSConfig{},
			},
		},
		Session: schema.SessionConfiguration{
			Redis: &schema.RedisSessionConfiguration{
				TLS:              &schema.TLSConfig{},
				HighAvailability: &schema.RedisHighAvailabilityConfiguration{},
			},
		},
		IdentityProviders: schema.IdentityProvidersConfiguration{
			OIDC: &schema.OpenIDConnectConfiguration{},
		},
	}

	return keysTemplateStruct{
		Timestamp: time.Now(),
		Keys:      readTags("", reflect.TypeOf(config)),
	}
}

var decodedTypes = []reflect.Type{
	reflect.TypeOf(mail.Address{}),
	reflect.TypeOf(regexp.Regexp{}),
	reflect.TypeOf(url.URL{}),
	reflect.TypeOf(time.Duration(0)),
}

func containsType(needle reflect.Type, haystack []reflect.Type) (contains bool) {
	for _, t := range haystack {
		if needle.Kind() == reflect.Ptr {
			if needle.Elem() == t {
				return true
			}
		} else if needle == t {
			return true
		}
	}

	return false
}

func readTags(prefix string, t reflect.Type) (tags []string) {
	tags = make([]string, 0)

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		tag := field.Tag.Get("koanf")

		if tag == "" {
			tags = append(tags, prefix)

			continue
		}

		switch field.Type.Kind() {
		case reflect.Struct:
			if !containsType(field.Type, decodedTypes) {
				tags = append(tags, readTags(getKeyNameFromTagAndPrefix(prefix, tag, false), field.Type)...)

				continue
			}
		case reflect.Slice:
			if field.Type.Elem().Kind() == reflect.Struct {
				if !containsType(field.Type.Elem(), decodedTypes) {
					tags = append(tags, getKeyNameFromTagAndPrefix(prefix, tag, false))
					tags = append(tags, readTags(getKeyNameFromTagAndPrefix(prefix, tag, true), field.Type.Elem())...)

					continue
				}
			}
		case reflect.Ptr:
			switch field.Type.Elem().Kind() {
			case reflect.Struct:
				if !containsType(field.Type.Elem(), decodedTypes) {
					tags = append(tags, readTags(getKeyNameFromTagAndPrefix(prefix, tag, false), field.Type.Elem())...)

					continue
				}
			case reflect.Slice:
				if field.Type.Elem().Elem().Kind() == reflect.Struct {
					if !containsType(field.Type.Elem(), decodedTypes) {
						tags = append(tags, readTags(getKeyNameFromTagAndPrefix(prefix, tag, true), field.Type.Elem())...)

						continue
					}
				}
			}
		}

		tags = append(tags, getKeyNameFromTagAndPrefix(prefix, tag, false))
	}

	return tags
}

func getKeyNameFromTagAndPrefix(prefix, name string, slice bool) string {
	nameParts := strings.SplitN(name, ",", 2)

	if prefix == "" {
		return nameParts[0]
	}

	if len(nameParts) == 2 && nameParts[1] == "squash" {
		return prefix
	}

	if slice {
		return fmt.Sprintf("%s.%s[]", prefix, nameParts[0])
	}

	return fmt.Sprintf("%s.%s", prefix, nameParts[0])
}
