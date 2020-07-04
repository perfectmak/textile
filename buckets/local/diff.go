package local

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/ipfs/go-merkledag/dagutils"
	"github.com/logrusorgru/aurora"
	"github.com/textileio/textile/cmd"
)

type Change struct {
	Type dagutils.ChangeType
	Path string
	Rel  string
}

func ChangeType(t dagutils.ChangeType) string {
	switch t {
	case dagutils.Mod:
		return "modified:"
	case dagutils.Add:
		return "new file:"
	case dagutils.Remove:
		return "deleted: "
	default:
		return ""
	}
}

func ChangeColor(t dagutils.ChangeType) func(arg interface{}) aurora.Value {
	switch t {
	case dagutils.Mod:
		return aurora.Yellow
	case dagutils.Add:
		return aurora.Green
	case dagutils.Remove:
		return aurora.Red
	default:
		return nil
	}
}

func (b *Bucket) Diff() ([]Change, error) {
	cr, err := b.confRoot()
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(b.cwd, cr)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), cmd.Timeout)
	defer cancel()
	diff, err := b.repo.Diff(ctx, rel)
	if err != nil {
		return nil, err
	}
	var all []Change
	if len(diff) == 0 {
		return all, nil
	}
	for _, c := range diff {
		r := filepath.Join(rel, c.Path)
		switch c.Type {
		case dagutils.Mod, dagutils.Add:
			names, err := b.walkPath(r)
			if err != nil {
				return nil, err
			}
			if len(names) > 0 {
				for _, n := range names {
					p := strings.TrimPrefix(n, rel+"/")
					all = append(all, Change{Type: c.Type, Path: p, Rel: n})
				}
			} else {
				all = append(all, Change{Type: c.Type, Path: c.Path, Rel: r})
			}
		case dagutils.Remove:
			all = append(all, Change{Type: c.Type, Path: c.Path, Rel: r})
		}
	}
	return all, nil
}

func (b *Bucket) walkPath(pth string) (names []string, err error) {
	err = filepath.Walk(pth, func(n string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			f := strings.TrimPrefix(n, pth+"/")
			if Ignore(n) || strings.HasPrefix(f, b.conf.Dir) || strings.HasSuffix(f, PatchExt) {
				return nil
			}
			names = append(names, n)
		}
		return nil
	})
	if err != nil {
		return
	}
	return names, nil
}
