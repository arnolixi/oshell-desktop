package main

// A real subprocess used to exercise executable locking, restart receipts and
// rollback on both Windows and Linux. It never opens user configuration.
import (
	"os"
	"oshell-desktop/internal/core"
	"path/filepath"
	"strconv"
	"time"
)

func main() {
	if len(os.Args) == 3 && (os.Args[1] == "--create-state" || os.Args[1] == "--verify-state") {
		store := &core.Store{Dir: os.Args[2], Portable: true}
		if os.Args[1] == "--create-state" {
			if _, e := os.Stat(filepath.Join(store.Dir, "config.json")); !os.IsNotExist(e) {
				panic("fixture directory must not contain configuration")
			}
			cfg := core.DefaultConfig()
			p := core.NewProfile()
			p.Name = "update fixture"
			p.Host = "fixture.invalid"
			cfg.Profiles = []core.Profile{p}
			if e := store.Save(cfg); e != nil {
				panic(e)
			}
			v, e := core.OpenVault(store.Dir)
			if e != nil {
				panic(e)
			}
			if e = v.Put(p, "synthetic-update-secret"); e != nil {
				panic(e)
			}
			if e = core.SaveUpdateToken(store.Dir, core.DefaultUpdateRepository, "github_pat_synthetic_fixture"); e != nil {
				panic(e)
			}
		} else {
			cfg, e := store.Load()
			if e != nil {
				panic(e)
			}
			if len(cfg.Profiles) != 1 || cfg.Profiles[0].Name != "update fixture" {
				panic("fixture session lost")
			}
			v, e := core.OpenVault(store.Dir)
			if e != nil {
				panic(e)
			}
			password, e := v.Get(cfg.Profiles[0])
			if e != nil || password != "synthetic-update-secret" {
				panic("fixture credential lost")
			}
			if token, e := core.ReadUpdateToken(store.Dir, core.DefaultUpdateRepository); e != nil || token != "github_pat_synthetic_fixture" {
				panic("fixture update token lost")
			}
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "--hold" {
		for {
			time.Sleep(time.Second)
		}
	}
	if handled, err := core.HandlePortableUpdate(); err != nil {
		os.WriteFile("fixture-error", []byte(err.Error()), 0600)
		os.Exit(9)
	} else if handled {
		return
	}
	exe, _ := os.Executable()
	dir := filepath.Dir(exe)
	if os.Getenv("OSHELL_UPDATE_RECEIPT") != "" {
		if _, e := os.Stat(filepath.Join(dir, "fail-new")); e == nil {
			os.Exit(42)
		}
		if e := core.ConfirmPortableUpdate("0.3.2"); e != nil {
			os.Exit(43)
		}
		if _, e := os.Stat(filepath.Join(dir, "crash-after-ready")); e == nil {
			os.Exit(44)
		}
	}
	os.WriteFile(filepath.Join(dir, "child-started"), []byte(strconv.Itoa(os.Getpid())), 0600)
	for {
		time.Sleep(time.Second)
	}
}
