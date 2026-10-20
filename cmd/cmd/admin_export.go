package cmd

import (
	"encoding/json"
	"fmt"
	"uuid"

	"github.com/spf13/cobra"
	"gorm.io/gorm"

	"github.com/halkeye/omnitar/internal/config"
	"github.com/halkeye/omnitar/internal/models"
)

// exportCmd represents the export command
var exportCmd = &cobra.Command{
	Use:   "export account-uuid",
	Short: "export account",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		accountUUID := args[0]
		account, err := gorm.G[*models.Account](cfg.Database()).Preload("Tokens", func(db gorm.PreloadBuilder) error {
			return nil
		}).Where(models.Account{ID: uuid.MustParse(accountUUID)}).First(cmd.Context())
		if err != nil {
			return fmt.Errorf("failed to fetch accounts from database: %w", err)
		}

		if account == nil {
			return fmt.Errorf("account not found: %s", accountUUID)
		}

		err = json.NewEncoder(cmd.OutOrStdout()).Encode(account)
		if err != nil {
			return fmt.Errorf("failed to encode output: %w", err)
		}
		return nil
	},
}

func init() {
	adminAccountsCmd.AddCommand(exportCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// exportCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// exportCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
