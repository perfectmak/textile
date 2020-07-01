package local

import cid "github.com/ipfs/go-cid"

type initOptions struct {
	name         string
	private      bool
	bootstrapCid cid.Cid
}

type InitOption func(*initOptions)

// WithName sets a name for the bucket.
func WithName(name string) InitOption {
	return func(args *initOptions) {
		args.name = name
	}
}

// WithPrivate specifies that an encryption password will be used for the bucket.
func WithPrivate(private bool) InitOption {
	return func(args *initOptions) {
		args.private = private
	}
}

// WithCid indicates that an inited bucket should be boostraped
// with a particular UnixFS DAG.
func WithCid(c cid.Cid) InitOption {
	return func(args *initOptions) {
		args.bootstrapCid = c
	}
}
