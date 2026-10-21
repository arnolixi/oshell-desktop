package core

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func davCollectionXML(href, status, resource string) string {
	var escaped bytes.Buffer
	xml.EscapeText(&escaped, []byte(href))
	return fmt.Sprintf(`<d:multistatus xmlns:d="DAV:"><d:response><d:href>%s</d:href><d:propstat><d:prop><d:resourcetype>%s</d:resourcetype></d:prop><d:status>HTTP/1.1 %s</d:status></d:propstat></d:response></d:multistatus>`, escaped.String(), resource, status)
}
func writeDAVCollection(w http.ResponseWriter, href string) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(207)
	fmt.Fprint(w, davCollectionXML(href, "200 OK", `<d:collection/>`))
}

func TestWebDAVAutomaticDirectorySelection(t *testing.T) {
	for _, scenario := range []string{"create", "existing", "explicit", "legacy", "race", "file", "denied", "wrong-href", "bad-xml", "mk-denied", "missing-parent", "false-405", "false-201", "legacy-invalid"} {
		t.Run(scenario, func(t *testing.T) {
			var requests []string
			created := scenario == "existing" || scenario == "explicit"
			payload := []byte(`{"format":"OShell.shared-vault"}`)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests = append(requests, r.Method+" "+r.URL.EscapedPath())
				user, password, ok := r.BasicAuth()
				if !ok || user != "fixture" || password != "secret" {
					t.Error("missing credentials")
				}
				if r.Method == "GET" {
					if r.URL.Path == "/dav/oshell-vault.json" && (scenario == "legacy" || scenario == "legacy-invalid") {
						w.Header().Set("ETag", `"legacy"`)
						if scenario == "legacy-invalid" {
							fmt.Fprint(w, "not a shared vault")
						} else {
							w.Write(payload)
						}
						return
					}
					w.WriteHeader(404)
					return
				}
				if r.URL.Path != "/dav/oshell/" {
					t.Errorf("unexpected path %s", r.URL.Path)
				}
				if r.Method == "MKCOL" {
					switch scenario {
					case "mk-denied":
						w.WriteHeader(403)
					case "missing-parent":
						w.WriteHeader(409)
					case "false-405":
						w.WriteHeader(405)
					case "false-201":
						w.WriteHeader(201)
					case "race":
						created = true
						w.WriteHeader(405)
					default:
						created = true
						w.WriteHeader(201)
					}
					return
				}
				if r.Method != "PROPFIND" || r.Header.Get("Depth") != "0" {
					t.Error("probe wrote configuration or used unexpected request")
					w.WriteHeader(500)
					return
				}
				switch scenario {
				case "denied":
					w.WriteHeader(403)
				case "file":
					w.WriteHeader(207)
					fmt.Fprint(w, davCollectionXML(r.URL.Path, "200 OK", ""))
				case "wrong-href":
					writeDAVCollection(w, "/unrelated/")
				case "bad-xml":
					w.WriteHeader(207)
					fmt.Fprint(w, "<broken")
				default:
					if created {
						writeDAVCollection(w, r.URL.Path)
					} else {
						w.WriteHeader(404)
					}
				}
			}))
			defer server.Close()
			address := server.URL + "/dav"
			if scenario == "explicit" {
				address += "/oshell"
			}
			client, err := NewWebDAVTransport(address, "fixture", "secret", false)
			if err != nil {
				t.Fatal(err)
			}
			err = client.Probe(context.Background())
			success := scenario == "create" || scenario == "existing" || scenario == "explicit" || scenario == "legacy" || scenario == "race"
			if (err == nil) != success {
				t.Fatalf("unexpected result: %v; %v", err, requests)
			}
			if !success {
				return
			}
			target, err := client.SyncDirectory(context.Background())
			want := "/dav/oshell/"
			if scenario == "legacy" {
				want = "/dav/"
			}
			if err != nil || target.Path != want {
				t.Fatal("wrong destination", target, err)
			}
			count := len(requests)
			target.Path = "/caller-mutation/"
			if _, err = client.Fetch(context.Background()); err != nil || len(requests) != count+1 || !strings.HasSuffix(requests[count], want+"oshell-vault.json") {
				t.Fatal("resolution was not cached or caller modified target", err, requests)
			}
			if scenario == "legacy" && strings.Contains(strings.Join(requests, "\n"), "MKCOL") {
				t.Fatal("legacy endpoint migrated")
			}
		})
	}
}

func TestWebDAVCollectionResponseValidation(t *testing.T) {
	target, _ := url.Parse("https://example.invalid/dav/a%20b/oshell/")
	good := davCollectionXML(target.Path, "200 OK", `<d:collection/>`)
	for _, tc := range []struct {
		name, body string
		valid      bool
	}{
		{"valid", good, true},
		{"absolute", davCollectionXML("https://example.invalid:443/dav/a%20b/oshell/", "200 OK", `<d:collection/>`), true},
		{"relative", davCollectionXML("./", "200 OK", `<d:collection/>`), true},
		{"denied-property", davCollectionXML(target.Path, "403 Forbidden", `<d:collection/>`), false},
		{"foreign-host", davCollectionXML("https://other.invalid"+target.Path, "200 OK", `<d:collection/>`), false},
		{"wrong-port", davCollectionXML("https://example.invalid:444"+target.Path, "200 OK", `<d:collection/>`), false},
		{"file", davCollectionXML(target.Path, "200 OK", ""), false},
		{"wrong-namespace", strings.ReplaceAll(good, "DAV:", "not-DAV:"), false},
		{"nested-collection", davCollectionXML(target.Path, "200 OK", `<d:other><d:collection/></d:other>`), false},
		{"duplicate-response", strings.Replace(good, "</d:multistatus>", strings.TrimPrefix(good, `<d:multistatus xmlns:d="DAV:">`), 1), false},
		{"trailing-root", good + good, false},
		{"invalid-xml", good[:len(good)-3], false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := validDAVCollection([]byte(tc.body), target); got != tc.valid {
				t.Fatal(got)
			}
		})
	}
}

func TestWebDAVDirectoryResolutionHonorsCancellation(t *testing.T) {
	client, err := NewWebDAVTransport("https://example.invalid/dav/", "", "", false)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.SyncDirectory(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
