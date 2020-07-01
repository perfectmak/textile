package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	cid "github.com/ipfs/go-cid"
	"github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/logrusorgru/aurora"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/db"
	bc "github.com/textileio/textile/api/buckets/client"
	"github.com/textileio/textile/api/common"
	"github.com/textileio/textile/buckets"
	"github.com/textileio/textile/buckets/local"
	"github.com/textileio/textile/cmd"
	"github.com/textileio/textile/util"
)

type bucketInfo struct {
	ID   thread.ID
	Name string
	Key  string
}

var bucketInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new or existing bucket",
	Long: `Initializes a new or existing bucket.

A .textile config directory and a seed file will be created in the current working directory.
Existing configs will not be overwritten.

Use the '--existing' flag to initialize from an existing remote bucket.
Use the '--cid' flag to initialize from an existing UnixFS DAG.
`,
	Args: cobra.ExactArgs(0),
	PreRun: func(c *cobra.Command, args []string) {
		cmd.ExpandConfigVars(config.Viper, config.Flags)
	},
	Run: func(c *cobra.Command, args []string) {
		bucks := local.NewBuckets(config, clients)

		bootCid, err := c.Flags().GetString("cid")
		if err != nil {
			cmd.Fatal(err)
		}
		existing, err := c.Flags().GetBool("existing")
		if err != nil {
			cmd.Fatal(err)
		}
		if bootCid != "" && existing {
			cmd.Fatal(errors.New("only one of --cid and --existing flags can be used at the same time"))
		}

		if existing {
			list, err := bucks.ListBuckets()
			if err != nil {
				cmd.Fatal(err)
			}
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
			if err != nil {
				cmd.Fatal(err)
			}

			selected := list[index]
			config.Viper.Set("thread", selected.ID.String())
			config.Viper.Set("key", selected.Key)
		}

		var dbID thread.ID
		xthread := config.Viper.GetString("thread")
		if xthread != "" {
			var err error
			dbID, err = thread.Decode(xthread)
			if err != nil {
				cmd.Fatal(fmt.Errorf("invalid thread ID"))
			}
		}

		xkey := config.Viper.GetString("key")
		initRemote := true
		if xkey != "" {
			if !dbID.Defined() {
				cmd.Fatal(fmt.Errorf("the --thread flag is required when using --key"))
			}
			initRemote = false
		}

		name, err := c.Flags().GetString("name")
		if err != nil {
			cmd.Fatal(err)
		}
		private, err := c.Flags().GetBool("private")
		if err != nil {
			cmd.Fatal(err)
		}
		if initRemote {
			if !c.Flags().Changed("name") {
				namep := promptui.Prompt{
					Label: "Enter a name for your new bucket (optional)",
				}
				var err error
				name, err = namep.Run()
				if err != nil {
					cmd.End("")
				}
			}
			if !c.Flags().Changed("private") {
				privp := promptui.Prompt{
					Label:     "Encrypt bucket contents",
					IsConfirm: true,
				}
				if _, err = privp.Run(); err == nil {
					private = true
				}
			}
		}

		if !dbID.Defined() {
			selected := clients.SelectThread("Buckets are written to a threadDB. Select or create a new one", aurora.Sprintf(
				aurora.BrightBlack("> Selected threadDB {{ .Label | white | bold }}")), true)
			if selected.Label == "Create new" {
				if selected.Name == "" {
					prompt := promptui.Prompt{
						Label: "Enter a name for your new threadDB (optional)",
					}
					var err error
					selected.Name, err = prompt.Run()
					if err != nil {
						cmd.End("")
					}
				}
				ctx, cancel := clients.Ctx.Auth(cmd.Timeout)
				defer cancel()
				ctx = common.NewThreadNameContext(ctx, selected.Name)
				dbID = thread.NewIDV1(thread.Raw, 32)
				if err := clients.Threads.NewDB(ctx, dbID, db.WithNewManagedName(selected.Name)); err != nil {
					cmd.Fatal(err)
				}
			} else {
				dbID = selected.ID
			}
			config.Viper.Set("thread", dbID.String())
		}

		if initRemote {
			ctx, cancel := clients.Ctx.Thread(cmd.Timeout)
			defer cancel()
			opts := []bc.InitOption{bc.WithName(name), bc.WithPrivate(private)}
			if bootCid != "" {
				bCid, err := cid.Decode(bootCid)
				if err != nil {
					cmd.Fatal(err)
				}
				opts = append(opts, bc.WithCid(bCid))
			}
			rep, err := clients.Buckets.Init(ctx, opts...)
			if err != nil {
				cmd.Fatal(err)
			}
			config.Viper.Set("key", rep.Root.Key)

			seed := filepath.Join(root, buckets.SeedName)
			file, err := os.Create(seed)
			if err != nil {
				cmd.Fatal(err)
			}
			_, err = file.Write(rep.Seed)
			if err != nil {
				file.Close()
				cmd.Fatal(err)
			}
			file.Close()

			buck, err := local.NewBucket(root, options.BalancedLayout)
			if err != nil {
				cmd.Fatal(err)
			}
			actx, acancel := context.WithTimeout(context.Background(), cmd.Timeout)
			defer acancel()
			if err = buck.SaveFile(actx, seed, buckets.SeedName); err != nil {
				cmd.Fatal(err)
			}
			sc, err := cid.Decode(rep.SeedCid)
			if err != nil {
				cmd.Fatal(err)
			}
			if err = buck.SetRemotePath(buckets.SeedName, sc); err != nil {
				cmd.Fatal(err)
			}
			rp, err := util.NewResolvedPath(rep.Root.Path)
			if err != nil {
				cmd.Fatal(err)
			}
			if err = buck.SetRemotePath("", rp.Cid()); err != nil {
				cmd.Fatal(err)
			}

			if bootCid != "" {
				getPath(rep.Root.Key, ".", ".", buck, nil, false)
				if err := buck.Save(ctx); err != nil {
					cmd.Fatal(err)
				}
			}

			printLinks(rep.Links)
		}

		if err := config.Viper.WriteConfigAs(filename); err != nil {
			cmd.Fatal(err)
		}
		if initRemote {
			prefix := "Initialized an empty bucket in %s"
			if bootCid != "" {
				prefix = "Initialized a bootstrapped bucket in %s"
			}
			cmd.Success(prefix, aurora.White(root).Bold())
		} else {
			key := config.Viper.GetString("key")
			count := getPath(key, "", root, nil, nil, false)

			buck, err := local.NewBucket(root, options.BalancedLayout)
			if err != nil {
				cmd.Fatal(err)
			}
			rr := getRemoteRoot(key)
			if err := buck.SetRemotePath("", rr); err != nil {
				cmd.Fatal(err)
			}
			buck.SetCidVersion(int(rr.Version()))
			ctx, cancel := context.WithTimeout(context.Background(), cmd.Timeout)
			defer cancel()
			if err = buck.Save(ctx); err != nil {
				cmd.Fatal(err)
			}
			cmd.Success("Initialized from remote and pulled %d objects to %s", aurora.White(count).Bold(), aurora.White(root).Bold())
		}
	},
}
