package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/authelia/authelia/internal/authentication"
	"github.com/authelia/authelia/internal/authorization"
	"github.com/authelia/authelia/internal/commands"
	"github.com/authelia/authelia/internal/configuration"
	"github.com/authelia/authelia/internal/logging"
	"github.com/authelia/authelia/internal/middlewares"
	"github.com/authelia/authelia/internal/notification"
	"github.com/authelia/authelia/internal/regulation"
	"github.com/authelia/authelia/internal/server"
	"github.com/authelia/authelia/internal/session"
	"github.com/authelia/authelia/internal/storage"
	"github.com/authelia/authelia/internal/utils"
)

var configPathFlag string

//nolint:gocyclo // TODO: Consider refactoring/simplifying, time permitting
func startServer() {
	if configPathFlag == "" {
		log.Fatal(errors.New("No config file path provided"))
	}

	config, errs := configuration.Read(configPathFlag)

	if len(errs) > 0 {
		for _, err := range errs {
			logging.Logger().Error(err)
		}
		panic(errors.New("Some errors have been reported"))
	}

	if err := logging.InitializeLogger(config.LogFilePath); err != nil {
		log.Fatalf("Cannot initialize logger: %v", err)
	}

	switch config.LogLevel {
	case "info":
		logging.Logger().Info("Logging severity set to info")
		logging.SetLevel(logrus.InfoLevel)
	case "debug":
		logging.Logger().Info("Logging severity set to debug")
		logging.SetLevel(logrus.DebugLevel)
	case "trace":
		logging.Logger().Info("Logging severity set to trace")
		logging.SetLevel(logrus.TraceLevel)
	}

	if os.Getenv("ENVIRONMENT") == "dev" {
		logging.Logger().Info("===> Authelia is running in development mode. <===")
	}

	var userProvider authentication.UserProvider

	if config.AuthenticationBackend.File != nil {
		userProvider = authentication.NewFileUserProvider(config.AuthenticationBackend.File)
	} else if config.AuthenticationBackend.Ldap != nil {
		userProvider = authentication.NewLDAPUserProvider(*config.AuthenticationBackend.Ldap)
	} else {
		log.Fatalf("Unrecognized authentication backend")
	}

	var storageProvider storage.Provider
	if config.Storage.PostgreSQL != nil {
		storageProvider = storage.NewPostgreSQLProvider(*config.Storage.PostgreSQL)
	} else if config.Storage.MySQL != nil {
		storageProvider = storage.NewMySQLProvider(*config.Storage.MySQL)
	} else if config.Storage.Local != nil {
		storageProvider = storage.NewSQLiteProvider(config.Storage.Local.Path)
	} else {
		log.Fatalf("Unrecognized storage backend")
	}

	var notifier notification.Notifier
	if config.Notifier.SMTP != nil {
		notifier = notification.NewSMTPNotifier(*config.Notifier.SMTP)
	} else if config.Notifier.FileSystem != nil {
		notifier = notification.NewFileNotifier(*config.Notifier.FileSystem)
	} else {
		log.Fatalf("Unrecognized notifier")
	}
	if !config.Notifier.DisableStartupCheck {
		_, err := notifier.StartupCheck()
		if err != nil {
			log.Fatalf("Error during notifier startup check: %s", err)
		}
	}

	clock := utils.RealClock{}
	authorizer := authorization.NewAuthorizer(config.AccessControl)
	sessionProvider := session.NewProvider(config.Session)

	var firstFactorDelay time.Duration
	if config.AuthenticationBackend.File != nil {
		password := utils.RandomString(20, authentication.HashingPossibleSaltCharacters)
		start := time.Now()
		_, _ = authentication.HashPassword(password, "",
			config.AuthenticationBackend.File.Password.Algorithm, config.AuthenticationBackend.File.Password.Iterations,
			config.AuthenticationBackend.File.Password.Memory*1024, config.AuthenticationBackend.File.Password.Parallelism,
			config.AuthenticationBackend.File.Password.KeyLength, config.AuthenticationBackend.File.Password.SaltLength)
		firstFactorDelay = time.Since(start)
	}

	regulator := regulation.NewRegulator(config.Regulation, storageProvider, clock, firstFactorDelay)

	providers := middlewares.Providers{
		Authorizer:      authorizer,
		UserProvider:    userProvider,
		Regulator:       regulator,
		StorageProvider: storageProvider,
		Notifier:        notifier,
		SessionProvider: sessionProvider,
	}
	server.StartServer(*config, providers)
}

func main() {
	rootCmd := &cobra.Command{
		Use: "authelia",
		Run: func(cmd *cobra.Command, args []string) {
			startServer()
		},
	}

	rootCmd.Flags().StringVar(&configPathFlag, "config", "", "Configuration file")

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Show the version of Authelia",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Authelia version %s, build %s", BuildTag, BuildCommit)
		},
	}

	rootCmd.AddCommand(versionCmd, commands.HashPasswordCmd,
		commands.ValidateConfigCmd, commands.CertificatesCmd)

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
