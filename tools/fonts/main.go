// Command fonts puts the MiSans faces the window draws with into
// internal/fonts/misans for go:embed, cut down to the characters the app
// shows (see subset.go). The faces come from Xiaomi's official package and
// must match the pinned SHA-256 sums.
//
// The font files stay out of the repository: the MiSans licence lets an app
// embed them but forbids distributing them on their own. The package is
// large, so only the entries needed are fetched, with range requests, into
// the user's cache directory, where the next run finds them.
//
//	go run ./tools/fonts
//
// Each run cuts the faces again for the text of the code as it is; it
// needs HarfBuzz's hb-subset. What it writes is ignored by Git.
package main

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const source = "https://hyperos.mi.com/font-download/MiSans.zip"

// faces are the entries of the package and their SHA-256 sums.
var faces = []struct{ name, sum string }{
	{"MiSans-Regular.ttf", "9c120f0a849bc0aa5048daae2a3c0f6eecd828b5b33fce682a9622833f5feea6"},
	{"MiSans-Medium.ttf", "b03e98374e971594b0b7a9706d0704241f76e1b88556cdda79c5039ef8a638d1"},
	{"MiSans-Semibold.ttf", "77c23f31ae124867778344970155a0c8d34a89897dedaab81aeee82ff00a4ce6"},
}

var client = &http.Client{Timeout: 5 * time.Minute}

func main() {
	cache, err := os.UserCacheDir()
	if err == nil {
		err = run(root(), filepath.Join(cache, "power-meter", "MiSans"))
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "fonts:", err)
		os.Exit(1)
	}
}

// run keeps the whole faces in cache, fetching those missing, and writes
// them, cut for the text of the code under root, into
// root/internal/fonts/misans.
func run(root, cache string) error {
	var missing []int
	for i, f := range faces {
		if stored(filepath.Join(cache, f.name)) != f.sum {
			missing = append(missing, i)
		}
	}
	if len(missing) > 0 {
		if err := fetch(cache, missing); err != nil {
			return err
		}
	}
	text, err := characters(root)
	if err != nil {
		return err
	}
	dir := filepath.Join(root, "internal", "fonts")
	chars := filepath.Join(dir, charactersFile)
	if err := writeFile(chars, []byte(text)); err != nil {
		return err
	}
	for _, f := range faces {
		data, err := subset(filepath.Join(cache, f.name), chars)
		if err != nil {
			return fmt.Errorf("%s: %w", f.name, err)
		}
		if err := writeFile(filepath.Join(dir, "misans", f.name), data); err != nil {
			return err
		}
	}
	return nil
}

func fetch(dir string, missing []int) error {
	pkg, err := openRemote(source)
	if err != nil {
		return err
	}
	for _, i := range missing {
		f := faces[i]
		data, err := pkg.extract("ttf/" + f.name)
		if err != nil {
			return fmt.Errorf("%s: %w", f.name, err)
		}
		if got := sum(data); got != f.sum {
			return fmt.Errorf("%s has SHA-256 %s, want %s: the package changed", f.name, got, f.sum)
		}
		if err := writeFile(filepath.Join(dir, f.name), data); err != nil {
			return err
		}
		fmt.Println("fetched", f.name)
	}
	return nil
}

// stored is the SHA-256 of a face's file, or "" without one.
func stored(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}

func sum(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// writeFile replaces path with data, unless it holds data already.
func writeFile(path string, data []byte) error {
	if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, data) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// remote is the package on the server, read with range requests.
type remote struct {
	url  string
	size int64
	zip  *zip.Reader
}

// openRemote reads the package's directory. The size comes from the
// Content-Range of a one-byte range request, not from a HEAD request: some
// of the CDN's edges answer HEAD without a length.
func openRemote(url string) (*remote, error) {
	_, header, err := fetchRange(url, "bytes=-1", 16)
	if err != nil {
		return nil, err
	}
	cr := header.Get("Content-Range")
	i := strings.LastIndexByte(cr, '/')
	size := int64(-1)
	if i >= 0 {
		size, _ = strconv.ParseInt(cr[i+1:], 10, 64)
	}
	if size <= 0 {
		return nil, fmt.Errorf("GET %s with a range: no size in Content-Range %q", url, cr)
	}
	r := &remote{url: url, size: size}
	r.zip, err = zip.NewReader(r, r.size)
	return r, err
}

// ReadAt reads one range; zip.NewReader only reads the directory with it.
func (r *remote) ReadAt(p []byte, off int64) (int, error) {
	if off >= r.size {
		return 0, io.EOF
	}
	end := min(off+int64(len(p)), r.size)
	data, err := r.get(off, end-off)
	n := copy(p, data)
	if err == nil && n < len(p) {
		err = io.EOF
	}
	return n, err
}

func (r *remote) get(off, n int64) ([]byte, error) {
	data, _, err := fetchRange(r.url, "bytes="+strconv.FormatInt(off, 10)+"-"+strconv.FormatInt(off+n-1, 10), n)
	return data, err
}

// attempts is how many times a range request is tried: the CDN now and then
// resets a stream (HTTP/2 INTERNAL_ERROR) or answers 5xx.
const attempts = 3

// retryWait is the wait before the second attempt, doubled for the third.
var retryWait = time.Second

// fetchRange gets at most limit bytes of url's range spec (a Range header
// value) and the answer's header. It retries network errors, including a
// body cut off midway, and 5xx answers; any other answer but 206 fails at
// once.
func fetchRange(url, spec string, limit int64) ([]byte, http.Header, error) {
	var err error
	for i := range attempts {
		if i > 0 {
			time.Sleep(retryWait << (i - 1))
		}
		var data []byte
		var header http.Header
		var transient bool
		data, header, transient, err = fetchOnce(url, spec, limit)
		if err == nil || !transient {
			return data, header, err
		}
	}
	return nil, nil, fmt.Errorf("after %d attempts: %w", attempts, err)
}

func fetchOnce(url, spec string, limit int64) (data []byte, header http.Header, transient bool, err error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, nil, false, err
	}
	req.Header.Set("Range", spec)
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, true, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent {
		return nil, nil, resp.StatusCode >= 500,
			fmt.Errorf("GET %s with range %s: %s; the server must answer range requests", url, spec, resp.Status)
	}
	if data, err = io.ReadAll(io.LimitReader(resp.Body, limit)); err != nil {
		return nil, nil, true, err
	}
	return data, resp.Header, false, nil
}

// extract fetches one entry, whose name ends with suffix, in one request.
func (r *remote) extract(suffix string) ([]byte, error) {
	for _, f := range r.zip.File {
		if len(f.Name) < len(suffix) || f.Name[len(f.Name)-len(suffix):] != suffix {
			continue
		}
		off, err := f.DataOffset()
		if err != nil {
			return nil, err
		}
		raw, err := r.get(off, int64(f.CompressedSize64))
		if err != nil {
			return nil, err
		}
		switch f.Method {
		case zip.Store:
			return raw, nil
		case zip.Deflate:
			return io.ReadAll(flate.NewReader(bytes.NewReader(raw)))
		}
		return nil, fmt.Errorf("compression method %d", f.Method)
	}
	return nil, errors.New("not in the package")
}

func root() string {
	dir, _ := os.Getwd()
	for d := dir; ; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d
		}
		if filepath.Dir(d) == d {
			return dir
		}
	}
}
