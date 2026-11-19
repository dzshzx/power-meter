//go:build !windows

package main

import "errors"

func checkFileVersion(string, string) error {
	return errors.New("file versions are read on Windows")
}
