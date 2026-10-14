//go:build darwin

package core

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json/v2"
	"fmt"
	"golang.org/x/sys/unix"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type UpdatePublishOptions struct{ Application, Output, Repository, Target, ApplicationID, Notes string }
type UpdateReleaseArtifacts struct{ ArchivePath, MetadataPath, PublicKey, SHA256 string }

func ReadUpdateSigningKey(path string) (ed25519.PrivateKey, error) {
	file, e := openImportFile(path)
	if e != nil {
		return nil, fmt.Errorf("无法打开签名私钥文件")
	}
	defer file.Close()
	var stat unix.Stat_t
	if unix.Fstat(int(file.Fd()), &stat) != nil || stat.Mode&unix.S_IFMT != unix.S_IFREG || stat.Uid != uint32(os.Geteuid()) || stat.Mode&0077 != 0 || stat.Size > 4096 {
		return nil, fmt.Errorf("签名私钥必须是当前用户私有的普通文件")
	}
	data, e := io.ReadAll(io.LimitReader(file, 4097))
	if e != nil || len(data) > 4096 {
		return nil, fmt.Errorf("签名私钥读取失败")
	}
	defer clear(data)
	decoded, e := base64.StdEncoding.DecodeString(strings.TrimSpace(string(data)))
	if e != nil || len(decoded) != ed25519.PrivateKeySize {
		clear(decoded)
		return nil, fmt.Errorf("需要 base64 编码的 64 字节 Ed25519 私钥")
	}
	derived := ed25519.NewKeyFromSeed(decoded[:ed25519.SeedSize])
	defer clear(derived)
	if subtle.ConstantTimeCompare(decoded, derived) != 1 {
		clear(decoded)
		return nil, fmt.Errorf("签名私钥内容不一致")
	}
	return ed25519.PrivateKey(decoded), nil
}
func PublishMacOSUpdate(ctx context.Context, options UpdatePublishOptions, key ed25519.PrivateKey) (result UpdateReleaseArtifacts, err error) {
	if len(key) != ed25519.PrivateKeySize {
		return result, fmt.Errorf("签名私钥无效")
	}
	if options.ApplicationID != "dev.oshell.desktop" && options.ApplicationID != "dev.oshell.desktop.fullpreview" {
		return result, fmt.Errorf("只能发布 OShell Desktop 产品")
	}
	if options.Target != "darwin-arm64" && options.Target != "darwin-amd64" {
		return result, fmt.Errorf("请选择 macOS 目标架构")
	}
	source, e := ParseUpdateSource(options.Repository)
	if e != nil {
		return result, e
	}
	info, e := ReadAppBundleInfo(options.Application)
	if e != nil {
		return result, e
	}
	pub := key.Public().(ed25519.PublicKey)
	publicKey := base64.StdEncoding.EncodeToString(pub)
	pinned, e := ParseUpdatePublicKey(info.UpdateKey)
	if e != nil || subtle.ConstantTimeCompare(pinned, pub) != 1 {
		return result, fmt.Errorf("应用内公钥与发布私钥不匹配")
	}
	if info.Identifier != options.ApplicationID || !updateBuildPattern.MatchString(info.Build) || info.MinimumOS != "13.0" {
		return result, fmt.Errorf("应用标识、数字构建号或最低系统版本无效")
	}
	if _, e = CompareUpdateVersions(info.Version, info.Version); e != nil {
		return result, e
	}
	if info.UpdateRepository != "" {
		configured, e := ParseUpdateSource(info.UpdateRepository)
		if e != nil || !strings.EqualFold(configured.Repository, source.Repository) {
			return result, fmt.Errorf("应用默认更新仓库与发布目标不匹配")
		}
	}
	if e = verifyNativeUpdateBundle(ctx, options.Application, options.Target); e != nil {
		return result, e
	}
	if e = os.MkdirAll(options.Output, 0755); e != nil {
		return result, e
	}
	lock, e := launchLock(options.Output, "update-release.lock")
	if e != nil {
		return result, fmt.Errorf("发布目录不可用或正被使用：%w", e)
	}
	defer lock.Close()
	work, e := os.MkdirTemp(options.Output, ".oshell-release-")
	if e != nil {
		return result, e
	}
	defer os.RemoveAll(work)
	archivePath := filepath.Join(work, "application.tar.gz")
	file, e := os.OpenFile(archivePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return result, e
	}
	hash := sha256.New()
	zip := gzip.NewWriter(io.MultiWriter(&limitedUpdateWriter{writer: file, remaining: UpdateArchiveLimit}, hash))
	writer := tar.NewWriter(zip)
	root := filepath.Clean(options.Application)
	parent := filepath.Dir(root)
	var expanded int64
	entries := 0
	e = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if e := ctx.Err(); e != nil {
			return e
		}
		entries++
		if entries > updateEntryLimit {
			return fmt.Errorf("应用项目过多")
		}
		info, e := entry.Info()
		if e != nil {
			return e
		}
		if info.Mode().IsRegular() {
			if info.Size() < 0 || info.Size() > UpdateArchiveLimit || info.Size() > updateExpandedLimit-expanded {
				return fmt.Errorf("应用超过解包大小限制")
			}
			expanded += info.Size()
		}
		link := ""
		if info.Mode()&os.ModeSymlink != 0 {
			link, e = os.Readlink(path)
			if e != nil {
				return e
			}
		} else if !info.IsDir() && !info.Mode().IsRegular() {
			return fmt.Errorf("应用包含特殊文件")
		}
		header, e := tar.FileInfoHeader(info, link)
		if e != nil {
			return e
		}
		relative, e := filepath.Rel(parent, path)
		if e != nil {
			return e
		}
		header.Name = filepath.ToSlash(relative)
		header.Uid = 0
		header.Gid = 0
		header.Uname = ""
		header.Gname = ""
		if e = writer.WriteHeader(header); e != nil {
			return e
		}
		if info.Mode().IsRegular() {
			input, e := openImportFile(path)
			if e != nil {
				return e
			}
			_, e = io.Copy(writer, updateContextReader{ctx, input})
			input.Close()
			return e
		}
		return nil
	})
	if closeErr := writer.Close(); e == nil {
		e = closeErr
	}
	if closeErr := zip.Close(); e == nil {
		e = closeErr
	}
	if syncErr := file.Sync(); e == nil {
		e = syncErr
	}
	if closeErr := file.Close(); e == nil {
		e = closeErr
	}
	if e != nil {
		return result, e
	}
	stat, e := os.Stat(archivePath)
	if e != nil || stat.Size() > UpdateArchiveLimit {
		return result, fmt.Errorf("发布安装包超过大小限制")
	}
	// Round-trip through the consumer's exact extraction and code-verification
	// path before signing/publishing, including framework-link constraints.
	verifyDir := filepath.Join(work, "verify")
	if e = os.Mkdir(verifyDir, 0700); e != nil {
		return result, e
	}
	archiveFile, e := os.Open(archivePath)
	if e != nil {
		return result, e
	}
	bundle, e := extractUpdateBundle(ctx, archiveFile, verifyDir)
	archiveFile.Close()
	if e != nil {
		return result, e
	}
	extracted, e := ReadAppBundleInfo(bundle)
	if e != nil || extracted != info {
		return result, fmt.Errorf("打包期间应用信息发生变化")
	}
	if e = verifyNativeUpdateBundle(ctx, bundle, options.Target); e != nil {
		return result, e
	}
	filename := "OShell-Desktop-" + info.Version + "-" + options.Target + ".tar.gz"
	digest := hash.Sum(nil)
	archive := UpdateArchive{URL: "https://github.com/" + source.Repository + "/releases/download/v" + info.Version + "/" + filename, Size: stat.Size(), SHA256: hex.EncodeToString(digest), Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(key, digest)), MinimumOS: info.MinimumOS}
	manifest := UpdateManifest{Repository: source.Repository, ApplicationID: info.Identifier, Version: info.Version, Build: info.Build, Notes: options.Notes, Platforms: map[string]UpdateArchive{options.Target: archive}}
	metadataDir := filepath.Join(options.Output, "updates")
	metadataPath := filepath.Join(metadataDir, "latest.json")
	if existing, e := readUpdateReleaseMetadata(metadataPath); e == nil {
		var envelope signedUpdateEnvelope
		var previous UpdateManifest
		if len(existing) > UpdateMetadataLimit || json.Unmarshal(existing, &envelope) != nil || envelope.Format != "OShellDesktop.signed-updates.v1" || !ed25519.Verify(pub, append([]byte(updateSignatureDomain), envelope.Payload...), envelope.Signature) || json.Unmarshal(envelope.Payload, &previous) != nil {
			return result, fmt.Errorf("发布目录已有无效或不同密钥的元数据")
		}
		if previous.ApplicationID != info.Identifier || !strings.EqualFold(previous.Repository, source.Repository) {
			return result, fmt.Errorf("发布目录属于其他应用或仓库")
		}
		comparison, e := CompareUpdateVersions(info.Version, previous.Version)
		oldBuild, oldErr := strconv.ParseUint(previous.Build, 10, 64)
		newBuild, _ := strconv.ParseUint(info.Build, 10, 64)
		if e != nil || oldErr != nil || comparison < 0 || (comparison == 0 && newBuild < oldBuild) {
			return result, fmt.Errorf("发布不能回退现有版本或构建号")
		}
		for target := range previous.Platforms {
			if _, e := DecodeSignedUpdate(existing, source, target, info.Identifier, "0.0.0", "", publicKey); e != nil {
				return result, fmt.Errorf("现有平台元数据无效：%w", e)
			}
		}
		if previous.Version == info.Version && previous.Build == info.Build && previous.ApplicationID == info.Identifier && strings.EqualFold(previous.Repository, source.Repository) {
			for target, value := range previous.Platforms {
				if target != options.Target {
					manifest.Platforms[target] = value
				}
			}
		}
	} else if !os.IsNotExist(e) {
		return result, e
	}
	metadata, e := SignUpdateManifest(manifest, key)
	if e != nil {
		return result, e
	}
	if _, e = DecodeSignedUpdate(metadata, source, options.Target, info.Identifier, "0.0.0", "", publicKey); e != nil {
		return result, e
	}
	if e = os.MkdirAll(metadataDir, 0755); e != nil {
		return result, e
	}
	finalArchive := filepath.Join(options.Output, filename)
	// A hard link publishes the complete file atomically without overwriting an
	// existing version, and the temporary tree can then be removed normally.
	if e = os.Link(archivePath, finalArchive); e != nil {
		return result, fmt.Errorf("安装包已存在或无法发布：%w", e)
	}
	published := false
	defer func() {
		if !published {
			os.Remove(finalArchive)
		}
	}()
	if e = os.Chmod(finalArchive, 0644); e != nil {
		return result, e
	}
	if e = ctx.Err(); e != nil {
		return result, e
	}
	if e = AtomicWrite(metadataPath, metadata); e != nil {
		return result, e
	}
	published = true
	return UpdateReleaseArtifacts{ArchivePath: finalArchive, MetadataPath: metadataPath, PublicKey: publicKey, SHA256: archive.SHA256}, nil
}

type limitedUpdateWriter struct {
	writer    io.Writer
	remaining int64
}

func (w *limitedUpdateWriter) Write(data []byte) (int, error) {
	if int64(len(data)) > w.remaining {
		return 0, fmt.Errorf("发布安装包超过大小限制")
	}
	n, e := w.writer.Write(data)
	w.remaining -= int64(n)
	return n, e
}
func readUpdateReleaseMetadata(path string) ([]byte, error) {
	file, e := openImportFile(path)
	if e != nil {
		return nil, e
	}
	defer file.Close()
	data, e := io.ReadAll(io.LimitReader(file, UpdateMetadataLimit+1))
	if e != nil {
		return nil, e
	}
	if len(data) > UpdateMetadataLimit {
		return nil, fmt.Errorf("现有更新元数据过大")
	}
	return data, nil
}

// GenerateUpdateSigningKeyFile is an explicit release-maintainer operation.
// It never overwrites a key and returns only the public half for build metadata.
func GenerateUpdateSigningKeyFile(path string) (string, error) {
	public, private, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		return "", e
	}
	defer clear(private)
	file, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return "", e
	}
	encoded := make([]byte, base64.StdEncoding.EncodedLen(len(private))+1)
	base64.StdEncoding.Encode(encoded[:len(encoded)-1], private)
	encoded[len(encoded)-1] = '\n'
	defer clear(encoded)
	_, e = file.Write(encoded)
	if e == nil {
		e = file.Sync()
	}
	closeErr := file.Close()
	if e == nil {
		e = closeErr
	}
	if e != nil {
		os.Remove(path)
		return "", e
	}
	return base64.StdEncoding.EncodeToString(public), nil
}
