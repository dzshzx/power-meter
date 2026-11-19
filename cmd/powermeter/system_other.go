//go:build !windows

package main

func systemCulture() string { return "" }
func systemDPI() int        { return 96 }
func darkAppearance() bool  { return false }
