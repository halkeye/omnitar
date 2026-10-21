package cmd

import (
	"fmt"
	"uuid"

	"github.com/spf13/cobra"
	"gorm.io/gorm"

	"github.com/halkeye/omnitar/internal/config"
	"github.com/halkeye/omnitar/internal/models"
)

// adminAccountsDeleteCmd represents the adminAccountsDelete command
var adminAccountsDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "delete account-uuid",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		_, err = gorm.G[models.Account](cfg.Database()).Where(models.Account{ID: uuid.MustParse(args[0])}).Delete(cmd.Context())
		if err != nil {
			return fmt.Errorf("failed to delete account from database: %w", err)
		}
		return nil
	},
}

func init() {
	adminAccountsCmd.AddCommand(adminAccountsDeleteCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// adminAccountsDeleteCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// adminAccountsDeleteCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
