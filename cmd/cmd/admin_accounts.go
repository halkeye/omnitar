package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// adminAccountsCmd represents the admin command
var adminAccountsCmd = &cobra.Command{
	Use:   "accounts",
	Short: "accounts",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("adminAccounts called")
	},
}

func init() {
	adminCmd.AddCommand(adminAccountsCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// adminAccountsCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// adminAccountsCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
