//go:build !windows

package settings

import "sync"

// Off Windows the settings live in memory, for tests and previews.
var (
	mu     sync.Mutex
	values = map[string]string{}
)

func read(name string) string {
	mu.Lock()
	defer mu.Unlock()
	return values[name]
}

func write(name, value string) error {
	mu.Lock()
	defer mu.Unlock()
	values[name] = value
	return nil
}
