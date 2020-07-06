package local

import (
	"github.com/ipfs/go-cid"
	pb "github.com/textileio/textile/api/buckets/pb"
	"github.com/textileio/textile/cmd"
)

type BucketItem struct {
	Cid   cid.Cid      `json:"cid"`
	Name  string       `json:"name"`
	Path  string       `json:"path"`
	Size  int64        `json:"size"`
	IsDir bool         `json:"is_dir"`
	Items []BucketItem `json:"items"`
}

func (b *Bucket) ListRemotePath(pth string) (items []BucketItem, err error) {
	if pth == "." || pth == "/" || pth == "./" {
		pth = ""
	}
	ctx, cancel := b.clients.Ctx.Thread(cmd.Timeout)
	defer cancel()
	key := b.conf.Viper.GetString("key")
	rep, err := b.clients.Buckets.ListPath(ctx, key, pth)
	if err != nil {
		return items, err
	}
	if len(rep.Item.Items) > 0 {
		items = make([]BucketItem, len(rep.Item.Items))
		for j, k := range rep.Item.Items {
			ii, err := pbItemToItem(k)
			if err != nil {
				return items, err
			}
			items[j] = ii
		}
	} else if !rep.Item.IsDir {
		items = make([]BucketItem, 1)
		item, err := pbItemToItem(rep.Item)
		if err != nil {
			return items, err
		}
		items[0] = item
	}
	return items, nil
}

func pbItemToItem(pi *pb.ListPathItem) (item BucketItem, err error) {
	c, err := cid.Decode(pi.Cid)
	if err != nil {
		return
	}
	items := make([]BucketItem, len(pi.Items))
	for j, k := range pi.Items {
		ii, err := pbItemToItem(k)
		if err != nil {
			return item, err
		}
		items[j] = ii
	}
	return BucketItem{
		Cid:   c,
		Name:  pi.Name,
		Path:  pi.Path,
		Size:  pi.Size,
		IsDir: pi.IsDir,
		Items: items,
	}, nil
}
