// Command release packages, smoke-tests, publishes and verifies Power
// Meter's Windows release assets. The commands that run the program run on
// Windows; next-version and publish run in GitHub Actions.
//
//	release set-version -version X.Y.Z
//	release next-version -bump patch|minor|major -branch <default branch> [-dry-run]
//	release package -version X.Y.Z -build dist/windows-amd64 -out <empty dir>
//	release publish -version X.Y.Z -assets <package output> [-dry-run]
//	release smoke -exe <PowerMeter.exe> -work <dir>
//	release install-test -version X.Y.Z -setup <setup.exe>
//	release verify -version X.Y.Z -out <empty dir>
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	var err error
	args := os.Args[2:]
	switch os.Args[1] {
	case "set-version":
		err = runSetVersion(args)
	case "next-version":
		err = runNextVersion(args)
	case "package":
		err = runPackage(args)
	case "publish":
		err = runPublish(args)
	case "smoke":
		err = runSmoke(args)
	case "install-test":
		err = runInstallTest(args)
	case "verify":
		err = runVerify(args)
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "release:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: release set-version|next-version|package|publish|smoke|install-test|verify [flags]")
	os.Exit(2)
}

// parse parses a subcommand's flags and requires the named ones.
func parse(fs *flag.FlagSet, args []string, required ...string) error {
	if err := fs.Parse(args); err != nil {
		return err
	}
	for _, name := range required {
		if f := fs.Lookup(name); f == nil || f.Value.String() == "" {
			return errors.New("missing -" + name)
		}
	}
	return nil
}
