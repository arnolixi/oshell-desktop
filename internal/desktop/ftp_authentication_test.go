package desktop

import (
	"bufio"
	"fmt"
	"net"
	"oshell-desktop/internal/core"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// This fixture pauses PASS so the UI can change the session before login completes.
func TestFTPLoginCannotOverwriteSessionChangedDuringAuthentication(t *testing.T) {
	for _, changed := range []bool{false, true} {
		t.Run(fmt.Sprint(changed), func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			ready, release, stop := make(chan struct{}, 1), make(chan struct{}), make(chan struct{})
			var mu sync.Mutex
			var sockets []net.Conn
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					socket, err := listener.Accept()
					if err != nil {
						return
					}
					mu.Lock()
					sockets = append(sockets, socket)
					mu.Unlock()
					wg.Add(1)
					go func() {
						defer wg.Done()
						defer socket.Close()
						fmt.Fprint(socket, "220 Fixture FTP\r\n")
						scanner := bufio.NewScanner(socket)
						for scanner.Scan() {
							command, argument, _ := strings.Cut(scanner.Text(), " ")
							switch command {
							case "USER":
								fmt.Fprint(socket, "331 Password required\r\n")
							case "PASS":
								if argument != "ftp-fixture-password" {
									fmt.Fprint(socket, "530 Denied\r\n")
									continue
								}
								select {
								case ready <- struct{}{}:
								default:
								}
								select {
								case <-release:
								case <-stop:
									return
								}
								fmt.Fprint(socket, "230 Logged in\r\n")
							case "FEAT":
								fmt.Fprint(socket, "211 End\r\n")
							case "TYPE", "OPTS":
								fmt.Fprint(socket, "200 OK\r\n")
							case "PWD":
								fmt.Fprint(socket, "257 \"/\"\r\n")
							case "QUIT":
								fmt.Fprint(socket, "221 Bye\r\n")
								return
							default:
								fmt.Fprint(socket, "502 Not implemented by login fixture\r\n")
							}
						}
					}()
				}
			}()
			t.Cleanup(func() {
				close(stop)
				listener.Close()
				mu.Lock()
				for _, socket := range sockets {
					socket.Close()
				}
				mu.Unlock()
				wg.Wait()
			})
			profile := core.NewProfile()
			profile.Kind = "ftp"
			profile.Host = "127.0.0.1"
			profile.Username = "tester"
			_, port, _ := net.SplitHostPort(listener.Addr().String())
			profile.Port, _ = strconv.Atoi(port)
			cfg := core.DefaultConfig()
			cfg.Profiles = []core.Profile{profile}
			store := &core.Store{Dir: t.TempDir()}
			if err = store.Save(cfg); err != nil {
				t.Fatal(err)
			}
			a := newApp(store, cfg)
			t.Cleanup(func() {
				for _, tab := range a.tabs {
					a.walk(tab.root, a.dispose)
				}
			})
			a.open(profile, core.Credentials{Password: "ftp-fixture-password", SavePassword: true}, 0)
			readySeen := false
			waitOperator(t, a, func() bool {
				select {
				case <-ready:
					readySeen = true
				default:
				}
				return readySeen
			})
			if changed {
				a.config.Profiles[0].Host = "changed.fixture.invalid"
			}
			close(release)
			waitOperator(t, a, func() bool { return len(a.tabs) == 1 && a.tabs[0].active.ftp != nil })
			got, err := a.vault.GetFor(profile, profile.Identity())
			if err != nil {
				t.Fatal(err)
			}
			if changed {
				if got != "" || !strings.Contains(a.status, "未保存") {
					t.Fatal("late FTP completion saved stale credentials")
				}
			} else {
				if got != "ftp-fixture-password" {
					t.Fatal("successful FTP login password not saved")
				}
			}
			// Reconnection uses session memory even with no saved password.
			if err := a.vault.Delete([]string{profile.ID}); err != nil {
				t.Fatal(err)
			}
			p := a.tabs[0].active
			waitOperator(t, a, func() bool { return p.files != nil && !p.files.busy })
			originalFiles, originalSession := p.files, p.ftp
			originalFiles.dir = "/retained-directory"
			originalFiles.filter = "retained-filter"
			a.reconnectActive()
			reauthenticated := false
			waitOperator(t, a, func() bool {
				select {
				case <-ready:
					reauthenticated = true
				default:
				}
				return reauthenticated && !p.files.busy
			})
			if a.authOpen || a.activePane() != p || p.ftp != originalSession || p.files == originalFiles || p.files.dir != "/retained-directory" || p.files.filter != "retained-filter" {
				t.Fatal("FTP reconnect lost session state")
			}
			if a.vault.HasStoredCredential(profile.ID) {
				t.Fatal("reconnect unexpectedly persisted password")
			}
			a.duplicateFileSession()
			a.dispose(p)
			waitOperator(t, a, func() bool {
				return len(a.tabs) == 2 && a.tabs[1].active.ftp != nil && a.tabs[1].active.files != nil && !a.tabs[1].active.files.busy
			})
			copied := a.tabs[1].active
			if copied.ftp == originalSession || a.authOpen || copied.profile.InitialDirectory != "/retained-directory" {
				t.Fatal("FTP copy did not retain independent login/path")
			}
			select {
			case <-ready:
			default:
				t.Fatal("FTP clone did not log in using session credential")
			}
			if a.vault.HasStoredCredential(profile.ID) {
				t.Fatal("copy persisted password without consent")
			}

		})
	}
}
