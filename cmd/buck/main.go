package main

import (
	"context"
	"time"

	"github.com/spf13/cobra"
	"github.com/textileio/textile/api/common"
	"github.com/textileio/textile/cmd"
	buck "github.com/textileio/textile/cmd/buck/cli"
)

const defaultTarget = "127.0.0.1:3006"

var clients *cmd.Clients

func init() {
	cobra.OnInitialize(cmd.InitConfig(buck.Config()))
	buck.Init(rootCmd)

	rootCmd.PersistentFlags().String("api", defaultTarget, "API target")
	err := buck.Config().Viper.BindPFlag("api", rootCmd.PersistentFlags().Lookup("api"))
	cmd.ErrCheck(err)
	buck.Config().Viper.SetDefault("api", defaultTarget)
}

func main() {
	cmd.ErrCheck(rootCmd.Execute())
}

var rootCmd = &cobra.Command{
	Use:   buck.Name,
	Short: "Bucket Client",
	Long: `The Bucket Client.

Manages files and folders in an object storage bucket.`,
	PersistentPreRun: func(c *cobra.Command, args []string) {
		buck.Config().Viper.SetConfigType("yaml")

		target, err := c.Flags().GetString("api")
		cmd.ErrCheck(err)
		clients = cmd.NewClients(target, false, &ctx{})
		buck.PreRun(clients)
	},
	PersistentPostRun: func(c *cobra.Command, args []string) {
		clients.Close()
	},
	Args: cobra.ExactArgs(0),
}

type ctx struct{}

func (c *ctx) Auth(duration time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), duration)
}

func (c *ctx) Thread(duration time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := c.Auth(duration)
	ctx = common.NewThreadIDContext(ctx, cmd.ThreadIDFromString(buck.Config().Viper.GetString("thread")))
	return ctx, cancel
}
