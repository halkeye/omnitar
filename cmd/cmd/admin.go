package cmd

import (
	"github.com/spf13/cobra"
)

// adminCmd represents the admin command
var adminCmd = &cobra.Command{
	Use:   "admin",
	Short: "Admin commands",
}

func init() {
	rootCmd.AddCommand(adminCmd)
}
