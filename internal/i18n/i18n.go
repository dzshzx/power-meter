// Package i18n holds the app's two languages. Chinese source strings are the
// lookup keys. Hardware diagnostics stay in their original form until
// presentation, so cached failures and source descriptions change language
// without reopening a driver.
package i18n

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
)

// Languages a user can choose; "system" follows the Windows display language.
const (
	System  = "system"
	Chinese = "zh-CN"
	English = "en"
)

type entry struct{ key, english string }

var (
	english = func() map[string]string {
		m := make(map[string]string, len(table))
		for _, e := range table {
			m[e.key] = e.english
		}
		return m
	}()

	mu            sync.RWMutex
	preference    = System
	systemCulture = ""
)

// Resolve picks the language shown for a saved preference and the system's
// UI culture name: a valid saved choice wins, otherwise any Chinese culture
// shows Chinese and every other culture English.
func Resolve(saved, culture string) string {
	if saved == Chinese || saved == English {
		return saved
	}
	if strings.HasPrefix(strings.ToLower(culture), "zh") {
		return Chinese
	}
	return English
}

// SetSystemCulture records the system's UI culture name, such as "zh-CN".
func SetSystemCulture(name string) {
	mu.Lock()
	systemCulture = name
	mu.Unlock()
}

// Preference is the selected language: System, Chinese or English.
func Preference() string {
	mu.RLock()
	defer mu.RUnlock()
	return preference
}

// ErrUnsupported reports a language other than System, Chinese or English.
var ErrUnsupported = errors.New("unsupported language")

// Select switches the language for the rest of the process. Saving the
// choice is the caller's.
func Select(language string) error {
	if language != System && language != Chinese && language != English {
		return ErrUnsupported
	}
	mu.Lock()
	preference = language
	mu.Unlock()
	return nil
}

// Load selects a saved preference, falling back to System for anything
// missing or malformed.
func Load(saved string) {
	if Select(saved) != nil {
		_ = Select(System)
	}
}

// IsChinese reports whether text shows in Chinese now.
func IsChinese() bool {
	mu.RLock()
	defer mu.RUnlock()
	return Resolve(preference, systemCulture) == Chinese
}

// AppName is the app's name in the current language.
func AppName() string { return Get("功率计") }

// Get translates a source string; unknown keys show as they are.
func Get(key string) string {
	if !IsChinese() {
		if v, ok := english[key]; ok {
			return v
		}
	}
	return key
}

// Format translates key and fills its {0}, {1}… placeholders.
func Format(key string, args ...any) string {
	return fill(Get(key), args...)
}

func fill(text string, args ...any) string {
	for i, a := range args {
		text = strings.ReplaceAll(text, "{"+strconv.Itoa(i)+"}", fmt.Sprint(a))
	}
	return text
}

// Diagnostic translates a reason or source description. Only authored
// prefixes are translated; OS error text, identifiers and user paths keep
// every byte.
func Diagnostic(text string) string {
	if text == "" || IsChinese() {
		return text
	}
	if v, ok := english[text]; ok {
		return v
	}
	for _, e := range table {
		if prefix, ok := strings.CutSuffix(e.key, "{0}"); ok && strings.HasPrefix(text, prefix) {
			return strings.ReplaceAll(e.english, "{0}", Diagnostic(text[len(prefix):]))
		}
		if (strings.HasSuffix(e.key, ": ") || strings.HasSuffix(e.key, "：") || strings.HasSuffix(e.key, ": 0x")) &&
			strings.HasPrefix(text, e.key) {
			return e.english + text[len(e.key):]
		}
	}
	const checkPrefix = "单位换算校验失败（"
	if rest, ok := strings.CutPrefix(text, checkPrefix); ok && strings.HasSuffix(rest, "）") {
		return "Energy-unit validation failed (" + strings.TrimSuffix(rest, "）") + ")"
	}
	if rest, ok := strings.CutSuffix(text, "（EMI 未交叉验证）"); ok {
		return rest + " (not cross-validated with EMI)"
	}
	if before, after, ok := strings.Cut(text, " 读取失败: 0x"); ok {
		return before + " read failed: 0x" + after
	}
	return text
}
