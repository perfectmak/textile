package local

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/ipfs/go-merkledag/dagutils"
	"github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/textileio/textile/api/buckets/client"
	"github.com/textileio/textile/cmd"
)

func (b *Bucket) PushLocalPath(opts ...PathOption) (roots Roots, err error) {
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
	cr, err := b.confRoot()
	if err != nil {
		return roots, err
	}
	if args.force { // Reset the diff to show all files as additions
		rel, err := filepath.Rel(b.cwd, cr)
		if err != nil {
			return roots, err
		}
		var reset []Change
		add, err := b.walkPath(cr)
		if err != nil {
			return roots, err
		}
		for _, a := range add {
			p := strings.TrimPrefix(a, rel+"/")
			reset = append(reset, Change{Type: dagutils.Add, Path: p, Rel: a})
		}
		// Add unique additions
	loop:
		for _, c := range reset {
			for _, x := range diff {
				if c.Path == x.Path {
					continue loop
				}
			}
			diff = append(diff, c)
		}
	}
	if len(diff) == 0 {
		return roots, ErrUpToDate
	}
	if args.confirm != nil {
		if ok := args.confirm(diff); !ok {
			return roots, ErrAborted
		}
	}

	r, err := b.Roots()
	if err != nil {
		return
	}
	xr := path.IpfsPath(r.Remote)
	var rm []Change
	events <- PathEvent{
		Path: cr,
		Type: PathStart,
	}
	key := b.conf.Viper.GetString("key")
	for _, c := range diff {
		switch c.Type {
		case dagutils.Mod, dagutils.Add:
			var added path.Resolved
			var err error
			added, xr, err = b.addFile(key, xr, c.Rel, c.Path, args.force, events)
			if err != nil {
				return roots, err
			}
			if err := b.repo.SetRemotePath(c.Rel, added.Cid()); err != nil {
				return roots, err
			}
		case dagutils.Remove:
			rm = append(rm, c)
		}
	}
	events <- PathEvent{
		Path: cr,
		Type: PathComplete,
	}
	if len(rm) > 0 {
		for _, c := range rm {
			var err error
			xr, err = b.rmFile(key, xr, c.Path, args.force, events)
			if err != nil {
				return roots, err
			}
			ctx, cancel := context.WithTimeout(context.Background(), cmd.Timeout)
			if err := b.repo.RemovePath(ctx, c.Rel); err != nil {
				cancel()
				return roots, err
			}
			cancel()
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), cmd.Timeout)
	defer cancel()
	if err = b.repo.Save(ctx); err != nil {
		return
	}
	rc, err := b.getRemoteRoot()
	if err != nil {
		return roots, err
	}
	if err = b.repo.SetRemotePath("", rc); err != nil {
		return
	}
	return b.Roots()
}

func (b *Bucket) addFile(key string, xroot path.Resolved, name, filePath string, force bool, events chan PathEvent) (added path.Resolved, root path.Resolved, err error) {
	file, err := os.Open(name)
	if err != nil {
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return
	}
	size := info.Size()

	events <- PathEvent{
		Path: filePath,
		Type: FileStart,
		Size: size,
	}

	progress := make(chan int64)
	go func() {
		for up := range progress {
			var u int64
			if up > size {
				u = size
			} else {
				u = up
			}
			events <- PathEvent{
				Path:     filePath,
				Type:     FileProgress,
				Size:     size,
				Progress: u,
			}
		}
	}()

	ctx, cancel := b.clients.Ctx.Thread(AddFileTimeout)
	defer cancel()
	opts := []client.Option{client.WithProgress(progress)}
	if !force {
		opts = append(opts, client.WithFastForwardOnly(xroot))
	}
	added, root, err = b.clients.Buckets.PushPath(ctx, key, filePath, file, opts...)
	if err != nil {
		return
	} else {
		events <- PathEvent{
			Path:     filePath,
			Cid:      added.Cid(),
			Type:     FileComplete,
			Size:     size,
			Progress: size,
		}
	}
	return added, root, nil
}

func (b *Bucket) rmFile(key string, xroot path.Resolved, filePath string, force bool, events chan PathEvent) (path.Resolved, error) {
	ctx, cancel := b.clients.Ctx.Thread(AddFileTimeout)
	defer cancel()
	var opts []client.Option
	if !force {
		opts = append(opts, client.WithFastForwardOnly(xroot))
	}
	root, err := b.clients.Buckets.RemovePath(ctx, key, filePath, opts...)
	if err != nil {
		if !strings.HasSuffix(err.Error(), "no link by that name") {
			return nil, err
		}
	}
	events <- PathEvent{
		Path: filePath,
		Type: FileRemoved,
	}
	return root, nil
}
