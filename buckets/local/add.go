package local

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/textileio/textile/api/buckets/client"
	"github.com/textileio/textile/cmd"
	"golang.org/x/sync/errgroup"
)

func (b *Bucket) AddRemoteCid(c cid.Cid, dest string, opts ...AddOption) error {
	args := &addOptions{}
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
	return b.mergeIpfsPath(path.IpfsPath(c), dest, args.merge, events)
}

func (b *Bucket) mergeIpfsPath(ipfsBasePth path.Path, dest string, merge SelectMergeFunc, events chan PathEvent) error {
	ok, err := b.containsPath(dest)
	if err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("destination %s is not in bucket path", dest)
	}

	folderReplace, toAdd, err := b.listMergePath(ipfsBasePth, "", dest, merge)
	if err != nil {
		return err
	}

	// Remove all the folders that were decided to be replaced.
	for _, fr := range folderReplace {
		if err := os.RemoveAll(fr); err != nil {
			return err
		}
	}

	// Add files that are missing, or were decided to be overwritten.
	if len(toAdd) > 0 {
		events <- PathEvent{
			Path: ipfsBasePth.String(),
			Type: PathStart,
		}
		eg, gctx := errgroup.WithContext(context.Background())
		for _, o := range toAdd {
			o := o
			eg.Go(func() error {
				if gctx.Err() != nil {
					return nil
				}
				if err := os.Remove(o.path); err != nil && !os.IsNotExist(err) {
					return err
				}
				trimmedDest := strings.TrimLeft(o.path, dest)
				return b.getIpfsFile(path.Join(ipfsBasePth, trimmedDest), o.path, o.size, o.cid, events)
			})
		}
		if err := eg.Wait(); err != nil {
			return err
		}
		events <- PathEvent{
			Path: ipfsBasePth.String(),
			Type: PathComplete,
		}
	}
	return nil
}

// listMergePath walks the local bucket and the remote IPFS UnixFS DAG asking
// the client if wants to (replace, merge, ignore) matching folders, and if wants
// to (overwrite, ignore) matching files. Any non-matching files or folders in the
// IPFS UnixFS DAG will be added locally.
// The first return value is a slice of paths of folders that were decided to be
// replaced completely (not merged). The second return value are a list of files
// that should be added locally. If one of them exist, can be understood that should
// be overwritten.
func (b *Bucket) listMergePath(ipfsBasePth path.Path, ipfsRelPath, dest string, merge SelectMergeFunc) ([]string, []object, error) {
	// List remote IPFS UnixFS path level
	ctx, cancel := b.clients.Ctx.Thread(cmd.Timeout)
	defer cancel()
	rep, err := b.clients.Buckets.ListIpfsPath(ctx, path.Join(ipfsBasePth, ipfsRelPath))
	if err != nil {
		return nil, nil, err
	}

	// If its a dir, ask if should be ignored, replaced, or merged.
	if rep.Item.IsDir {
		var replacedFolders []string
		var toAdd []object

		var folderExists bool

		localFolderPath := filepath.Join(dest, ipfsRelPath)
		if _, err := os.Stat(localFolderPath); err == nil {
			folderExists = true
		}

		if folderExists && merge != nil {
			ms, err := merge(fmt.Sprintf("Merge strategy for  %s", localFolderPath), true)
			if err != nil {
				return nil, nil, err
			}
			switch ms {
			case Skip:
				return nil, nil, nil
			case Merge:
				break
			case Replace:
				replacedFolders = append(replacedFolders, localFolderPath)
				merge = nil
			}
		}
		for _, i := range rep.Item.Items {
			nestFolderReplace, nestAdd, err := b.listMergePath(ipfsBasePth, filepath.Join(ipfsRelPath, i.Name), dest, merge)
			if err != nil {
				return nil, nil, err
			}
			replacedFolders = append(replacedFolders, nestFolderReplace...)
			toAdd = append(toAdd, nestAdd...)
		}
		return replacedFolders, toAdd, nil
	}

	// If it's a file it exists, confirm whether or not it should be overwritten.
	pth := filepath.Join(dest, ipfsRelPath)
	if _, err := os.Stat(pth); err == nil && merge != nil {
		ms, err := merge(fmt.Sprintf("Overwrite  %s", pth), false)
		if err != nil {
			return nil, nil, err
		}
		switch ms {
		case Skip:
			return nil, nil, nil
		case Merge:
			return nil, nil, fmt.Errorf("cannot merge files")
		case Replace:
			break
		}
	} else if err != nil && os.IsNotExist(err) {
		return nil, nil, err
	}

	c, err := cid.Decode(rep.Item.Cid)
	if err != nil {
		return nil, nil, err
	}
	o := object{path: pth, name: rep.Item.Name, size: rep.Item.Size, cid: c}
	return nil, []object{o}, nil
}

func (b *Bucket) getIpfsFile(ipfsPath path.Path, filePath string, size int64, c cid.Cid, events chan PathEvent) error {
	if err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
		return err
	}
	file, err := os.Create(filePath)
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
	if err := b.clients.Buckets.PullIpfsPath(ctx, ipfsPath, file, client.WithProgress(progress)); err != nil {
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
