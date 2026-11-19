package selftest

import "testing"

func TestCorePasses(t *testing.T) {
	report, passed := Run(Core())
	if !passed {
		t.Errorf("core self test failed:\n%s", report)
	}
	want := "PASS  command line parser routes and rejects arguments\r\n" +
		"PASS  embedded IntelMSR module matches the third-party notice\r\n" +
		"PASS  embedded third-party notices carry the LGPL text\r\n" +
		"\r\nPower self test passed.\r\n"
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
	want := "PASS  passes\r\nFAIL  fails\r\nFAIL  panics\r\n\r\n2 power self test case(s) failed.\r\n"
	if passed || report != want {
		t.Errorf("report:\n%q\nwant:\n%q", report, want)
	}
}
