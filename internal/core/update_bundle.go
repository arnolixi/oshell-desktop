package core

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

const updateExpandedLimit int64 = 4 << 30
const updateEntryLimit = 100000

type AppBundleInfo struct {
	Identifier, Version, Build, Executable, MinimumOS string
	UpdateKey, UpdateRepository                       string
}

func ReadAppBundleInfo(bundle string) (AppBundleInfo, error) {
	var result AppBundleInfo
	file, e := openImportFile(filepath.Join(bundle, "Contents", "Info.plist"))
	if e != nil {
		return result, e
	}
	defer file.Close()
	data, e := io.ReadAll(io.LimitReader(file, 1<<20+1))
	if e != nil || len(data) > 1<<20 {
		return result, fmt.Errorf("应用信息过大或不可读")
	}
	decoder := xml.NewDecoder(bytes.NewReader(data))
	depth := 0
	rootDict := false
	finished := false
	values := map[string]string{}
	key := ""
	expectValue := false
	for {
		token, e := decoder.Token()
		if e == io.EOF {
			break
		}
		if e != nil {
			return result, fmt.Errorf("应用信息格式无效")
		}
		switch token := token.(type) {
		case xml.Directive:
			if strings.Contains(strings.ToUpper(string(token)), "ENTITY") {
				return result, fmt.Errorf("应用信息包含实体声明")
			}
		case xml.StartElement:
			depth++
			if depth == 1 {
				if token.Name.Local != "plist" || finished {
					return result, fmt.Errorf("应用信息根节点无效")
				}
				continue
			}
			if depth == 2 {
				if token.Name.Local != "dict" || rootDict {
					return result, fmt.Errorf("应用信息缺少唯一字典")
				}
				rootDict = true
				continue
			}
			if depth != 3 {
				return result, fmt.Errorf("应用信息结构无效")
			}
			if !expectValue {
				if token.Name.Local != "key" {
					return result, fmt.Errorf("应用信息键无效")
				}
				if e = decoder.DecodeElement(&key, &token); e != nil {
					return result, e
				}
				depth--
				if _, exists := values[key]; exists {
					return result, fmt.Errorf("应用信息包含重复键")
				}
				values[key] = ""
				expectValue = true
			} else {
				if token.Name.Local == "string" {
					var value string
					if e = decoder.DecodeElement(&value, &token); e != nil {
						return result, e
					}
					values[key] = value
				} else {
					if e = decoder.Skip(); e != nil {
						return result, e
					}
				}
				depth--
				expectValue = false
			}
		case xml.EndElement:
			depth--
			if depth == 0 {
				finished = true
			}
		}
	}
	if !rootDict || !finished || depth != 0 || expectValue {
		return result, fmt.Errorf("应用信息不完整")
	}
	result = AppBundleInfo{Identifier: values["CFBundleIdentifier"], Version: values["CFBundleShortVersionString"], Build: values["CFBundleVersion"], Executable: values["CFBundleExecutable"], MinimumOS: values["LSMinimumSystemVersion"], UpdateKey: values["OShellDesktopUpdatePublicKey"], UpdateRepository: values["OShellDesktopUpdateRepository"]}
	if result.Identifier == "" || result.Executable == "" || result.Executable == "." || result.Executable == ".." || strings.ContainsAny(result.Executable, "/\\") || strings.ContainsFunc(result.Executable, unicode.IsControl) {
		return result, fmt.Errorf("应用标识或执行文件名无效")
	}
	return result, nil
}

type updateContextReader struct {
	context.Context
	reader io.Reader
}

func (r updateContextReader) Read(data []byte) (int, error) {
	if e := r.Err(); e != nil {
		return 0, e
	}
	return r.reader.Read(data)
}
func (d *VerifiedDownload) verifiedArchive(ctx context.Context) (*os.File, error) {
	if d == nil || d.update == nil || len(d.update.publicKey) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("更新尚未验证")
	}
	file, e := openImportFile(d.path)
	if e != nil {
		return nil, e
	}
	fail := func(err error) (*os.File, error) { file.Close(); return nil, err }
	info, e := file.Stat()
	if e != nil || !info.Mode().IsRegular() || info.Size() != d.update.archive.Size {
		return fail(fmt.Errorf("暂存安装包大小或类型已变化"))
	}
	hash := sha256.New()
	n, e := io.Copy(hash, io.LimitReader(updateContextReader{ctx, file}, d.update.archive.Size+1))
	if e != nil {
		return fail(e)
	}
	expected, _ := hex.DecodeString(d.update.archive.SHA256)
	signature, _ := base64.StdEncoding.DecodeString(d.update.archive.Signature)
	actual := hash.Sum(nil)
	if n != d.update.archive.Size || subtle.ConstantTimeCompare(expected, actual) != 1 || !ed25519.Verify(d.update.publicKey, actual, signature) {
		return fail(fmt.Errorf("暂存安装包签名不匹配"))
	}
	if _, e = file.Seek(0, io.SeekStart); e != nil {
		return fail(e)
	}
	return file, nil
}
func extractUpdateBundle(ctx context.Context, input io.Reader, destination string) (string, error) {
	gzipReader, e := gzip.NewReader(input)
	if e != nil {
		return "", fmt.Errorf("更新压缩包无效")
	}
	defer gzipReader.Close()
	reader := tar.NewReader(updateContextReader{ctx, gzipReader})
	root := ""
	seen := map[string]bool{}
	fold := cases.Fold()
	var total int64
	count := 0
	type link struct{ name, target string }
	var links []link
	for {
		if e = ctx.Err(); e != nil {
			return "", e
		}
		header, e := reader.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			return "", fmt.Errorf("更新归档损坏")
		}
		count++
		if count > updateEntryLimit {
			return "", fmt.Errorf("更新归档项目过多")
		}
		name := strings.TrimSuffix(header.Name, "/")
		if !utf8.ValidString(name) || name == "" || len(name) > 4096 || strings.ContainsAny(name, "\\\x00") || strings.ContainsFunc(name, unicode.IsControl) || path.IsAbs(name) || path.Clean(name) != name {
			return "", fmt.Errorf("更新归档路径无效")
		}
		parts := strings.Split(name, "/")
		for _, part := range parts {
			if part == "." || part == ".." || part == "" || len(part) > 255 {
				return "", fmt.Errorf("更新归档路径无效")
			}
		}
		if root == "" {
			root = parts[0]
			if !strings.HasSuffix(root, ".app") || strings.HasPrefix(root, ".") {
				return "", fmt.Errorf("安装包必须仅包含一个应用")
			}
		}
		if parts[0] != root {
			return "", fmt.Errorf("安装包包含额外根目录")
		}
		key := fold.String(norm.NFC.String(name))
		if seen[key] {
			return "", fmt.Errorf("更新归档路径重复或大小写冲突")
		}
		seen[key] = true
		target := filepath.Join(destination, filepath.FromSlash(name))
		switch header.Typeflag {
		case tar.TypeDir:
			if e = os.MkdirAll(target, 0755); e != nil {
				return "", e
			}
		case tar.TypeReg, tar.TypeRegA:
			if header.Size < 0 || header.Size > UpdateArchiveLimit || header.Size > updateExpandedLimit-total {
				return "", fmt.Errorf("更新解包超过大小限制")
			}
			total += header.Size
			if e = os.MkdirAll(filepath.Dir(target), 0755); e != nil {
				return "", e
			}
			file, e := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
			if e != nil {
				return "", e
			}
			_, copyErr := io.CopyN(file, reader, header.Size)
			if copyErr == nil {
				copyErr = file.Sync()
			}
			if copyErr == nil {
				copyErr = file.Chmod(os.FileMode(header.Mode) & 0777)
			}
			closeErr := file.Close()
			if copyErr != nil {
				return "", copyErr
			}
			if closeErr != nil {
				return "", closeErr
			}
		case tar.TypeSymlink:
			// Framework-style downward relative links are supported. Parent traversal
			// is unnecessary for our bundles and unsafe through another link's target.
			if header.Linkname == "" || path.IsAbs(header.Linkname) || strings.ContainsAny(header.Linkname, "\\\x00") || strings.ContainsFunc(header.Linkname, unicode.IsControl) {
				return "", fmt.Errorf("更新符号链接无效")
			}
			for _, part := range strings.Split(header.Linkname, "/") {
				if part == ".." {
					return "", fmt.Errorf("更新符号链接越界")
				}
			}
			if resolved := path.Clean(path.Join(path.Dir(name), header.Linkname)); resolved != root && !strings.HasPrefix(resolved, root+"/") {
				return "", fmt.Errorf("更新符号链接越界")
			}
			links = append(links, link{name, header.Linkname})
		default:
			return "", fmt.Errorf("更新归档包含不支持的特殊文件")
		}
	}
	if root == "" {
		return "", fmt.Errorf("更新归档为空")
	}
	// Consume the gzip trailer too, so CRC corruption is not ignored at tar EOF.
	trailing, e := io.ReadAll(io.LimitReader(updateContextReader{ctx, gzipReader}, 1<<20+1))
	if e != nil || len(trailing) > 1<<20 {
		return "", fmt.Errorf("更新压缩校验失败")
	}
	for _, value := range trailing {
		if value != 0 {
			return "", fmt.Errorf("更新归档末尾含额外内容")
		}
	}
	for _, entry := range links {
		target := filepath.Join(destination, filepath.FromSlash(entry.name))
		parent := filepath.Dir(target)
		if e = os.MkdirAll(parent, 0755); e != nil {
			return "", e
		}
		for check := parent; check != destination; check = filepath.Dir(check) {
			info, e := os.Lstat(check)
			if e != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				return "", fmt.Errorf("符号链接的父目录无效")
			}
		}
		if e = os.Symlink(entry.target, target); e != nil {
			return "", e
		}
	}
	return filepath.Join(destination, root), nil
}
