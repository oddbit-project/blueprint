package config

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// CheckRemovedKeys returns an error wrapping ErrRemovedKey if the configuration held by p still
// contains any of the removed or renamed keys.
//
// Keys of removed are dotted paths relative to p (pass the root provider for root-relative paths).
// Matching ignores case in every segment, like encoding/json; the error reports each key as spelled
// in the config. Values of removed are replacement hints included in the error message. Only
// providers whose Get can fill a map[string]json.RawMessage (the JSON provider) are supported;
// others yield an error wrapping ErrNotImplemented.
func CheckRemovedKeys(p ConfigProvider, removed map[string]string) error {
	if len(removed) == 0 {
		return nil
	}

	paths := make([]string, 0, len(removed))
	for path := range removed {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	for _, path := range paths {
		for _, segment := range strings.Split(path, ".") {
			if segment == "" {
				return fmt.Errorf("%w: %q", ErrInvalidKeyPath, path)
			}
		}
	}
	for i, a := range paths {
		for _, b := range paths[i+1:] {
			if strings.EqualFold(a, b) {
				return fmt.Errorf("%w: %q and %q differ only in case", ErrInvalidKeyPath, a, b)
			}
		}
	}

	var root map[string]json.RawMessage
	if err := p.Get(&root); err != nil {
		return fmt.Errorf("%w: provider does not expose a config tree: %w", ErrNotImplemented, err)
	}

	var items []string
	for _, registered := range paths {
		hint := removed[registered]
		for _, actual := range findKeyPaths(root, strings.Split(registered, "."), nil) {
			item := fmt.Sprintf("%q", actual)
			if actual != registered {
				item += fmt.Sprintf(" (registered as %q)", registered)
			}
			if hint != "" {
				item += ": " + hint
			}
			items = append(items, item)
		}
	}
	if len(items) == 0 {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrRemovedKey, strings.Join(items, "; "))
}

// findKeyPaths walks obj following segments case-insensitively and returns the paths, as spelled
// in the config, of every key matching the full path.
func findKeyPaths(obj map[string]json.RawMessage, segments []string, prefix []string) []string {
	var candidates []string
	for k := range obj {
		if strings.EqualFold(k, segments[0]) {
			candidates = append(candidates, k)
		}
	}
	sort.Strings(candidates)

	var hits []string
	for _, k := range candidates {
		path := append(append([]string(nil), prefix...), k)
		if len(segments) == 1 {
			hits = append(hits, strings.Join(path, "."))
			continue
		}
		var child map[string]json.RawMessage
		if err := json.Unmarshal(obj[k], &child); err != nil || child == nil {
			continue
		}
		hits = append(hits, findKeyPaths(child, segments[1:], path)...)
	}
	return hits
}
