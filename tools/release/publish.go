package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

var releaseTag = regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)$`)

// semver is a release version: major, minor, patch.
type semver [3]int

func (v semver) String() string { return fmt.Sprintf("%d.%d.%d", v[0], v[1], v[2]) }

func parseTag(tag string) (semver, bool) {
	m := releaseTag.FindStringSubmatch(tag)
	if m == nil {
		return semver{}, false
	}
	var v semver
	for i := range v {
		n, err := strconv.Atoi(m[i+1])
		if err != nil {
			return semver{}, false
		}
		v[i] = n
	}
	return v, true
}

// highest is the highest vX.Y.Z tag among tags; others are ignored.
func highest(tags []string) (semver, string, bool) {
	var best semver
	name := ""
	for _, tag := range tags {
		if v, ok := parseTag(tag); ok && (name == "" || slices.Compare(v[:], best[:]) > 0) {
			best, name = v, tag
		}
	}
	return best, name, name != ""
}

// releaseVersion is the version a release run builds. Released means a
// vX.Y.Z tag on the commit plus a published Release of the same name; a
// tag without a published Release is an unfinished run, which keeps its
// version. Otherwise the highest tag takes the bump. published lists the
// published Releases and is only called when the commit carries a tag.
// note says how the version was chosen.
func releaseVersion(tags, headTags []string, published func() ([]string, error), bump string, dryRun bool) (version, note string, err error) {
	base, _, ok := highest(tags)
	if !ok {
		return "", "", errors.New("no vX.Y.Z tag: the version baseline is unknown")
	}
	if head, headTag, onHead := highest(headTags); onHead {
		names, err := published()
		if err != nil {
			return "", "", err
		}
		if !slices.Contains(names, headTag) {
			return fits(head, "resuming "+headTag+": the tag is on this commit but its Release is not published")
		}
		if !dryRun {
			return "", "", fmt.Errorf("这个提交已发过版：it is %s and its Release is published", headTag)
		}
		note = headTag + " is already released from this commit; the dry run previews the next bump. "
	}
	next := base
	switch bump {
	case "patch":
		next[2]++
	case "minor":
		next = semver{base[0], base[1] + 1, 0}
	case "major":
		next = semver{base[0] + 1, 0, 0}
	default:
		return "", "", fmt.Errorf("unknown bump %q", bump)
	}
	return fits(next, note+"baseline v"+base.String()+", bump "+bump)
}

// fits accepts a version whose fields each fit the 16 bits of a Windows
// file version field.
func fits(v semver, note string) (string, string, error) {
	for _, n := range v {
		if n > 65535 {
			return "", "", fmt.Errorf("version %s does not fit a Windows file version", v)
		}
	}
	return v.String(), note, nil
}

// runNextVersion prints the version=X.Y.Z and tag=vX.Y.Z outputs of the
// release run on this commit, in GitHub Actions.
func runNextVersion(args []string) error {
	fs := flag.NewFlagSet("next-version", flag.ContinueOnError)
	bump := fs.String("bump", "", "patch, minor or major")
	dryRun := fs.Bool("dry-run", false, "a dry run, which may run on any branch")
	branch := fs.String("branch", "", "the default branch, the only one a release runs on")
	if err := parse(fs, args, "bump", "branch"); err != nil {
		return err
	}
	env, err := actionsEnv("GITHUB_REF", "GITHUB_SHA", "GITHUB_REPOSITORY")
	if err != nil {
		return err
	}
	if ref := env["GITHUB_REF"]; !*dryRun && ref != "refs/heads/"+*branch {
		return fmt.Errorf("a release (dry_run=false) must run on %s; this run is on %s", *branch, ref)
	}
	tags, err := commandLines("git", "tag", "--list", "v*")
	if err != nil {
		return err
	}
	headTags, err := commandLines("git", "tag", "--points-at", env["GITHUB_SHA"])
	if err != nil {
		return err
	}
	published := func() ([]string, error) {
		out, err := gh("api", "repos/"+env["GITHUB_REPOSITORY"]+"/releases?per_page=100", "--paginate",
			"--jq", ".[] | select(.draft | not) | .tag_name")
		return strings.Fields(out), err
	}
	version, note, err := releaseVersion(tags, headTags, published, *bump, *dryRun)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "release v%s (dry_run=%t): %s\n", version, *dryRun, note)
	fmt.Printf("version=%s\ntag=v%s\n", version, version)
	return nil
}

// runPublish summarizes the assets package wrote and, unless it is a dry
// run, publishes them as the Release of this commit, in GitHub Actions.
// Order: drop stale drafts of the tag, create a draft with every asset, tag
// the commit through the API, then publish. A failed run is recovered by
// running the release again: next-version resumes a tagged commit whose
// Release is not published.
func runPublish(args []string) error {
	fs := flag.NewFlagSet("publish", flag.ContinueOnError)
	version := fs.String("version", "", "release version X.Y.Z")
	assets := fs.String("assets", "", "the directory package wrote")
	dryRun := fs.Bool("dry-run", false, "only summarize")
	if err := parse(fs, args, "version", "assets"); err != nil {
		return err
	}
	env, err := actionsEnv("GITHUB_SHA", "GITHUB_REPOSITORY")
	if err != nil {
		return err
	}
	if err := checkAssets(*assets, *version); err != nil {
		return err
	}
	tag, sha := "v"+*version, env["GITHUB_SHA"]
	summary, files, err := releaseSummary(*assets, tag, sha, *dryRun)
	if err != nil {
		return err
	}
	fmt.Print(summary)
	if path := os.Getenv("GITHUB_STEP_SUMMARY"); path != "" {
		if err := appendFile(path, summary); err != nil {
			return err
		}
	}
	if *dryRun {
		return nil
	}
	return publishRelease(env["GITHUB_REPOSITORY"], tag, sha, files)
}

// releaseSummary is the run summary: the version, the source commit and
// every asset with its SHA-256. files are the assets' paths.
func releaseSummary(dir, tag, sha string, dryRun bool) (summary string, files []string, err error) {
	heading := "Releasing " + tag
	if dryRun {
		heading = "Dry run: would release " + tag
	}
	lines := []string{"## " + heading, "", "- version: `" + tag + "`", "- source_commit: `" + sha + "`", "",
		"| asset | sha256 |", "| --- | --- |"}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", nil, err
	}
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		sum, err := fileHash(path)
		if err != nil {
			return "", nil, err
		}
		lines = append(lines, "| "+e.Name()+" | `"+sum+"` |")
		files = append(files, path)
	}
	return strings.Join(lines, "\n") + "\n", files, nil
}

func publishRelease(repo, tag, sha string, files []string) error {
	if err := deleteDrafts(repo, tag); err != nil {
		return err
	}
	create := append([]string{"release", "create", tag}, files...)
	create = append(create, "--repo", repo, "--draft", "--target", sha, "--title", tag,
		"--notes", "source_commit: "+sha, "--generate-notes")
	if _, err := gh(create...); err != nil {
		return err
	}
	if err := tagCommit(repo, tag, sha); err != nil {
		return err
	}
	_, err := gh("release", "edit", tag, "--repo", repo, "--verify-tag", "--draft=false")
	return err
}

// deleteDrafts deletes the draft Releases of tag a failed run left.
func deleteDrafts(repo, tag string) error {
	out, err := gh("api", "repos/"+repo+"/releases?per_page=100", "--paginate",
		"--jq", ".[] | select(.draft) | [.id, .tag_name] | @tsv")
	if err != nil {
		return err
	}
	for _, line := range strings.Split(out, "\n") {
		id, name, ok := strings.Cut(strings.TrimSpace(line), "\t")
		if !ok || name != tag {
			continue
		}
		if _, err := gh("api", "-X", "DELETE", "repos/"+repo+"/releases/"+id); err != nil {
			return err
		}
	}
	return nil
}

// tagCommit creates the annotated tag on sha through the API, or accepts
// the tag when it already points at sha.
func tagCommit(repo, tag, sha string) error {
	api := "repos/" + repo
	out, err := gh("api", api+"/git/matching-refs/tags/"+tag, "--jq", ".[] | [.ref, .object.type, .object.sha] | @tsv")
	if err != nil {
		return err
	}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Split(strings.TrimSpace(line), "\t")
		if len(fields) != 3 || fields[0] != "refs/tags/"+tag {
			continue
		}
		commit := fields[2]
		if fields[1] == "tag" {
			if commit, err = gh("api", api+"/git/tags/"+fields[2], "--jq", ".object.sha"); err != nil {
				return err
			}
		}
		if commit != sha {
			return fmt.Errorf("tag %s already points at %s, not %s", tag, commit, sha)
		}
		fmt.Printf("tag %s already points at this commit\n", tag)
		return nil
	}
	object, err := gh("api", api+"/git/tags", "-f", "tag="+tag, "-f", "message=Release "+tag,
		"-f", "object="+sha, "-f", "type=commit", "--jq", ".sha")
	if err != nil {
		return err
	}
	_, err = gh("api", api+"/git/refs", "-f", "ref=refs/tags/"+tag, "-f", "sha="+object, "--silent")
	return err
}

// gh runs the GitHub CLI and returns its trimmed output; tests replace it.
var gh = func(args ...string) (string, error) {
	out, err := exec.Command("gh", args...).Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			err = fmt.Errorf("%w: %s", err, bytes.TrimSpace(exit.Stderr))
		}
		return "", fmt.Errorf("gh %s: %w", args[0], err)
	}
	return strings.TrimSpace(string(out)), nil
}

// commandLines runs a command and returns its output lines.
func commandLines(name string, args ...string) ([]string, error) {
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		return nil, fmt.Errorf("%s %v: %w", name, args, err)
	}
	return strings.Fields(string(out)), nil
}

// actionsEnv reads variables GitHub Actions sets for every step.
func actionsEnv(names ...string) (map[string]string, error) {
	env := map[string]string{}
	for _, name := range names {
		if env[name] = os.Getenv(name); env[name] == "" {
			return nil, errors.New(name + " is not set: this command runs in GitHub Actions")
		}
	}
	return env, nil
}

func appendFile(path, text string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(text); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
