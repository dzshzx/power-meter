package winapp

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
)

// The protected logon copy is <root>\<SID>\PowerMeter.exe, with source.txt
// beside it naming the installation it was copied from.
const (
	ExecutableName = "PowerMeter.exe"
	SourceName     = "source.txt"
)

// IdentityFor is the installation the image at path represents: the image
// itself, or for a protected copy the installation recorded in its
// source.txt. Launching that installation then restores the running logon
// instance instead of starting a second one. It only names instances;
// callers still refuse to copy whenever image and identity differ.
func IdentityFor(image, sid string) string {
	return identityFor(image, sid, os.ReadFile)
}

func identityFor(image, sid string, read func(string) ([]byte, error)) string {
	full, ok := FullPath(image)
	if !ok {
		return image
	}
	folder, name := SplitPath(full)
	_, folderName := SplitPath(folder)
	if !strings.EqualFold(name, ExecutableName) || !strings.EqualFold(folderName, sid) {
		return full
	}
	if source := readSource(folder, read); source != "" {
		return source
	}
	return full
}

// CopySource is the installation recorded in a protected copy folder's
// source.txt, or "" when the file is missing, unreadable or does not hold a
// fully qualified path. It is only used for ownership decisions and instance
// naming, never as a file to run or copy.
func CopySource(folder string) string {
	return readSource(folder, os.ReadFile)
}

func readSource(folder string, read func(string) ([]byte, error)) string {
	data, err := read(folder + `\` + SourceName)
	if err != nil {
		return ""
	}
	text := strings.TrimSpace(strings.TrimPrefix(string(data), "\uFEFF"))
	full, ok := FullPath(text)
	if !ok {
		return ""
	}
	return full
}

// instanceKey names the single-instance objects of one installation for
// one user: the user's SID and a hash of the installation's path, so the
// name stays short and case-insensitive.
func instanceKey(sid, identity string) string {
	full, ok := FullPath(identity)
	if !ok {
		full = identity
	}
	sum := sha256.Sum256([]byte(strings.ToUpper(full)))
	return sid + "." + strings.ToUpper(hex.EncodeToString(sum[:]))
}
