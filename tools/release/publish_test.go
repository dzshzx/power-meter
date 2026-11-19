package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseVersion(t *testing.T) {
	tags := []string{"v1.5.5", "v1.6.1", "v1.6.0", "v1.10", "v2.0.0-rc1", "latest"}
	none := func() ([]string, error) {
		t.Error("published Releases are read only for a tagged commit")
		return nil, nil
	}
	cases := []struct {
		name      string
		tags      []string
		headTags  []string
		published []string
		bump      string
		dryRun    bool
		want      string
		err       string
	}{
		{"patch bumps the highest release tag", tags, nil, nil, "patch", false, "1.6.2", ""},
		{"minor resets the patch", tags, nil, nil, "minor", false, "1.7.0", ""},
		{"major resets the rest", tags, nil, nil, "major", false, "2.0.0", ""},
		{"versions compare as numbers", []string{"v1.9.0", "v1.10.0"}, nil, nil, "patch", false, "1.10.1", ""},
		{"a tag without a published Release resumes", tags, []string{"v1.6.2"}, []string{"v1.6.1"}, "minor", false, "1.6.2", ""},
		{"a released commit refuses a release", tags, []string{"v1.6.1"}, []string{"v1.6.1"}, "patch", false, "", "已发过版"},
		{"a dry run on a released commit previews the bump", tags, []string{"v1.6.1"}, []string{"v1.6.1"}, "patch", true, "1.6.2", ""},
		{"no release tag has no baseline", []string{"v1.0", "x"}, nil, nil, "patch", false, "", "baseline"},
		{"an unknown bump fails", tags, nil, nil, "pico", false, "", "unknown bump"},
		{"a field beyond 16 bits fails", []string{"v1.2.65535"}, nil, nil, "patch", false, "", "Windows file version"},
	}
	for _, c := range cases {
		published := none
		if c.headTags != nil {
			published = func() ([]string, error) { return c.published, nil }
		}
		got, _, err := releaseVersion(c.tags, c.headTags, published, c.bump, c.dryRun)
		if got != c.want || (c.err == "") != (err == nil) || (err != nil && !strings.Contains(err.Error(), c.err)) {
			t.Errorf("%s: %q, %v", c.name, got, err)
		}
	}
	failing := func() ([]string, error) { return nil, errors.New("offline") }
	if _, _, err := releaseVersion(tags, []string{"v1.6.1"}, failing, "patch", true); err == nil {
		t.Error("an unreadable Release list fails")
	}
}

// fakeGH answers gh calls by the first answer whose key the joined
// arguments start with, and records every call.
func fakeGH(t *testing.T, answers map[string]string) *[]string {
	t.Helper()
	var calls []string
	saved := gh
	t.Cleanup(func() { gh = saved })
	gh = func(args ...string) (string, error) {
		call := strings.Join(args, " ")
		calls = append(calls, call)
		for key, out := range answers {
			if strings.HasPrefix(call, key) {
				return out, nil
			}
		}
		return "", nil
	}
	return &calls
}

func TestTagCommit(t *testing.T) {
	const sha, other = "c0ffee", "decade"
	refs := "api repos/o/r/git/matching-refs/tags/v1.2.3"
	cases := []struct {
		name    string
		answers map[string]string
		created bool
		err     bool
	}{
		{"an absent tag is created", map[string]string{refs: "refs/tags/v1.2.30\tcommit\t" + other, "api repos/o/r/git/tags -f": "7a9"}, true, false},
		{"a lightweight tag on the commit is kept", map[string]string{refs: "refs/tags/v1.2.3\tcommit\t" + sha}, false, false},
		{"an annotated tag is followed to its commit", map[string]string{refs: "refs/tags/v1.2.3\ttag\t7a9", "api repos/o/r/git/tags/7a9": sha}, false, false},
		{"a tag on another commit fails", map[string]string{refs: "refs/tags/v1.2.3\tcommit\t" + other}, false, true},
	}
	for _, c := range cases {
		calls := fakeGH(t, c.answers)
		err := tagCommit("o/r", "v1.2.3", sha)
		created := strings.Contains(strings.Join(*calls, "\n"), "git/refs -f ref=refs/tags/v1.2.3 -f sha=7a9")
		if (err != nil) != c.err || created != c.created {
			t.Errorf("%s: err %v, created %t, calls %q", c.name, err, created, *calls)
		}
	}
}

func TestDeleteDrafts(t *testing.T) {
	calls := fakeGH(t, map[string]string{"api repos/o/r/releases?": "11\tv1.2.3\n12\tv1.2.4\n13\tv1.2.3"})
	if err := deleteDrafts("o/r", "v1.2.3"); err != nil {
		t.Fatal(err)
	}
	var deleted []string
	for _, call := range *calls {
		if id, ok := strings.CutPrefix(call, "api -X DELETE repos/o/r/releases/"); ok {
			deleted = append(deleted, id)
		}
	}
	if strings.Join(deleted, ",") != "11,13" {
		t.Errorf("only drafts of the tag are deleted: %v", deleted)
	}
}

func TestReleaseSummary(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.exe"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	summary, files, err := releaseSummary(dir, "v1.2.3", "c0ffee", true)
	if err != nil {
		t.Fatal(err)
	}
	const sumA = "ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb"
	for _, want := range []string{"## Dry run: would release v1.2.3\n", "- source_commit: `c0ffee`\n", "| a.exe | `" + sumA + "` |\n"} {
		if !strings.Contains(summary, want) {
			t.Errorf("summary lacks %q:\n%s", want, summary)
		}
	}
	if len(files) != 1 || files[0] != filepath.Join(dir, "a.exe") {
		t.Errorf("files %v", files)
	}
}
