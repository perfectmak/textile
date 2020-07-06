package cli

import (
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"github.com/textileio/textile/buckets/local"
	"github.com/textileio/textile/cmd"
)

var archiveCmd = &cobra.Command{
	Use:   "archive",
	Short: "Create a Filecoin archive",
	Long:  `Creates a Filecoin archive from the remote bucket root.`,
	Run: func(c *cobra.Command, args []string) {
		cmd.Warn("Archives are currently saved on an experimental test network. They may be lost at any time.")
		prompt := promptui.Prompt{
			Label:     "Proceed",
			IsConfirm: true,
		}
		if _, err := prompt.Run(); err != nil {
			cmd.End("")
		}
		buck, err := bucks.GetLocalBucket()
		cmd.ErrCheck(err)
		err = buck.ArchiveRemote()
		cmd.ErrCheck(err)
		cmd.Success("Archive queued successfully")
	},
}

var archiveStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status of the latest archive",
	Long:  `Shows the status of the most recent bucket archive.`,
	Run: func(c *cobra.Command, args []string) {
		watch, err := c.Flags().GetBool("watch")
		cmd.ErrCheck(err)
		buck, err := bucks.GetLocalBucket()
		cmd.ErrCheck(err)
		msgs, err := buck.ArchiveStatus(watch)
		cmd.ErrCheck(err)
		for m := range msgs {
			switch m.Type {
			case local.ArchiveMessage:
				cmd.Message(m.Message)
			case local.ArchiveWarning:
				cmd.Warn(m.Message)
			case local.ArchiveError:
				cmd.Fatal(m.Error)
			case local.ArchiveSuccess:
				cmd.Success(m.Message)
			}
		}
	},
}

var archiveInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show info about the current archive",
	Long:  `Shows information about the current archive.`,
	Run: func(c *cobra.Command, args []string) {
		buck, err := bucks.GetLocalBucket()
		cmd.ErrCheck(err)
		info, err := buck.ArchiveInfo()
		cmd.ErrCheck(err)
		cmd.Message("Archive of cid %s has %d deals:\n", info.Archive.Cid, len(info.Archive.Deals))
		var data [][]string
		for _, d := range info.Archive.Deals {
			data = append(data, []string{d.ProposalCid.String(), d.Miner})
		}
		cmd.RenderTable([]string{"proposal cid", "miner"}, data)
	},
}
