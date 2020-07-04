package cli

import (
	"errors"

	"github.com/logrusorgru/aurora"
	"github.com/spf13/cobra"
	"github.com/textileio/textile/buckets"
	"github.com/textileio/textile/buckets/local"
	"github.com/textileio/textile/cmd"
)

const nonFastForwardMsg = "the root of your bucket is behind (try `%s` before pushing again)"

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push bucket object changes",
	Long:  `Pushes paths that have been added to and paths that have been removed or differ from the local bucket root.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		buck, err := bucks.GetLocalBucket()
		cmd.ErrCheck(err)
		force, err := c.Flags().GetBool("force")
		cmd.ErrCheck(err)
		yes, err := c.Flags().GetBool("yes")
		cmd.ErrCheck(err)
		events := make(chan local.PathEvent)
		go handleProgressBars(events)
		roots, err := buck.PushLocalPath(
			local.WithConfirm(getConfirm("Push %d changes", yes)),
			local.WithForce(force),
			local.WithPathEvents(events))
		if errors.Is(err, local.ErrAborted) {
			cmd.End("")
		} else if errors.Is(err, local.ErrUpToDate) {
			cmd.End("Everything up-to-date")
		} else if errors.Is(err, buckets.ErrNonFastForward) {
			cmd.Fatal(errors.New(nonFastForwardMsg), aurora.Cyan("buck pull"))
		} else if err != nil {
			cmd.Fatal(err)
		}
		cmd.Message("%s", aurora.White(roots.Remote).Bold())
	},
}
