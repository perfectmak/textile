package local

import cid "github.com/ipfs/go-cid"

type newOptions struct {
	name       string
	private    bool
	fromBucket *BucketInfo
	fromCid    cid.Cid
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

// WithBucket indicates that an inited bucket should be a mirror of an
// existing bucket. Use this option to pull down buckets created on
// a different machine or by other org members.
func WithBucket(b BucketInfo) NewOption {
	return func(args *newOptions) {
		args.fromBucket = &b
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

func WithEvents(ch chan PathEvent) PathOption {
	return func(args *pathOptions) {
		args.events = ch
	}
}
