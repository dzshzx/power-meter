//go:build !windows

package main

import "errors"

func runInstallTest([]string) error {
	return errors.New("install-test runs on Windows")
}
