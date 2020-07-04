package local

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/textile/api/common"
	"github.com/textileio/textile/cmd"
)

var ErrNotABucket = fmt.Errorf("not a bucket (or any of the parent directories): .textile")

type Buckets struct {
	conf    cmd.Config
	clients *cmd.Clients
}

func NewBuckets(config cmd.Config, clients *cmd.Clients) *Buckets {
	return &Buckets{conf: config, clients: clients}
}

func (b *Buckets) NewLocalBucket(opts ...NewOption) (*Bucket, error) {
	args := &newOptions{}
	for _, opt := range opts {
		opt(args)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(cwd, b.conf.Dir)
	if err = os.MkdirAll(dir, os.ModePerm); err != nil {
		return nil, err
	}
	cn := filepath.Join(dir, b.conf.Name+".yml")
	if _, err := os.Stat(cn); err == nil {
		return nil, fmt.Errorf("bucket %s is already initialized", cwd)
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
