package local

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/textile/api/common"
	"github.com/textileio/textile/cmd"
)

var (
	ErrNotABucket = fmt.Errorf("not a bucket (or any of the parent directories): .textile")
)

type Buckets struct {
	cwd     string
	conf    cmd.Config
	clients *cmd.Clients
}

func NewBuckets(config cmd.Config, clients *cmd.Clients) (*Buckets, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return &Buckets{
		cwd:     cwd,
		conf:    config,
		clients: clients,
	}, nil
}

func (b *Buckets) Create() error {
	dir := filepath.Join(b.cwd, b.conf.Dir)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return err
	}
	cn, err := b.confName()
	if err != nil {
		return err
	}
	if _, err := os.Stat(cn); err == nil {
		return fmt.Errorf("bucket %s is already initialized", b.cwd)
	}

}

type BucketInfo struct {
	ID   thread.ID
	Name string
	Key  string
}

func (b *Buckets) Init() error {

}

func (b *Buckets) ListBuckets() (list []BucketInfo, err error) {
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

func (b *Buckets) confName() (name string, err error) {
	dir := filepath.Join(b.cwd, b.conf.Dir)
	if err = os.MkdirAll(dir, os.ModePerm); err != nil {
		return
	}
	return filepath.Join(dir, b.conf.Name+".yml"), nil
}

func (b *Buckets) ensureScope() error {
	cmd.ExpandConfigVars(b.conf.Viper, b.conf.Flags)
	if b.conf.Viper.ConfigFileUsed() == "" {
		return ErrNotABucket
	}
	return nil
}

type Links struct {
	URL  string
	WWW  string
	IPNS string
}

func (b *Buckets) Links() (links Links, err error) {
	if err = b.ensureScope(); err != nil {
		return
	}

	ctx, cancel := b.clients.Ctx.Thread(cmd.Timeout)
	defer cancel()
	key := b.conf.Viper.GetString("key")
	res, err := b.clients.Buckets.Links(ctx, key)
	if err != nil {
		return
	}
	return Links{URL: res.URL, WWW: res.WWW, IPNS: res.IPNS}, nil
}
