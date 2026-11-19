// Command release packages, smoke-tests and verifies Power Meter's Windows
// release assets. It runs on Windows, where the program it checks runs.
//
//	release package -version X.Y.Z -build dist/windows-amd64 -out <empty dir>
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
	case "package":
		err = runPackage(args)
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
	fmt.Fprintln(os.Stderr, "usage: release package|smoke|install-test|verify [flags]")
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
