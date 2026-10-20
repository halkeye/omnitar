package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"gorm.io/gorm"

	"github.com/halkeye/omnitar/internal/config"
	"github.com/halkeye/omnitar/internal/models"
)

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "list accounts",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		accounts, err := gorm.G[models.Account](cfg.Database()).Find(cmd.Context())
		if err != nil {
			return fmt.Errorf("failed to fetch accounts from database: %w", err)
		}
		for _, account := range accounts {
			fmt.Printf("Account: %s\n", account.ID)
		}
		return nil
	},
}

func init() {
	adminAccountsCmd.AddCommand(listCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// listCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// listCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
