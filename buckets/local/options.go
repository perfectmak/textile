package local

import (
	cid "github.com/ipfs/go-cid"
	"github.com/textileio/go-threads/core/thread"
)

type newOptions struct {
	name    string
	private bool
	thread  thread.ID
	key     string
	fromCid cid.Cid
	events  chan PathEvent
}

type NewOption func(*newOptions)

// WithName sets a name for the bucket.
func WithName(name string) NewOption {
	return func(args *newOptions) {
		args.name = name
	}
}

// WithPrivate specifies that an encryption key will be used for the bucket.
func WithPrivate(private bool) NewOption {
	return func(args *newOptions) {
		args.private = private
	}
}

// WithCid indicates that an inited bucket should be boostraped
// with a particular UnixFS DAG.
func WithCid(c cid.Cid) NewOption {
	return func(args *newOptions) {
		args.fromCid = c
	}
}

func WithExistingPathEvents(ch chan PathEvent) NewOption {
	return func(args *newOptions) {
		args.events = ch
	}
}

type pathOptions struct {
	confirm ConfirmDiffFunc
	force   bool
	hard    bool
	events  chan PathEvent
}

type PathOption func(*pathOptions)

type ConfirmDiffFunc func([]Change) bool

func WithConfirm(f ConfirmDiffFunc) PathOption {
	return func(args *pathOptions) {
		args.confirm = f
	}
}

func WithForce(b bool) PathOption {
	return func(args *pathOptions) {
		args.force = b
	}
}

func WithHard(b bool) PathOption {
	return func(args *pathOptions) {
		args.hard = b
	}
}

func WithPathEvents(ch chan PathEvent) PathOption {
	return func(args *pathOptions) {
		args.events = ch
	}
}

type addOptions struct {
	merge  SelectMergeFunc
	events chan PathEvent
}

type SelectMergeFunc func(description string, isDir bool) (MergeStrategy, error)

type MergeStrategy string

const (
	Skip    MergeStrategy = "Skip"
	Merge                 = "Merge"
	Replace               = "Replace"
)

type AddOption func(*addOptions)

func WithSelectMerge(f SelectMergeFunc) AddOption {
	return func(args *addOptions) {
		args.merge = f
	}
}

func WithAddEvents(ch chan PathEvent) AddOption {
	return func(args *addOptions) {
		args.events = ch
	}
}
