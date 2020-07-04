package local

// --Buckets--
//x RemoteBuckets
// NewLocalBucket
//x GetLocalBucket
//
// --Bucket--
//x LocalRepo
//x Roots
//x RemoteLinks
//x CatRemotePath
//x EncryptLocalPath
//x DecryptLocalPath
//x ListRemotePath
//x PushLocalPath
// AddRemoteCid
//x PullRemotePath
// ArchiveRemote

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/textileio/textile/buckets"

	"github.com/ipfs/go-cid"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/textile/api/buckets/client"
	"github.com/textileio/textile/api/common"
	"github.com/textileio/textile/cmd"
	"github.com/textileio/textile/util"
)

var (
	ErrNotABucket     = fmt.Errorf("not a bucket (or any of the parent directories): .textile")
	ErrInvalidThread  = fmt.Errorf("invalid thread ID")
	ErrThreadRequired = fmt.Errorf("thread ID is required")
)

type Buckets struct {
	conf    cmd.Config
	clients *cmd.Clients
}

func NewBuckets(config cmd.Config, clients *cmd.Clients) *Buckets {
	cmd.ExpandConfigVars(config.Viper, config.Flags)
	return &Buckets{conf: config, clients: clients}
}

func (b *Buckets) Clients() *cmd.Clients {
	return b.clients
}

type Config struct {
	Key    string
	Thread thread.ID
}

func (b *Buckets) GetLocalConfig() (conf Config, err error) {
	conf = Config{Key: b.conf.Viper.GetString("key")}
	tids := b.conf.Viper.GetString("thread")
	if tids != "" {
		var err error
		conf.Thread, err = thread.Decode(tids)
		if err != nil {
			return conf, ErrInvalidThread
		}
	}
	if conf.Key != "" && !conf.Thread.Defined() {
		return conf, ErrThreadRequired
	}
	return conf, nil
}

func (b *Buckets) NewBucket(conf Config, opts ...NewOption) (buck *Bucket, links Links, err error) {
	args := &newOptions{}
	for _, opt := range opts {
		opt(args)
	}
	events := args.events
	if events == nil {
		events = make(chan PathEvent)
		go func() {
			for range events {
				// noop
			}
		}()
	}

	// Ensure we're not going to overwrite an existing local config
	cwd, err := os.Getwd()
	if err != nil {
		return
	}
	dir := filepath.Join(cwd, b.conf.Dir)
	if err = os.MkdirAll(dir, os.ModePerm); err != nil {
		return
	}
	config := filepath.Join(dir, b.conf.Name+".yml")
	if _, err := os.Stat(config); err == nil {
		return nil, links, fmt.Errorf("bucket %s is already initialized", cwd)
	}

	// Check config values
	if !conf.Thread.Defined() {
		return nil, links, ErrThreadRequired
	}
	b.conf.Viper.Set("thread", conf.Thread.String())
	b.conf.Viper.Set("key", conf.Key)
	buck = &Bucket{
		cwd:     cwd,
		conf:    b.conf,
		clients: b.clients,
	}

	initRemote := conf.Key == ""
	if initRemote {
		ctx, cancel := b.clients.Ctx.Thread(cmd.Timeout)
		defer cancel()
		rep, err := b.clients.Buckets.Init(ctx,
			client.WithName(args.name),
			client.WithPrivate(args.private),
			client.WithCid(args.fromCid))
		if err != nil {
			return nil, links, err
		}
		buck.conf.Viper.Set("key", rep.Root.Key)

		seed := filepath.Join(cwd, buckets.SeedName)
		file, err := os.Create(seed)
		if err != nil {
			return nil, links, err
		}
		_, err = file.Write(rep.Seed)
		if err != nil {
			file.Close()
			return nil, links, err
		}
		file.Close()

		if err = buck.loadLocalRepo(false); err != nil {
			return nil, links, err
		}
		saveCtx, saveCancel := context.WithTimeout(context.Background(), cmd.Timeout)
		defer saveCancel()
		if err = buck.repo.SaveFile(saveCtx, seed, buckets.SeedName); err != nil {
			return nil, links, err
		}
		sc, err := cid.Decode(rep.SeedCid)
		if err != nil {
			return nil, links, err
		}
		if err = buck.repo.SetRemotePath(buckets.SeedName, sc); err != nil {
			return nil, links, err
		}
		rp, err := util.NewResolvedPath(rep.Root.Path)
		if err != nil {
			return nil, links, err
		}
		if err = buck.repo.SetRemotePath("", rp.Cid()); err != nil {
			return nil, links, err
		}

		links = Links{URL: rep.Links.URL, WWW: rep.Links.WWW, IPNS: rep.Links.IPNS}
	} else {
		if err := buck.loadLocalRepo(true); err != nil {
			return nil, links, err
		}
		r, err := buck.Roots()
		if err != nil {
			return nil, links, err
		}
		if err = buck.repo.SetRemotePath("", r.Remote); err != nil {
			return nil, links, err
		}

		links, err = buck.RemoteLinks()
		if err != nil {
			return nil, links, err
		}
	}

	// Write the local config to disk
	if err = buck.conf.Viper.WriteConfigAs(config); err != nil {
		return
	}

	// Pull remote bucket contents
	if !initRemote || args.fromCid.Defined() {
		if _, err := buck.getPath("", cwd, nil, false, events); err != nil {
			return nil, links, err
		}
		saveCtx, saveCancel := context.WithTimeout(context.Background(), cmd.Timeout)
		defer saveCancel()
		if err = buck.repo.Save(saveCtx); err != nil {
			return nil, links, err
		}
	}
	return buck, links, nil
}

func (b *Buckets) GetLocalBucket() (*Bucket, error) {
	if err := b.requireConfig(); err != nil {
		return nil, err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	buck := &Bucket{
		cwd:     cwd,
		conf:    b.conf,
		clients: b.clients,
	}
	if err = buck.loadLocalRepo(true); err != nil {
		return nil, err
	}
	return buck, nil
}

func (b *Buckets) requireConfig() error {
	if b.conf.Viper.ConfigFileUsed() == "" {
		return ErrNotABucket
	}
	return nil
}

type BucketInfo struct {
	ID   thread.ID
	Name string
	Key  string
}

func (b *Buckets) RemoteBuckets() (list []BucketInfo, err error) {
	threads := b.clients.ListThreads(true)
	ctx, cancel := b.clients.Ctx.Auth(cmd.Timeout)
	defer cancel()
	for _, t := range threads {
		ctx = common.NewThreadIDContext(ctx, t.ID)
		res, err := b.clients.Buckets.List(ctx)
		if err != nil {
			cmd.Fatal(err)
		}
		for _, root := range res.Roots {
			name := "unnamed"
			if root.Name != "" {
				name = root.Name
			}
			list = append(list, BucketInfo{
				ID:   t.ID,
				Name: name,
				Key:  root.Key,
			})
		}
	}
	return list, nil
}
