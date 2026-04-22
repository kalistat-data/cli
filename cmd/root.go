/*
Copyright © 2026 Kalistat
*/
package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"github.com/kalistat-data/cli/internal/api"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "kalistat",
	Short: "A brief description of your application",
	Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// errSilent signals "exit non-zero, but do not print anything else" — the
// command has already produced user-facing output.
var errSilent = errors.New("silent")

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err == nil {
		return
	}

	if !errors.Is(err, errSilent) {
		jsonMode, _ := rootCmd.PersistentFlags().GetBool("json")
		printError(err, jsonMode)
	}
	os.Exit(1)
}

func printError(err error, jsonMode bool) {
	var apiErr *api.APIError
	if errors.As(err, &apiErr) && jsonMode {
		os.Stdout.Write(apiErr.Body)
		if !bytes.HasSuffix(apiErr.Body, []byte("\n")) {
			fmt.Println()
		}
		return
	}
	fmt.Fprintf(os.Stderr, "Error: %s\n", err)
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.kalistat.yaml)")
	rootCmd.PersistentFlags().Bool("json", false, "Output raw JSON response")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
