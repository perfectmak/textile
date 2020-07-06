package local

import (
	"fmt"

	"github.com/ipfs/go-cid"
	pb "github.com/textileio/textile/api/buckets/pb"
	"github.com/textileio/textile/cmd"
)

func (b *Bucket) ArchiveRemote() error {
	ctx, cancel := b.clients.Ctx.Thread(cmd.Timeout)
	defer cancel()
	key := b.conf.Viper.GetString("key")
	if _, err := b.clients.Buckets.Archive(ctx, key); err != nil {
		return err
	}
	return nil
}

type ArchiveStatusMessage struct {
	Type    ArchiveMessageType
	Message string
	Error   error
}

type ArchiveMessageType int

const (
	ArchiveMessage ArchiveMessageType = iota
	ArchiveWarning
	ArchiveError
	ArchiveSuccess
)

func (b *Bucket) ArchiveStatus(watch bool) (<-chan ArchiveStatusMessage, error) {
	ctx, cancel := b.clients.Ctx.Thread(cmd.Timeout)
	defer cancel()
	key := b.conf.Viper.GetString("key")
	rep, err := b.clients.Buckets.ArchiveStatus(ctx, key)
	if err != nil {
		return nil, err
	}
	msgs := make(chan ArchiveStatusMessage)
	go func() {
		defer close(msgs)
		switch rep.GetStatus() {
		case pb.ArchiveStatusReply_Failed:
			msgs <- ArchiveStatusMessage{
				Type:    ArchiveWarning,
				Message: "Archive failed with message: " + rep.GetFailedMsg(),
			}
		case pb.ArchiveStatusReply_Canceled:
			msgs <- ArchiveStatusMessage{
				Type:    ArchiveWarning,
				Message: "Archive was superseded by a new executing archive",
			}
		case pb.ArchiveStatusReply_Executing:
			msgs <- ArchiveStatusMessage{
				Type:    ArchiveMessage,
				Message: "Archive is currently executing, grab a coffee and be patient...",
			}
		case pb.ArchiveStatusReply_Done:
			msgs <- ArchiveStatusMessage{
				Type:    ArchiveSuccess,
				Message: "Archive executed successfully!",
			}
		default:
			msgs <- ArchiveStatusMessage{
				Type:    ArchiveWarning,
				Message: "Archive status unknown",
			}
		}
		if watch {
			ch := make(chan string)
			wCtx, cancel := b.clients.Ctx.Auth(cmd.TimeoutArchiveWatch)
			defer cancel()
			var err error
			go func() {
				err = b.clients.Buckets.ArchiveWatch(wCtx, key, ch)
				close(ch)
			}()
			for msg := range ch {
				msgs <- ArchiveStatusMessage{Type: ArchiveMessage, Message: "\t " + msg}
				sctx, scancel := b.clients.Ctx.Auth(cmd.TimeoutArchiveStatus)
				r, err := b.clients.Buckets.ArchiveStatus(sctx, key)
				if err != nil {
					msgs <- ArchiveStatusMessage{Type: ArchiveError, Error: err}
					cancel()
					return
				}
				scancel()
				final, err := isJobStatusFinal(r.GetStatus())
				if err != nil {
					msgs <- ArchiveStatusMessage{Type: ArchiveError, Error: err}
					cancel()
				} else if final {
					cancel()
				}
			}
			if err != nil {
				msgs <- ArchiveStatusMessage{Type: ArchiveError, Error: err}
			}
		}
	}()
	return msgs, nil
}

func isJobStatusFinal(status pb.ArchiveStatusReply_Status) (bool, error) {
	switch status {
	case pb.ArchiveStatusReply_Failed, pb.ArchiveStatusReply_Canceled, pb.ArchiveStatusReply_Done:
		return true, nil
	case pb.ArchiveStatusReply_Executing:
		return false, nil
	}
	return true, fmt.Errorf("unknown job status")

}

type ArchiveInfo struct {
	Key     string  `json:"key"`
	Archive Archive `json:"archive"`
}

type Archive struct {
	Cid   cid.Cid       `json:"cid"`
	Deals []ArchiveDeal `json:"deals"`
}

type ArchiveDeal struct {
	ProposalCid cid.Cid `json:"proposal_cid"`
	Miner       string  `json:"miner"`
}

func (b *Bucket) ArchiveInfo() (info ArchiveInfo, err error) {
	ctx, cancel := b.clients.Ctx.Thread(cmd.Timeout)
	defer cancel()
	key := b.conf.Viper.GetString("key")
	rep, err := b.clients.Buckets.ArchiveInfo(ctx, key)
	if err != nil {
		return info, err
	}
	return pbArchiveInfoToArchiveInfo(rep)
}

func pbArchiveInfoToArchiveInfo(pi *pb.ArchiveInfoReply) (info ArchiveInfo, err error) {
	info.Key = pi.Key
	if pi.Archive != nil {
		info.Archive.Cid, err = cid.Decode(pi.Archive.Cid)
		if err != nil {
			return
		}
		deals := make([]ArchiveDeal, len(pi.Archive.Deals))
		for i, d := range pi.Archive.Deals {
			deals[i].Miner = d.Miner
			deals[i].ProposalCid, err = cid.Decode(d.ProposalCid)
			if err != nil {
				return
			}
		}
	}
	return info, err
}
