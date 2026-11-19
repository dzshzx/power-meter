package main

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// A package served the way the CDN's edges do: ranges answered, HEAD
// without a length.
func TestRemoteWithoutHeadLength(t *testing.T) {
	var pkg bytes.Buffer
	w := zip.NewWriter(&pkg)
	files := map[string]string{
		"MiSans/ttf/MiSans-Regular.ttf": strings.Repeat("regular ", 4000),
		"MiSans/ttf/MiSans-Medium.ttf":  "medium",
	}
	for name, body := range files {
		method := zip.Deflate
		if name == "MiSans/ttf/MiSans-Medium.ttf" {
			method = zip.Store
		}
		f, err := w.CreateHeader(&zip.FileHeader{Name: name, Method: method})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	ranges := 0
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodHead {
			rw.WriteHeader(http.StatusOK)
			return
		}
		if req.Header.Get("Range") != "" {
			ranges++
		}
		http.ServeContent(rw, req, "MiSans.zip", time.Time{}, bytes.NewReader(pkg.Bytes()))
	}))
	defer srv.Close()

	r, err := openRemote(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if r.size != int64(pkg.Len()) {
		t.Fatalf("size %d, want %d", r.size, pkg.Len())
	}
	for _, name := range []string{"MiSans-Regular.ttf", "MiSans-Medium.ttf"} {
		got, err := r.extract(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if want := files["MiSans/ttf/"+name]; string(got) != want {
			t.Fatalf("%s: got %d bytes, want %d", name, len(got), len(want))
		}
	}
	if ranges == 0 {
		t.Fatal("no range requests")
	}
}

// A server that ignores ranges is refused instead of downloading the whole
// package.
func TestRemoteNeedsRanges(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		_, _ = rw.Write([]byte("whole file"))
	}))
	defer srv.Close()
	if _, err := openRemote(srv.URL); err == nil || !strings.Contains(err.Error(), "range") {
		t.Fatalf("openRemote: %v", err)
	}
}
