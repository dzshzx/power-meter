// Package settings keeps the user's display mode and language under
// HKCU\Software\PowerMeter. Reads never fail: a missing or unreadable value
// gives the default, as a fresh install has.
package settings

import (
	"github.com/dzshzx/power-meter/internal/i18n"
	"github.com/dzshzx/power-meter/internal/meter"
)

// Path is the settings key under HKEY_CURRENT_USER.
const Path = `Software\PowerMeter`

// Value names.
const (
	displayMode = "DisplayMode"
	language    = "Language"
)

// Mode is the saved display mode, whole-system power by default.
func Mode() meter.Mode {
	return meter.ParseMode(read(displayMode))
}

// SaveMode remembers the display mode.
func SaveMode(m meter.Mode) error {
	return write(displayMode, m.String())
}

// Language is the saved language preference: i18n.System, Chinese or
// English.
func Language() string {
	switch v := read(language); v {
	case i18n.Chinese, i18n.English:
		return v
	}
	return i18n.System
}

// SaveLanguage remembers the language preference.
func SaveLanguage(lang string) error {
	return write(language, lang)
}
