package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"gorm.io/gorm"

	"github.com/halkeye/omnitar/internal/config"
	"github.com/halkeye/omnitar/internal/models"
)

// importCmd represents the import command
var importCmd = &cobra.Command{
	Use:   "import json",
	Short: "import account",
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		var account models.Account
		err = json.NewDecoder(cmd.InOrStdin()).Decode(&account)
		fmt.Printf("Importing account: %s\n", account.ID)
		if err != nil {
			return fmt.Errorf("failed to decode input: %w", err)
		}

		cfg.Database().WithContext(cmd.Context()).Transaction(func(tx *gorm.DB) error {
			return gorm.G[models.Account](tx).Create(cmd.Context(), &account)
		})

		return nil
	},
}

func init() {
	adminAccountsCmd.AddCommand(importCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// importCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// importCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
