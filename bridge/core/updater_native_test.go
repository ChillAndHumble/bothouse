package core

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestBothouseUpdateNativeAndLegacyFallback(t *testing.T) {
	for _, goos := range []string{"linux", "windows", "darwin"} {
		for _, scenario := range []string{"native", "legacy", "tampered", "missing-sums", "unlisted"} {
			t.Run(goos+"/"+scenario, func(t *testing.T) {
				native := "bothouse-" + goos + "-arm64"
				legacy := "cc-connect-v0.1.1-" + goos + "-arm64.tar.gz"
				if goos == "windows" {
					native += ".exe"
					legacy = "cc-connect-v0.1.1-windows-arm64.zip"
				}
				if goos == "darwin" {
					native += ".dmg"
				}
				names := []string{native, legacy}
				var requests []string
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					name := strings.TrimPrefix(r.URL.Path, "/v0.1.1/")
					requests = append(requests, name)
					switch name {
					case names[0]:
						if scenario == "legacy" {
							http.NotFound(w, r)
							return
						}
						fmt.Fprint(w, "native executable")
					case names[1]:
						fmt.Fprint(w, "legacy archive")
					case "SHA256SUMS":
						if scenario == "missing-sums" {
							http.NotFound(w, r)
							return
						}
						data := "native executable"
						if scenario == "tampered" {
							data = "unmodified executable"
						}
						sumName := names[0]
						if scenario == "unlisted" {
							sumName = "other-file"
						}
						fmt.Fprintf(w, "%x  %s\n", sha256.Sum256([]byte(data)), sumName)
					case "checksums.txt":
						fmt.Fprintf(w, "%x  %s\n", sha256.Sum256([]byte("legacy archive")), names[1])
					default:
						http.NotFound(w, r)
					}
				}))
				defer server.Close()
				data, name, err := DownloadReleaseAsset([]string{server.URL}, "v0.1.1", goos, "arm64")
				switch scenario {
				case "native":
					if err != nil || name != names[0] || string(data) != "native executable" {
						t.Fatalf("%q %q %v", data, name, err)
					}
					if !reflect.DeepEqual(requests, []string{names[0], "SHA256SUMS"}) {
						t.Fatalf("unexpected requests: %v", requests)
					}
				case "legacy":
					if err != nil || name != names[1] || string(data) != "legacy archive" {
						t.Fatalf("%q %q %v", data, name, err)
					}
					if !reflect.DeepEqual(requests, []string{names[0], names[1], "checksums.txt"}) {
						t.Fatalf("unexpected requests: %v", requests)
					}
				default:
					if err == nil {
						t.Fatal("unverified executable accepted")
					}
					if !reflect.DeepEqual(requests, []string{names[0], "SHA256SUMS"}) {
						t.Fatalf("checksum failure triggered fallback: %v", requests)
					}
				}
			})
		}
	}
}

func TestExtractReleaseBinaryNativeAndLegacy(t *testing.T) {
	payload := []byte("executable bytes")
	var tgz, zipped bytes.Buffer
	gz := gzip.NewWriter(&tgz)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: "cc-connect", Mode: 0755, Size: int64(len(payload))}); err != nil {
		t.Fatal(err)
	}
	tw.Write(payload)
	tw.Close()
	gz.Close()
	zw := zip.NewWriter(&zipped)
	entry, err := zw.Create("cc-connect.exe")
	if err != nil {
		t.Fatal(err)
	}
	entry.Write(payload)
	zw.Close()
	for _, tt := range []struct {
		name string
		data []byte
	}{
		{"bothouse-linux-arm64", payload}, {"bothouse-windows-arm64.exe", payload},
		{"cc-connect-v0.1.1-darwin-arm64.tar.gz", tgz.Bytes()}, {"cc-connect-v0.1.1-windows-arm64.zip", zipped.Bytes()},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractReleaseBinary(tt.data, tt.name)
			if err != nil || !bytes.Equal(got, payload) {
				t.Fatalf("%q %v", got, err)
			}
		})
	}
}
