package settings

import "golang.org/x/sys/windows/registry"

func read(name string) string {
	k, err := registry.OpenKey(registry.CURRENT_USER, Path, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()
	v, _, err := k.GetStringValue(name)
	if err != nil {
		return ""
	}
	return v
}

func write(name, value string) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, Path, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	return k.SetStringValue(name, value)
}
