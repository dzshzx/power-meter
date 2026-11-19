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

// A 5xx answer, as the CDN gives now and then, is tried again; a 4xx is not.
func TestRetries(t *testing.T) {
	retryWait = 0
	defer func() { retryWait = time.Second }()
	for _, c := range []struct {
		status   int
		requests int
		ok       bool
	}{
		{http.StatusServiceUnavailable, 2, true},
		{http.StatusNotFound, 1, false},
	} {
		requests := 0
		srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			requests++
			if requests == 1 {
				rw.WriteHeader(c.status)
				return
			}
			http.ServeContent(rw, req, "x", time.Time{}, strings.NewReader("0123456789"))
		}))
		data, _, err := fetchRange(srv.URL, "bytes=2-4", 3)
		srv.Close()
		if requests != c.requests || (err == nil) != c.ok || (c.ok && string(data) != "234") {
			t.Errorf("first answer %d: %d requests, data %q, err %v", c.status, requests, data, err)
		}
	}
}

// Every attempt failing ends in an error that says so.
func TestRetriesGiveUp(t *testing.T) {
	retryWait = 0
	defer func() { retryWait = time.Second }()
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		requests++
		rw.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()
	if _, _, err := fetchRange(srv.URL, "bytes=0-1", 2); err == nil || requests != attempts || !strings.Contains(err.Error(), "attempts") {
		t.Fatalf("%d requests, err %v", requests, err)
	}
}
