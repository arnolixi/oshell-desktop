package desktop

import (
	"fmt"
	"github.com/egoist/mygo/ui"
	"os"
	"oshell-desktop/internal/core"
	"path"
	"strings"
)

// Parsed once; classification uses directory metadata, never extra remote reads.
var (
	documentFileIcon = glyph(`<path d="M4 2.5h8l4 4v11H4zM12 2.5v4h4"/><path d="M7 10h6M7 13h6"/>`)
	archiveFileIcon  = glyph(`<rect x="3" y="3" width="14" height="14" rx="2"/><path d="M9 3h2v3H9v3h2v3H9v3h2"/>`)
	imageFileIcon    = glyph(`<rect x="2.5" y="3" width="15" height="14" rx="2"/><circle cx="7" cy="7" r="1.5"/><path d="m3 15 4-4 3 2 3-5 4 7"/>`)
	codeFileIcon     = glyph(`<path d="M4 2.5h8l4 4v11H4zM12 2.5v4h4M8 10l-2 2 2 2m4-4 2 2-2 2"/>`)
	mediaFileIcon    = glyph(`<rect x="3" y="3" width="14" height="14" rx="2"/><path d="m8 6 6 4-6 4z"/>`)
	audioFileIcon    = glyph(`<path d="M8 14V5l8-2v9M8 7l8-2"/><ellipse cx="5.5" cy="14.5" rx="2.5" ry="2"/><ellipse cx="13.5" cy="12.5" rx="2.5" ry="2"/>`)
)

type filePresentation struct {
	kind  string
	icon  *ui.SVG
	color ui.Color
}

func presentFile(entry os.FileInfo) filePresentation {
	mode := entry.Mode()
	makeStyle := func(kind string, icon *ui.SVG, color string) filePresentation {
		return filePresentation{kind, icon, ui.Hex(color)}
	}
	// A link to a directory must remain visibly a link.
	if mode&os.ModeSymlink != 0 {
		return makeStyle("链接", linkIcon, "#8B78C6")
	}
	if entry.IsDir() {
		return makeStyle("目录", folderIcon, "#428BC1")
	}
	if !mode.IsRegular() {
		return makeStyle("特殊文件", settingsIcon, "#8A8196")
	}
	switch strings.ToLower(path.Ext(entry.Name())) {
	case ".zip", ".tar", ".gz", ".bz2", ".xz", ".7z", ".rar", ".tgz", ".zst":
		return makeStyle("压缩包", archiveFileIcon, "#B58132")
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".ico", ".heic", ".bmp", ".tiff":
		return makeStyle("图片", imageFileIcon, "#4C9B70")
	case ".mp3", ".wav", ".flac", ".aac", ".ogg", ".m4a":
		return makeStyle("音频", audioFileIcon, "#AF72AE")
	case ".mp4", ".mov", ".mkv", ".avi", ".webm":
		return makeStyle("视频", mediaFileIcon, "#AF72AE")
	case ".go", ".py", ".js", ".ts", ".tsx", ".jsx", ".c", ".h", ".cpp", ".rs", ".swift", ".java", ".sh", ".bash", ".zsh", ".ps1":
		return makeStyle("代码", codeFileIcon, "#508FA6")
	case ".json", ".yaml", ".yml", ".toml", ".ini", ".conf", ".cfg", ".xml", ".env":
		return makeStyle("配置", settingsIcon, "#8B78C6")
	case ".pdf":
		return makeStyle("PDF", documentFileIcon, "#C06A60")
	case ".txt", ".md", ".log", ".csv", ".rtf", ".doc", ".docx":
		return makeStyle("文档", documentFileIcon, "#7B91A6")
	}
	if strings.EqualFold(entry.Name(), ".env") {
		return makeStyle("配置", settingsIcon, "#8B78C6")
	}
	if mode.Perm()&0111 != 0 {
		return makeStyle("可执行", terminalIcon, "#509975")
	}
	return makeStyle("文件", documentFileIcon, "#7B91A6")
}
func fileAttributeHint(entry os.FileInfo, kind string) string {
	attributes := core.RemoteFileAttributes(entry)
	permission := attributes.Permissions
	if attributes.Octal != "—" {
		permission += "（" + attributes.Octal + "）"
	}
	text := entry.Name() + "\n类型：" + kind + "\n权限：" + permission + "\n所属用户 UID：" + attributes.User + "\n所属组 GID：" + attributes.Group
	if strings.HasPrefix(entry.Name(), ".") {
		text += "\n隐藏项目"
	}
	if entry.Mode().IsRegular() && entry.Mode().Perm()&0111 != 0 {
		text += "\n具有执行权限"
	}
	if !entry.IsDir() {
		text += fmt.Sprintf("\n大小：%s（%d 字节）", bytesLabel(entry.Size()), entry.Size())
	}
	return text + "\n修改时间：" + fileModifiedDetail(entry)
}
