package selftest

import "testing"

func TestCorePasses(t *testing.T) {
	report, passed := Run(Core())
	if !passed {
		t.Errorf("core self test failed:\n%s", report)
	}
	want := "PASS  command line parser routes and rejects arguments\n" +
		"PASS  embedded IntelMSR module matches the third-party notice\n" +
		"PASS  embedded third-party notices carry the LGPL text\n" +
		"\nPower self test passed.\n"
	if report != want {
		t.Errorf("report:\n%q\nwant:\n%q", report, want)
	}
}

func TestFailuresAreCounted(t *testing.T) {
	cases := []Case{
		{"passes", func() bool { return true }},
		{"fails", func() bool { return false }},
		{"panics", func() bool { panic("boom") }},
	}
	report, passed := Run(cases)
	want := "PASS  passes\nFAIL  fails\nFAIL  panics\n\n2 power self test case(s) failed.\n"
	if passed || report != want {
		t.Errorf("report:\n%q\nwant:\n%q", report, want)
	}
}
