//go:build darwin

package main

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"os"
	"oshell-desktop/internal/core"
	"runtime"
)

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run() error {
	app := flag.String("app", "", "signed .app bundle")
	output := flag.String("out", "", "offline output directory; nothing is uploaded")
	repository := flag.String("repository", "", "public GitHub owner/repository")
	target := flag.String("target", "darwin-"+runtime.GOARCH, "darwin-arm64 or darwin-amd64")
	identity := flag.String("application-id", "dev.oshell.desktop", "expected Desktop bundle identifier")
	keyFile := flag.String("key-file", "", "private 0600 base64 Ed25519 key file")
	notes := flag.String("notes", "", "UTF-8 release notes file")
	generate := flag.Bool("generate-key", false, "explicitly create a new private key at -key-file; never overwrite")
	printKey := flag.Bool("public-key", false, "print only the public key for Info.plist configuration")
	flag.Parse()
	if *keyFile == "" {
		return fmt.Errorf("请通过 -key-file 指定私钥文件；不要把私钥放在命令行")
	}
	if *generate {
		public, e := core.GenerateUpdateSigningKeyFile(*keyFile)
		if e != nil {
			return e
		}
		fmt.Println(public)
		return nil
	}
	key, e := core.ReadUpdateSigningKey(*keyFile)
	if e != nil {
		return e
	}
	defer clear(key)
	if *printKey {
		fmt.Println(base64.StdEncoding.EncodeToString(key.Public().(ed25519.PublicKey)))
		return nil
	}
	if *app == "" || *output == "" || *repository == "" {
		return fmt.Errorf("需要 -app、-out 和 -repository")
	}
	text := ""
	if *notes != "" {
		file, e := os.Open(*notes)
		if e != nil {
			return e
		}
		data, e := io.ReadAll(io.LimitReader(file, 16385))
		file.Close()
		if e != nil || len(data) > 16384 {
			return fmt.Errorf("发布说明过大或无法读取")
		}
		text = string(data)
	}
	result, e := core.PublishMacOSUpdate(context.Background(), core.UpdatePublishOptions{Application: *app, Output: *output, Repository: *repository, Target: *target, ApplicationID: *identity, Notes: text}, key)
	if e != nil {
		return e
	}
	fmt.Println(result.ArchivePath)
	fmt.Println(result.MetadataPath)
	fmt.Println("SHA256:", result.SHA256)
	return nil
}
