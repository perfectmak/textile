package local

import (
	"fmt"
	"io"
	"os"

	"github.com/textileio/dcrypto"
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
	errs := make(chan error)
	go func() {
		ctx, cancel := b.clients.Ctx.Thread(GetFileTimeout)
		defer cancel()
		key := b.conf.Viper.GetString("key")
		if err := b.clients.Buckets.PullPath(ctx, key, pth, writer); err != nil {
			errs <- err
			return
		}
		if err := writer.Close(); err != nil {
			errs <- err
			return
		}
	}()
	go func() {
		r, err := dcrypto.NewDecrypterWithPassword(reader, []byte(password))
		if err != nil {
			errs <- err
			return
		}
		defer r.Close()
		if _, err := io.Copy(w, r); err != nil {
			errs <- err
			return
		}
		close(errs)
	}()
	return <-errs
}
