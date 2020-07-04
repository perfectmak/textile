package local

import (
	"github.com/ipfs/go-cid"
	pb "github.com/textileio/textile/api/buckets/pb"
	"github.com/textileio/textile/cmd"
)

type BucketItem struct {
	Cid   cid.Cid      `json:"cid,omitempty"`
	Name  string       `json:"name,omitempty"`
	Path  string       `json:"path,omitempty"`
	Size  int64        `json:"size,omitempty"`
	IsDir bool         `json:"isDir,omitempty"`
	Items []BucketItem `json:"items,omitempty"`
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

func pbItemToItem(i *pb.ListPathItem) (b BucketItem, err error) {
	c, err := cid.Decode(i.Cid)
	if err != nil {
		return
	}
	items := make([]BucketItem, len(i.Items))
	for j, k := range i.Items {
		ii, err := pbItemToItem(k)
		if err != nil {
			return b, err
		}
		items[j] = ii
	}
	return BucketItem{
		Cid:   c,
		Name:  i.Name,
		Path:  i.Path,
		Size:  i.Size,
		IsDir: i.IsDir,
		Items: items,
	}, nil
}
