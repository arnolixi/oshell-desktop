//go:build !darwin

package core

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

func platformFonts(ctx context.Context) ([]FontEntry, error) {
	var command *exec.Cmd
	if runtime.GOOS == "windows" {
		script := `Add-Type -AssemblyName System.Drawing; [Console]::OutputEncoding = [System.Text.Encoding]::UTF8; $fonts = New-Object System.Drawing.Text.InstalledFontCollection; $bitmap = New-Object System.Drawing.Bitmap 1,1; $graphics = [System.Drawing.Graphics]::FromImage($bitmap); try { foreach ($family in $fonts.Families) { try { $font = New-Object System.Drawing.Font($family,13,[System.Drawing.FontStyle]::Regular,[System.Drawing.GraphicsUnit]::Pixel); try { $one = $graphics.MeasureString('i',$font,0,[System.Drawing.StringFormat]::GenericTypographic).Width; $two = $graphics.MeasureString('W',$font,0,[System.Drawing.StringFormat]::GenericTypographic).Width; $mono = 0; if ([Math]::Abs($one-$two) -lt 0.05) { $mono = 100 }; [Console]::WriteLine($family.Name + "` + "`t" + `" + $mono) } finally { $font.Dispose() } } catch {} } } finally { $graphics.Dispose(); $bitmap.Dispose(); $fonts.Dispose() }`
		command = exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
	} else {
		command = exec.CommandContext(ctx, "fc-list", ":charset=20-7e", "-f", "%{family[0]}\t%{spacing}\n")
	}
	var out limitedOutput
	command.Stdout = &out
	if e := command.Run(); e != nil {
		return nil, fmt.Errorf("无法枚举系统字体，可手动填写已安装字体名称")
	}
	var fonts []FontEntry
	for line := range strings.SplitSeq(out.String(), "\n") {
		name, spacing, _ := strings.Cut(strings.TrimSpace(line), "\t")
		fonts = append(fonts, FontEntry{name, spacing == "100" || spacing == "90"})
	}
	return fonts, nil
}
