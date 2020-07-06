package local

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/textileio/textile/buckets"
	"github.com/textileio/textile/cmd"
	"github.com/textileio/textile/util"
)

var (
	AddFileTimeout = time.Hour * 24
	GetFileTimeout = time.Hour * 24

	ErrUpToDate = fmt.Errorf("everything up-to-date")
	ErrAborted  = fmt.Errorf("operation aborted by caller")
)

type PathEvent struct {
	Path     string
	Cid      cid.Cid
	Type     PathEventType
	Size     int64
	Progress int64
}

type PathEventType int

const (
	PathStart PathEventType = iota
	PathComplete
	FileStart
	FileProgress
	FileComplete
	FileRemoved
)

type Bucket struct {
	cwd     string
	conf    cmd.Config
	clients *cmd.Clients
	repo    *Repo
}

func (b *Bucket) Cwd() string {
	return b.cwd
}

type Roots struct {
	Local  cid.Cid `json:"local"`
	Remote cid.Cid `json:"remote"`
}

func (b *Bucket) Roots() (roots Roots, err error) {
	lc, rc, err := b.repo.Root()
	if err != nil {
		return
	}
	if !rc.Defined() {
		rc, err = b.getRemoteRoot()
		if err != nil {
			return
		}
	}
	return Roots{Local: lc, Remote: rc}, nil
}

type Links struct {
	URL  string `json:"url"`
	WWW  string `json:"www"`
	IPNS string `json:"ipns"`
}

func (b *Bucket) RemoteLinks() (links Links, err error) {
	ctx, cancel := b.clients.Ctx.Thread(cmd.Timeout)
	defer cancel()
	key := b.conf.Viper.GetString("key")
	res, err := b.clients.Buckets.Links(ctx, key)
	if err != nil {
		return
	}
	return Links{URL: res.URL, WWW: res.WWW, IPNS: res.IPNS}, nil
}

func (b *Bucket) CatRemotePath(pth string, w io.Writer) error {
	ctx, cancel := b.clients.Ctx.Thread(GetFileTimeout)
	defer cancel()
	key := b.conf.Viper.GetString("key")
	return b.clients.Buckets.PullPath(ctx, key, pth, w)
}

func (b *Bucket) Destroy() error {
	cr, err := b.confRoot()
	if err != nil {
		return err
	}
	ctx, cancel := b.clients.Ctx.Thread(cmd.Timeout)
	defer cancel()
	key := b.conf.Viper.GetString("key")
	if err := b.clients.Buckets.Remove(ctx, key); err != nil {
		cmd.Fatal(err)
	}
	_ = os.RemoveAll(filepath.Join(cr, buckets.SeedName))
	_ = os.RemoveAll(filepath.Join(cr, b.conf.Dir))
	return nil
}

func (b *Bucket) loadLocalRepo(setCidVersion bool) error {
	r, err := NewRepo(b.cwd, options.BalancedLayout)
	if err != nil {
		return err
	}
	if setCidVersion {
		if err = b.setRepoCidVersion(r); err != nil {
			return err
		}
	}
	b.repo = r
	return nil
}

func (b *Bucket) setRepoCidVersion(repo *Repo) error {
	r, err := b.Roots()
	if err != nil {
		return err
	}
	if !r.Remote.Defined() {
		r.Remote, err = b.getRemoteRoot()
		if err != nil {
			return err
		}
		repo.SetCidVersion(int(r.Remote.Version()))
	}
	return nil
}

func (b *Bucket) confRoot() (string, error) {
	conf := b.conf.Viper.ConfigFileUsed()
	if conf == "" {
		return "", ErrNotABucket
	}
	return filepath.Dir(filepath.Dir(conf)), nil
}

func (b *Bucket) containsPath(pth string) (c bool, err error) {
	r, err := b.confRoot()
	if err != nil {
		return
	}
	ar, err := filepath.Abs(r)
	if err != nil {
		return
	}
	ap, err := filepath.Abs(pth)
	if err != nil {
		return
	}
	return strings.HasPrefix(ap, ar), nil
}

func (b *Bucket) getRemoteRoot() (cid.Cid, error) {
	ctx, cancel := b.clients.Ctx.Thread(cmd.Timeout)
	defer cancel()
	key := b.conf.Viper.GetString("key")
	rr, err := b.clients.Buckets.Root(ctx, key)
	if err != nil {
		return cid.Undef, err
	}
	rp, err := util.NewResolvedPath(rr.Root.Path)
	if err != nil {
		return cid.Undef, err
	}
	return rp.Cid(), nil
}
