package cmd

import (
	"github.com/spf13/cobra"

	"github.com/halkeye/omnitar/internal/server"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Runs the http server",
	RunE: func(cmd *cobra.Command, _ []string) error {
		return server.Run(cmd.Context())
	},
}

func init() {
	rootCmd.AddCommand(serverCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// serverCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// serverCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
