package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/authelia/authelia/internal/commands"
	"github.com/authelia/authelia/internal/logging"
	"github.com/authelia/authelia/internal/utils"
)

var configPathFlag string

func main() {
	logger := logging.Logger()

	version := utils.Version()

	rootCmd := &cobra.Command{
		Use: "authelia",
		Run: func(cmd *cobra.Command, args []string) {
			startServer()
		},
		Version: version,
		Short:   fmt.Sprintf("authelia %s", version),
		Long:    fmt.Sprintf(fmtAutheliaLong, version),
	}

	rootCmd.Flags().StringVar(&configPathFlag, "config", "", "Configuration file")

	buildCmd := &cobra.Command{
		Use:   "build",
		Short: "Show the build of Authelia",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf(fmtAutheliaBuild, utils.BuildTag, utils.BuildStateTag, utils.BuildBranch, utils.BuildCommit,
				utils.BuildNumber, utils.BuildArch, utils.BuildDate, utils.BuildStateExtra)
		},
	}

	rootCmd.AddCommand(buildCmd, commands.HashPasswordCmd,
		commands.ValidateConfigCmd, commands.CertificatesCmd,
		commands.RSACmd)

	if err := rootCmd.Execute(); err != nil {
		logger.Fatal(err)
	}
}
