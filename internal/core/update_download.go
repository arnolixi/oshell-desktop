package core

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type VerifiedDownload struct {
	path   string
	update *VerifiedUpdate
}

func (d *VerifiedDownload) ArchivePath() string { return d.path }
func (d *VerifiedDownload) Remove() error       { return os.Remove(d.path) }
func updateDownloadClient() *http.Client {
	return &http.Client{Timeout: 15 * time.Minute, CheckRedirect: func(request *http.Request, via []*http.Request) error {
		// Never forward a private-repository token to signed asset URLs.
		if request.URL.Host != "api.github.com" {
			request.Header.Del("Authorization")
		}
		if len(via) >= 8 || request.URL.Scheme != "https" || request.URL.User != nil {
			return fmt.Errorf("更新下载重定向无效")
		}
		switch request.URL.Host {
		case "github.com", "release-assets.githubusercontent.com", "objects.githubusercontent.com":
			return nil
		}
		return fmt.Errorf("更新下载跳转到未允许的主机")
	}}
}
func (u *VerifiedUpdate) Download(ctx context.Context, directory string, progress func(int64, int64)) (*VerifiedDownload, error) {
	return u.download(ctx, directory, progress, updateDownloadClient())
}
func (u *VerifiedUpdate) download(ctx context.Context, directory string, progress func(int64, int64), client *http.Client) (result *VerifiedDownload, err error) {
	if u == nil || len(u.publicKey) != ed25519.PublicKeySize || u.archive.Size <= 0 {
		return nil, fmt.Errorf("更新尚未通过签名验证")
	}
	if err = os.MkdirAll(directory, 0700); err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, u.archive.URL, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", "OShell Desktop")
	response, err := client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("更新下载失败")
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("更新下载返回 HTTP %d", response.StatusCode)
	}
	if response.ContentLength >= 0 && response.ContentLength != u.archive.Size {
		return nil, fmt.Errorf("安装包大小与签名信息不一致")
	}
	file, err := os.CreateTemp(directory, ".oshell-update-*.download")
	if err != nil {
		return nil, err
	}
	path := file.Name()
	defer func() {
		file.Close()
		if result == nil {
			os.Remove(path)
		}
	}()
	digest := sha256.New()
	buffer := make([]byte, 64<<10)
	var count int64
	last := time.Time{}
	for {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		n, e := response.Body.Read(buffer)
		if n > 0 {
			count += int64(n)
			if count > u.archive.Size {
				return nil, fmt.Errorf("安装包超过声明大小")
			}
			if _, err = file.Write(buffer[:n]); err != nil {
				return nil, err
			}
			digest.Write(buffer[:n])
			if progress != nil && time.Since(last) >= 100*time.Millisecond {
				progress(count, u.archive.Size)
				last = time.Now()
			}
		}
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, fmt.Errorf("更新下载中断")
		}
	}
	if count != u.archive.Size {
		return nil, fmt.Errorf("安装包下载不完整")
	}
	expected, _ := hex.DecodeString(u.archive.SHA256)
	actual := digest.Sum(nil)
	signature, _ := base64.StdEncoding.DecodeString(u.archive.Signature)
	if subtle.ConstantTimeCompare(expected, actual) != 1 || !ed25519.Verify(u.publicKey, actual, signature) {
		return nil, fmt.Errorf("安装包签名验证失败")
	}
	if err = file.Sync(); err != nil {
		return nil, err
	}
	if err = file.Close(); err != nil {
		return nil, err
	}
	if progress != nil {
		progress(count, count)
	}
	return &VerifiedDownload{path: path, update: u}, nil
}
func CheckSignedUpdate(ctx context.Context, cache *UpdateMetadataCache, source UpdateSource, target, applicationID, currentVersion, currentBuild, keyText string) (*VerifiedUpdate, error) {
	if _, e := ParseUpdatePublicKey(keyText); e != nil {
		return nil, e
	}
	load := func(ctx context.Context) ([]byte, error) {
		data, e := FetchUpdateMetadata(ctx, source)
		if e != nil {
			return nil, e
		}
		if _, e = DecodeSignedUpdate(data, source, target, applicationID, currentVersion, currentBuild, keyText); e != nil {
			return nil, e
		}
		return data, nil
	}
	data, e := cache.Value(ctx, load)
	if e != nil {
		return nil, e
	}
	return DecodeSignedUpdate(data, source, target, applicationID, currentVersion, currentBuild, keyText)
}
