package local

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	cid "github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-merkledag/dagutils"
	"github.com/textileio/textile/api/buckets/client"
	"github.com/textileio/textile/cmd"
	"golang.org/x/sync/errgroup"
)

func (b *Bucket) PullRemotePath(opts ...PathOption) (roots Roots, err error) {
	args := &pathOptions{}
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

	diff, err := b.Diff()
	if err != nil {
		return
	}
	if args.confirm != nil && args.hard && len(diff) > 0 {
		if ok := args.confirm(diff); !ok {
			return roots, ErrAborted
		}
	}

	// Tmp move local modifications and additions if not pulling hard
	if !args.hard {
		for _, c := range diff {
			switch c.Type {
			case dagutils.Mod, dagutils.Add:
				if err := os.Rename(c.Rel, c.Rel+".buckpatch"); err != nil {
					return roots, err
				}
			}
		}
	}

	cr, err := b.confRoot()
	if err != nil {
		return
	}
	count, err := b.getPath("", cr, diff, args.force, events)
	if err != nil {
		return
	}
	if count == 0 {
		return roots, ErrUpToDate
	}

	ctx, cancel := context.WithTimeout(context.Background(), cmd.Timeout)
	defer cancel()
	if err = b.repo.Save(ctx); err != nil {
		return
	}
	rc, err := b.getRemoteRoot()
	if err != nil {
		return
	}
	if err = b.repo.SetRemotePath("", rc); err != nil {
		return
	}

	// Re-apply local changes if not pulling hard
	if !args.hard {
		for _, c := range diff {
			switch c.Type {
			case dagutils.Mod, dagutils.Add:
				if err := os.Rename(c.Rel+".buckpatch", c.Rel); err != nil {
					return roots, err
				}
			case dagutils.Remove:
				// If the file was also deleted on the remote,
				// the local deletion will already have been handled by getPath.
				// So, we just ignore the error here.
				_ = os.Remove(c.Rel)
			}
		}
	}
	return b.Roots()
}

func (b *Bucket) getPath(pth, dest string, diff []Change, force bool, events chan PathEvent) (count int, err error) {
	key := b.conf.Viper.GetString("key")
	all, missing, err := b.listPath(key, pth, dest, force)
	if err != nil {
		return
	}
	count = len(missing)
	var rm []string
	list, err := b.walkPath(dest)
	if err != nil {
		return
	}
loop:
	for _, n := range list {
		for _, r := range all {
			if r.name == n {
				continue loop
			}
		}
		rm = append(rm, n)
	}
looop:
	for _, l := range diff {
		for _, r := range all {
			if r.path == l.Path {
				continue looop
			}
		}
		rm = append(rm, l.Rel)
	}
	count += len(rm)
	if count == 0 {
		return
	}

	if len(missing) > 0 {
		events <- PathEvent{
			Path: pth,
			Type: PathStart,
		}
		eg, gctx := errgroup.WithContext(context.Background())
		for _, o := range missing {
			o := o
			eg.Go(func() error {
				if gctx.Err() != nil {
					return nil
				}
				if err := b.getFile(key, o.path, o.name, o.size, o.cid, events); err != nil {
					return err
				}
				return b.repo.SetRemotePath(o.path, o.cid)
			})
		}
		if err := eg.Wait(); err != nil {
			return count, err
		}
		events <- PathEvent{
			Path: pth,
			Type: PathComplete,
		}
	}
	if len(rm) > 0 {
		for _, r := range rm {
			// The file may have been modified locally, in which case it will have been moved to a patch.
			// So, we just ignore the error here.
			_ = os.Remove(r)
			events <- PathEvent{
				Path: strings.TrimPrefix(r, dest+"/"),
				Type: FileRemoved,
			}
		}
	}
	return count, nil
}

type object struct {
	path string
	name string
	cid  cid.Cid
	size int64
}

func (b *Bucket) listPath(key, pth, dest string, force bool) (all, missing []object, err error) {
	ctx, cancel := b.clients.Ctx.Thread(cmd.Timeout)
	defer cancel()
	rep, err := b.clients.Buckets.ListPath(ctx, key, pth)
	if err != nil {
		return
	}
	if rep.Item.IsDir {
		for _, i := range rep.Item.Items {
			a, m, err := b.listPath(key, filepath.Join(pth, filepath.Base(i.Path)), dest, force)
			if err != nil {
				return nil, nil, err
			}
			all = append(all, a...)
			missing = append(missing, m...)
		}
	} else {
		name := filepath.Join(dest, pth)
		c, err := cid.Decode(rep.Item.Cid)
		if err != nil {
			return nil, nil, err
		}
		o := object{path: pth, name: name, size: rep.Item.Size, cid: c}
		all = append(all, o)
		if !force {
			c, err := cid.Decode(rep.Item.Cid)
			if err != nil {
				return nil, nil, err
			}
			lc, err := b.repo.HashFile(name)
			if err == nil && lc.Equals(c) { // File exists, skip it
				return
			} else {
				match, err := b.repo.MatchPath(pth, lc, c)
				if err != nil {
					if !errors.Is(err, ds.ErrNotFound) {
						return nil, nil, err
					}
				} else if match { // File exists, skip it
					return
				}
			}
		}
		missing = append(missing, o)
	}
	return all, missing, nil
}

func (b *Bucket) getFile(key, filePath, name string, size int64, c cid.Cid, events chan PathEvent) error {
	if err := os.MkdirAll(filepath.Dir(name), os.ModePerm); err != nil {
		return err
	}
	file, err := os.Create(name)
	if err != nil {
		return err
	}
	defer file.Close()

	events <- PathEvent{
		Path: filePath,
		Cid:  c,
		Type: FileStart,
		Size: size,
	}

	progress := make(chan int64)
	go func() {
		for up := range progress {
			events <- PathEvent{
				Path:     filePath,
				Cid:      c,
				Type:     FileProgress,
				Size:     size,
				Progress: up,
			}
		}
	}()
	ctx, cancel := b.clients.Ctx.Thread(GetFileTimeout)
	defer cancel()
	if err := b.clients.Buckets.PullPath(ctx, key, filePath, file, client.WithProgress(progress)); err != nil {
		return err
	}
	events <- PathEvent{
		Path:     filePath,
		Cid:      c,
		Type:     FileComplete,
		Size:     size,
		Progress: size,
	}
	return nil
}
