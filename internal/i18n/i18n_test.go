package i18n

import "testing"

func TestResolve(t *testing.T) {
	if Resolve("en", "zh-CN") != English || Resolve("zh-CN", "en-US") != Chinese ||
		Resolve("system", "zh-TW") != Chinese || Resolve("invalid", "fr-FR") != English {
		t.Error("language preference overrides the system, with safe fallback")
	}
}

func TestLanguageSwitch(t *testing.T) {
	t.Cleanup(func() { _ = Select(System) })
	const cached = "需要以管理员身份运行才能读取平台功率"
	if err := Select(English); err != nil {
		t.Fatal(err)
	}
	if AppName() != "Power Meter" || Diagnostic(cached) != "Run as administrator to read platform power" ||
		Diagnostic(`EMI 初始化失败: C:\测试`) != `EMI initialization failed: C:\测试` {
		t.Errorf("English branding and cached diagnostic presentation: %q %q", AppName(), Diagnostic(cached))
	}
	if err := Select(Chinese); err != nil {
		t.Fatal(err)
	}
	if AppName() != "功率计" || Diagnostic(cached) != cached {
		t.Error("Chinese branding and diagnostics switch back without driver restart")
	}
	if Select("fr") == nil {
		t.Error("unsupported language accepted")
	}
	Load("bogus")
	if Preference() != System {
		t.Error("a malformed saved preference falls back to system")
	}
}

func TestDiagnostic(t *testing.T) {
	t.Cleanup(func() { _ = Select(System) })
	_ = Select(English)
	cases := map[string]string{
		"单位换算校验失败（MSR 24.50 W vs EMI 12.02 W）": "Energy-unit validation failed (MSR 24.50 W vs EMI 12.02 W)",
		"PawnIO MSR 0x64D（EMI 未交叉验证）":          "PawnIO MSR 0x64D (not cross-validated with EMI)",
		"RAPL 读取失败: 0x80004005":                "RAPL read failed: 0x80004005",
		"自启设置未完成：拒绝访问。":                        "Startup settings were not changed: 拒绝访问。",
		"自启设置未完成：需要以管理员身份运行才能读取平台功率":           "Startup settings were not changed: Run as administrator to read platform power",
		"加载 IntelMSR 模块失败: 0x1F":               "Failed to load IntelMSR module: 0x1F",
		"OS said no":                           "OS said no",
	}
	for in, want := range cases {
		if got := Diagnostic(in); got != want {
			t.Errorf("Diagnostic(%q) = %q, want %q", in, got, want)
		}
	}
	if Format("均值 · {0}/10分", Format("{0}秒", "3")) != "Avg · 3s/10m" {
		t.Errorf("format: %q", Format("均值 · {0}/10分", Format("{0}秒", "3")))
	}
	if Format("{0} 分 {1} 秒", 2, 5) != "2 min 5s" {
		t.Error("two placeholders")
	}
	if Get("not a key") != "not a key" {
		t.Error("unknown keys show as they are")
	}
}

func TestTableKeysUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, e := range table {
		if seen[e.key] {
			t.Errorf("duplicate key %q", e.key)
		}
		seen[e.key] = true
	}
}
