package cli

import (
	"errors"

	"github.com/logrusorgru/aurora"
	"github.com/spf13/cobra"
	"github.com/textileio/textile/buckets/local"
	"github.com/textileio/textile/cmd"
)

var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull bucket object changes",
	Long:  `Pulls paths that have been added to and paths that have been removed or differ from the remote bucket root.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		force, err := c.Flags().GetBool("force")
		if err != nil {
			cmd.Fatal(err)
		}
		hard, err := c.Flags().GetBool("hard")
		if err != nil {
			cmd.Fatal(err)
		}
		yes, err := c.Flags().GetBool("yes")
		if err != nil {
			cmd.Fatal(err)
		}
		buck, err := bucks.GetLocalBucket()
		if err != nil {
			cmd.Fatal(err)
		}
		events := make(chan local.PathEvent)
		go handleProgressBars(events)
		roots, err := buck.PullRemotePath(
			local.WithConfirm(getConfirm("Discard %d local changes", yes)),
			local.WithForce(force),
			local.WithHard(hard),
			local.WithEvents(events))
		if errors.Is(err, local.ErrAborted) {
			cmd.End("")
		} else if errors.Is(err, local.ErrUpToDate) {
			cmd.End("Everything up-to-date")
		} else if err != nil {
			cmd.Fatal(err)
		}
		cmd.Message("%s", aurora.White(roots.Remote).Bold())
	},
}
