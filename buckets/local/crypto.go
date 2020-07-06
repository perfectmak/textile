package local

import (
	"fmt"
	"io"
	"os"

	"github.com/textileio/dcrypto"
	"golang.org/x/sync/errgroup"
)

func (b *Bucket) EncryptLocalPath(pth, password string, w io.Writer) error {
	file, err := os.Open(pth)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("path %s is not a file", pth)
	}
	r, err := dcrypto.NewEncrypterWithPassword(file, []byte(password))
	if err != nil {
		return err
	}
	if _, err := io.Copy(w, r); err != nil {
		return err
	}
	return nil
}

func (b *Bucket) DecryptLocalPath(pth, password string, w io.Writer) error {
	reader, writer := io.Pipe()
	eg := new(errgroup.Group)
	eg.Go(func() error {
		ctx, cancel := b.clients.Ctx.Thread(GetFileTimeout)
		defer cancel()
		key := b.conf.Viper.GetString("key")
		if err := b.clients.Buckets.PullPath(ctx, key, pth, writer); err != nil {
			return err
		}
		return writer.Close()
	})
	eg.Go(func() error {
		r, err := dcrypto.NewDecrypterWithPassword(reader, []byte(password))
		if err != nil {
			return err
		}
		defer r.Close()
		if _, err := io.Copy(w, r); err != nil {
			return err
		}
		return nil
	})
	return eg.Wait()
}
