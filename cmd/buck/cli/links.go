package cli

import (
	"github.com/spf13/cobra"
	"github.com/textileio/textile/cmd"
)

var bucketLinksCmd = &cobra.Command{
	Use: "links",
	Aliases: []string{
		"link",
	},
	Short: "Show links to where this bucket can be accessed",
	Long:  `Displays a thread, IPNS, and website link to this bucket.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		links, err := buck.Links()
		if err != nil {
			cmd.Fatal(err)
		}
		printLinks(links)
	},
}
