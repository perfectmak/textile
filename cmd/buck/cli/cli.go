package cli

import (
	"os"
	"strconv"

	"github.com/logrusorgru/aurora"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/textileio/textile/buckets/local"
	"github.com/textileio/textile/cmd"
	"github.com/textileio/uiprogress"
)

const Name = "buck"

var (
	config = cmd.Config{
		Viper: viper.New(),
		Dir:   ".textile",
		Name:  "config",
		Flags: map[string]cmd.Flag{
			"key": {
				Key:      "key",
				DefValue: "",
			},
			"org": {
				Key:      "org",
				DefValue: "",
			},
			"thread": {
				Key:      "thread",
				DefValue: "",
			},
		},
		EnvPre: "BUCK",
		Global: false,
	}

	bucks *local.Buckets
)

func init() {
	uiprogress.Empty = ' '
	uiprogress.Fill = '-'
}

func Init(baseCmd *cobra.Command) {
	baseCmd.AddCommand(initCmd, linksCmd, rootCmd, statusCmd, lsCmd, pushCmd, pullCmd, addCmd, catCmd, destroyCmd, encryptCmd, decryptCmd, archiveCmd)
	archiveCmd.AddCommand(archiveStatusCmd, archiveInfoCmd)

	initCmd.PersistentFlags().String("key", "", "Bucket key")
	initCmd.PersistentFlags().String("org", "", "Org username")
	initCmd.PersistentFlags().String("thread", "", "Thread ID")
	if err := cmd.BindFlags(config.Viper, initCmd, config.Flags); err != nil {
		cmd.Fatal(err)
	}
	initCmd.Flags().StringP("name", "n", "", "Bucket name")
	initCmd.Flags().BoolP("private", "p", false, "Obfuscates files and folders with encryption")
	initCmd.Flags().String("cid", "", "Bootstrap the bucket with a UnixFS Cid from the IPFS network")
	initCmd.Flags().BoolP("existing", "e", false, "Initializes from an existing remote bucket if true")

	pushCmd.Flags().BoolP("force", "f", false, "Allows non-fast-forward updates if true")
	pushCmd.Flags().BoolP("yes", "y", false, "Skips the confirmation prompt if true")

	pullCmd.Flags().BoolP("force", "f", false, "Force pull all remote files if true")
	pullCmd.Flags().Bool("hard", false, "Pulls and prunes local changes if true")
	pullCmd.Flags().BoolP("yes", "y", false, "Skips the confirmation prompt if true")

	addCmd.Flags().BoolP("yes", "y", false, "Skips confirmations prompts to always overwrite files and merge folders")

	encryptCmd.Flags().StringP("password", "p", "", "Encryption password")
	decryptCmd.Flags().StringP("password", "p", "", "Decryption password")

	archiveStatusCmd.Flags().BoolP("watch", "w", false, "Watch execution log")
}

func Config() cmd.Config {
	return config
}

func PreRun(c *cmd.Clients) {
	bucks = local.NewBuckets(config, c)
	cmd.ExpandConfigVars(config.Viper, config.Flags)
}

var statusCmd = &cobra.Command{
	Use: "status",
	Aliases: []string{
		"st",
	},
	Short: "Show bucket object changes",
	Long:  `Displays paths that have been added to and paths that have been removed or differ from the local bucket root.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		buck, err := bucks.GetLocalBucket()
		if err != nil {
			cmd.Fatal(err)
		}
		diff, err := buck.Diff()
		if err != nil {
			cmd.Fatal(err)
		}
		if len(diff) == 0 {
			cmd.End("Everything up-to-date")
		}
		for _, c := range diff {
			cf := local.ChangeColor(c.Type)
			cmd.Message("%s  %s", cf(local.ChangeType(c.Type)), cf(c.Rel))
		}
	},
}

var rootCmd = &cobra.Command{
	Use:   "root",
	Short: "Show bucket root CIDs",
	Long:  `Shows the local and remote bucket root CIDs (these will differ if the bucket is encrypted).`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		buck, err := bucks.GetLocalBucket()
		if err != nil {
			cmd.Fatal(err)
		}
		r, err := buck.Roots()
		if err != nil {
			cmd.Fatal(err)
		}
		cmd.Message("%s (local)", aurora.White(r.Local).Bold())
		cmd.Message("%s (remote)", aurora.White(r.Remote).Bold())
	},
}

var linksCmd = &cobra.Command{
	Use: "links",
	Aliases: []string{
		"link",
	},
	Short: "Show links to where this bucket can be accessed",
	Long:  `Displays a thread, IPNS, and website link to this bucket.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		buck, err := bucks.GetLocalBucket()
		if err != nil {
			cmd.Fatal(err)
		}
		links, err := buck.RemoteLinks()
		if err != nil {
			cmd.Fatal(err)
		}
		printLinks(links)
	},
}

func printLinks(reply local.Links) {
	cmd.Message("Your bucket links:")
	cmd.Message("%s Thread link", aurora.White(reply.URL).Bold())
	cmd.Message("%s IPNS link (propagation can be slow)", aurora.White(reply.IPNS).Bold())
	if reply.WWW != "" {
		cmd.Message("%s Bucket website", aurora.White(reply.WWW).Bold())
	}
}

var lsCmd = &cobra.Command{
	Use: "ls [path]",
	Aliases: []string{
		"list",
	},
	Short: "List top-level or nested bucket objects",
	Long:  `Lists top-level or nested bucket objects.`,
	Args:  cobra.MaximumNArgs(1),
	Run: func(c *cobra.Command, args []string) {
		buck, err := bucks.GetLocalBucket()
		if err != nil {
			cmd.Fatal(err)
		}
		var pth string
		if len(args) > 0 {
			pth = args[0]
		}
		items, err := buck.ListRemotePath(pth)
		if err != nil {
			cmd.Fatal(err)
		}
		var data [][]string
		if len(items) > 0 {
			for _, item := range items {
				var links string
				if item.IsDir {
					links = strconv.Itoa(len(item.Items))
				} else {
					links = "n/a"
				}
				data = append(data, []string{
					item.Name,
					strconv.Itoa(int(item.Size)),
					strconv.FormatBool(item.IsDir),
					links,
					item.Cid.String(),
				})
			}
		}
		if len(data) > 0 {
			cmd.RenderTable([]string{"name", "size", "dir", "objects", "cid"}, data)
		}
		cmd.Message("Found %d objects", aurora.White(len(data)).Bold())
	},
}

var catCmd = &cobra.Command{
	Use:   "cat [path]",
	Short: "Cat bucket objects at path",
	Long:  `Cats bucket objects at path.`,
	Args:  cobra.ExactArgs(1),
	Run: func(c *cobra.Command, args []string) {
		buck, err := bucks.GetLocalBucket()
		if err != nil {
			cmd.Fatal(err)
		}
		if err := buck.CatRemotePath(args[0], os.Stdout); err != nil {
			cmd.Fatal(err)
		}
	},
}

var encryptCmd = &cobra.Command{
	Use:   "encrypt [file] [password]",
	Short: "Encrypt file with a password",
	Long:  `Encrypts file with a password (WARNING: Password is not recoverable).`,
	Args:  cobra.ExactArgs(2),
	Run: func(c *cobra.Command, args []string) {
		buck, err := bucks.GetLocalBucket()
		if err != nil {
			cmd.Fatal(err)
		}
		if err := buck.EncryptLocalPath(args[0], args[1], os.Stdout); err != nil {
			cmd.Fatal(err)
		}
	},
}

var decryptCmd = &cobra.Command{
	Use:   "decrypt [path] [password]",
	Short: "Decrypt bucket objects at path with password",
	Long:  `Decrypts bucket objects at path with the given password and writes to stdout.`,
	Args:  cobra.ExactArgs(2),
	Run: func(c *cobra.Command, args []string) {
		buck, err := bucks.GetLocalBucket()
		if err != nil {
			cmd.Fatal(err)
		}
		if err := buck.DecryptLocalPath(args[0], args[1], os.Stdout); err != nil {
			cmd.Fatal(err)
		}
	},
}

var destroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroy bucket and all objects",
	Long:  `Destroys the bucket and all objects.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		buck, err := bucks.GetLocalBucket()
		if err != nil {
			cmd.Fatal(err)
		}
		cmd.Warn("%s", aurora.Red("This action cannot be undone. The bucket and all associated data will be permanently deleted."))
		prompt := promptui.Prompt{
			Label:     "Are you absolutely sure",
			IsConfirm: true,
		}
		if _, err := prompt.Run(); err != nil {
			cmd.End("")
		}
		if err := buck.Destroy(); err != nil {
			cmd.Fatal(err)
		}
		cmd.Success("Your bucket has been deleted")
	},
}
