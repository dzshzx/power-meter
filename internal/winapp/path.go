package winapp

import "strings"

// FullPath normalizes a fully qualified Windows path the way the app compares
// paths: forward slashes become backslashes, repeated separators collapse,
// and "." and ".." segments are resolved. It is lexical, so it behaves the
// same on every platform and never consults the file system. ok is false
// for anything that is not fully qualified (a drive letter with a separator,
// or a UNC share), including relative paths.
func FullPath(p string) (string, bool) {
	p = strings.ReplaceAll(p, "/", `\`)
	if rest, ok := strings.CutPrefix(p, `\\?\UNC\`); ok {
		p = `\\` + rest
	} else if rest, ok := strings.CutPrefix(p, `\\?\`); ok {
		p = rest
	}
	prefix, rest, ok := splitRoot(p)
	if !ok {
		return "", false
	}
	var segments []string
	for _, s := range strings.Split(rest, `\`) {
		switch s {
		case "", ".":
		case "..":
			if len(segments) > 0 {
				segments = segments[:len(segments)-1]
			}
		default:
			segments = append(segments, s)
		}
	}
	if len(segments) == 0 {
		return prefix + `\`, true
	}
	return prefix + `\` + strings.Join(segments, `\`), true
}

// splitRoot separates "C:" or `\\server\share` from the rest of the path.
func splitRoot(p string) (prefix, rest string, ok bool) {
	if len(p) >= 3 && isLetter(p[0]) && p[1] == ':' && p[2] == '\\' {
		return p[:2], p[3:], true
	}
	unc, found := strings.CutPrefix(p, `\\`)
	if !found || strings.HasPrefix(unc, `.\`) || strings.HasPrefix(unc, `?\`) {
		return "", "", false
	}
	parts := strings.SplitN(unc, `\`, 3)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	if len(parts) == 3 {
		rest = parts[2]
	}
	return `\\` + parts[0] + `\` + parts[1], rest, true
}

func isLetter(c byte) bool { return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' }

// SamePath reports whether two fully qualified paths name the same file,
// ignoring case as Windows does. A path that is not fully qualified never
// matches.
func SamePath(a, b string) bool {
	fa, okA := FullPath(a)
	fb, okB := FullPath(b)
	return okA && okB && strings.EqualFold(fa, fb)
}

// SplitPath splits a path from FullPath into its folder and final name.
// The folder of a top-level entry is the root itself, such as `C:\`.
func SplitPath(full string) (dir, name string) {
	i := strings.LastIndexByte(full, '\\')
	if i < 0 {
		return "", full
	}
	dir, name = full[:i], full[i+1:]
	if strings.HasSuffix(dir, ":") {
		dir += `\`
	}
	return dir, name
}
