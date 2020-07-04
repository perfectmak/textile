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
	confirm ConfirmFunc
	force   bool
	hard    bool
	events  chan PathEvent
}

type PathOption func(*pathOptions)

type ConfirmFunc func([]Change) bool

func WithConfirm(f ConfirmFunc) PathOption {
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
