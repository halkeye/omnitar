package cmd

import (
	"github.com/spf13/cobra"
)

// adminAccountsCmd represents the admin command
var adminAccountsCmd = &cobra.Command{
	Use: "accounts",
}

func init() {
	adminCmd.AddCommand(adminAccountsCmd)
}
