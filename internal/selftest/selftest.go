// Package selftest is the release smoke test of the built executable
// (--self-test). The rules themselves are unit-tested in their packages;
// these cases only check that the shipped binary carries and wires them.
//
// Core holds the cases that need no window. Other packages add theirs:
//
//	report, passed := selftest.Run(append(selftest.Core(), windowCases...))
package selftest

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"

	"github.com/dzshzx/power-meter/internal/cli"
	thirdparty "github.com/dzshzx/power-meter/third_party"
)

// Case is one named check.
type Case struct {
	Name string
	Pass func() bool
}

const newline = "\n"

// Run runs the cases in order and returns the report, one PASS or FAIL line
// per case and a summary, and whether every case passed. A case that panics
// fails.
func Run(cases []Case) (string, bool) {
	var b strings.Builder
	failures := 0
	for _, c := range cases {
		mark := "PASS  "
		if !passes(c) {
			mark = "FAIL  "
			failures++
		}
		b.WriteString(mark + c.Name + newline)
	}
	b.WriteString(newline)
	if failures == 0 {
		b.WriteString("Power self test passed." + newline)
		return b.String(), true
	}
	b.WriteString(strconv.Itoa(failures) + " power self test case(s) failed." + newline)
	return b.String(), false
}

func passes(c Case) (ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	return c.Pass()
}

// Core is the cases that need no window: the command line routes, and the
// embedded IntelMSR module and notices are the ones NOTICE.md describes.
func Core() []Case {
	return []Case{
		{"command line parser routes and rejects arguments", commandLine},
		{"embedded IntelMSR module matches the third-party notice", embeddedModule},
		{"embedded third-party notices carry the LGPL text", embeddedNotices},
	}
}

func commandLine() bool {
	probe := cli.Parse([]string{"--POWER-PROBE", "out.txt", "12"})
	return probe.Valid && probe.Command == cli.PowerProbe && probe.Seconds == 12 &&
		cli.Parse(nil).Valid && !cli.Parse([]string{"--wat"}).Valid
}

func embeddedModule() bool {
	sum := sha256.Sum256(thirdparty.IntelMSR)
	return len(thirdparty.IntelMSR) > 0 && strings.Contains(thirdparty.Notice(), "`"+hex.EncodeToString(sum[:])+"`")
}

func embeddedNotices() bool {
	return strings.Contains(thirdparty.Notices(), "GNU LESSER GENERAL PUBLIC LICENSE")
}
