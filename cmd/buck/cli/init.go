package cli

import (
	"errors"
	"fmt"

	cid "github.com/ipfs/go-cid"
	"github.com/logrusorgru/aurora"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/db"
	"github.com/textileio/textile/api/common"
	"github.com/textileio/textile/buckets/local"
	"github.com/textileio/textile/cmd"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new or existing bucket",
	Long: `Initializes a new or existing bucket.

A .textile config directory and a seed file will be created in the current working directory.
Existing configs will not be overwritten.

Use the '--existing' flag to initialize from an existing remote bucket.
Use the '--cid' flag to initialize from an existing UnixFS DAG.
`,
	Args: cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		conf, err := bucks.GetLocalConfig()
		if errors.Is(err, local.ErrThreadRequired) {
			cmd.Fatal(fmt.Errorf("the --thread flag is required when using --key"))
		}
		cmd.ErrCheck(err)

		var xcid cid.Cid
		xcids, err := c.Flags().GetString("cid")
		cmd.ErrCheck(err)
		if xcids != "" {
			xcid, err = cid.Decode(xcids)
			cmd.ErrCheck(err)
		}

		existing, err := c.Flags().GetBool("existing")
		cmd.ErrCheck(err)
		if existing && xcid.Defined() {
			cmd.Fatal(errors.New("only one of --cid and --existing flags can be used at the same time"))
		}

		var name string
		var private bool
		if !existing {
			if c.Flags().Changed("name") {
				name, err = c.Flags().GetString("name")
				cmd.ErrCheck(err)
			} else {
				namep := promptui.Prompt{
					Label: "Enter a name for your new bucket (optional)",
				}
				name, err = namep.Run()
				if err != nil {
					cmd.End("")
				}
			}
			if c.Flags().Changed("private") {
				private, err = c.Flags().GetBool("private")
				cmd.ErrCheck(err)
			} else {
				privp := promptui.Prompt{
					Label:     "Encrypt bucket contents",
					IsConfirm: true,
				}
				if _, err = privp.Run(); err == nil {
					private = true
				}
			}
		}

		if existing {
			list, err := bucks.RemoteBuckets()
			cmd.ErrCheck(err)
			prompt := promptui.Select{
				Label: "Which exiting bucket do you want to init from?",
				Items: list,
				Templates: &promptui.SelectTemplates{
					Active:   fmt.Sprintf(`{{ "%s" | cyan }} {{ .Name | bold }} {{ .Key | faint | bold }}`, promptui.IconSelect),
					Inactive: `{{ .Name | faint }} {{ .Key | faint | bold }}`,
					Selected: aurora.Sprintf(aurora.BrightBlack("> Selected bucket {{ .Name | white | bold }}")),
				},
			}
			index, _, err := prompt.Run()
			cmd.ErrCheck(err)
			selected := list[index]
			name = selected.Name
			conf.Thread = selected.ID
			conf.Key = selected.Key
		}

		if !conf.Thread.Defined() {
			selected := bucks.Clients().SelectThread(
				"Buckets are written to a threadDB. Select or create a new one",
				aurora.Sprintf(aurora.BrightBlack("> Selected threadDB {{ .Label | white | bold }}")),
				true)
			if selected.Label == "Create new" {
				if selected.Name == "" {
					prompt := promptui.Prompt{
						Label: "Enter a name for your new threadDB (optional)",
					}
					selected.Name, err = prompt.Run()
					if err != nil {
						cmd.End("")
					}
				}
				ctx, cancel := bucks.Clients().Ctx.Auth(cmd.Timeout)
				defer cancel()
				ctx = common.NewThreadNameContext(ctx, selected.Name)
				conf.Thread = thread.NewIDV1(thread.Raw, 32)
				err = bucks.Clients().Threads.NewDB(ctx, conf.Thread, db.WithNewManagedName(selected.Name))
				cmd.ErrCheck(err)
			} else {
				conf.Thread = selected.ID
			}
		}

		events := make(chan local.PathEvent)
		go handleProgressBars(events)
		buck, links, err := bucks.NewBucket(
			conf,
			local.WithName(name),
			local.WithPrivate(private),
			local.WithCid(xcid),
			local.WithExistingPathEvents(events))
		cmd.ErrCheck(err)

		printLinks(links)

		var msg string
		if !existing {
			msg = "Initialized a new empty bucket in %s"
			if xcid.Defined() {
				msg = "Initialized a new bootstrapped bucket in %s"
			}
		} else {
			msg = "Initialized from an existing bucket in %s"
		}
		cmd.Success(msg, aurora.White(buck.Cwd()).Bold())
	},
}
