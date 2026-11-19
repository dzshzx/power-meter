# MiSans faces

`go run ./tools/fonts` (or `go generate ./internal/fonts`) puts
`MiSans-Regular.ttf.gz`, `MiSans-Medium.ttf.gz` and `MiSans-Semibold.ttf.gz`
here from Xiaomi's official package, checked against the SHA-256 sums pinned
in `tools/fonts`. They are not committed: the MiSans licence forbids
distributing the fonts on their own.

The program compiles without them and then draws with the system's fonts; its
`--self-test` fails, so the Windows validation and Release workflows, which
fetch them, are the only builds that ship.
