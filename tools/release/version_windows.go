package main

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// checkFileVersion checks the executable's VS_FIXEDFILEINFO file version,
// which mygo build writes from mygo.json's version, against X.Y.Z.0.
func checkFileVersion(path, version string) error {
	got, err := fileVersion(path)
	if err != nil {
		return err
	}
	if got != version+".0" {
		return fmt.Errorf("%s has file version %s, want %s.0", path, got, version)
	}
	return nil
}

func fileVersion(path string) (string, error) {
	size, err := windows.GetFileVersionInfoSize(path, nil)
	if err != nil {
		return "", err
	}
	buf := make([]byte, size)
	if err := windows.GetFileVersionInfo(path, 0, size, unsafe.Pointer(&buf[0])); err != nil {
		return "", err
	}
	var info *windows.VS_FIXEDFILEINFO
	var n uint32
	if err := windows.VerQueryValue(unsafe.Pointer(&buf[0]), `\`, unsafe.Pointer(&info), &n); err != nil {
		return "", err
	}
	return fmt.Sprintf("%d.%d.%d.%d", info.FileVersionMS>>16, info.FileVersionMS&0xffff,
		info.FileVersionLS>>16, info.FileVersionLS&0xffff), nil
}
