package CommonCrawl

import (
	"errors"
	"path/filepath"
	"strings"
)

var windowsReserved = map[string]struct{}{
	"CON": {}, "PRN": {}, "AUX": {}, "NUL": {},
	"COM1": {}, "COM2": {}, "COM3": {}, "COM4": {},
	"COM5": {}, "COM6": {}, "COM7": {}, "COM8": {}, "COM9": {},
	"LPT1": {}, "LPT2": {}, "LPT3": {}, "LPT4": {},
	"LPT5": {}, "LPT6": {}, "LPT7": {}, "LPT8": {}, "LPT9": {},
}

// SanitizePath converts an arbitrary user path into a safe path.
func SanitizePath(path string, replacement rune) (string, error) {
	if path == "" {
		return "", errors.New("empty path")
	}

	// Normalize all separators first.
	path = strings.ReplaceAll(path, "\\", "/")

	isAbs := filepath.IsAbs(path)
	volume := filepath.VolumeName(path)
	path = strings.TrimPrefix(path, volume)

	parts := strings.Split(path, "/")
	safeParts := make([]string, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)

		if part == "" || part == "." {
			continue
		}

		// Reject traversal entirely.
		if part == ".." {
			return "", errors.New("path traversal detected")
		}

		part = sanitizeComponent(part, replacement)

		if part == "" {
			continue
		}

		base := strings.ToUpper(strings.TrimSuffix(part, filepath.Ext(part)))
		if _, found := windowsReserved[base]; found {
			part = string(replacement) + part
		}

		safeParts = append(safeParts, part)
	}

	if len(safeParts) == 0 {
		if volume != "" && isAbs {
			return volume + string(filepath.Separator), nil
		}
		return "", errors.New("path became empty")
	}

	safePath := filepath.Join(safeParts...)
	if volume != "" {
		if isAbs {
			return filepath.Join(volume+string(filepath.Separator), safePath), nil
		}
		return volume + safePath, nil
	}
	if isAbs {
		return filepath.Join(string(filepath.Separator), safePath), nil
	}
	return safePath, nil
}

func sanitizeComponent(name string, replacement rune) string {
	var b strings.Builder

	for _, r := range name {
		switch {
		// Control characters
		case r < 32:
			b.WriteRune(replacement)

		// Illegal on Windows, therefore illegal everywhere.
		case strings.ContainsRune(`<>:"/\|?*`, r):
			b.WriteRune(replacement)

		default:
			b.WriteRune(r)
		}
	}

	result := b.String()

	// Windows incompatibilities.
	result = strings.TrimRight(result, ". ")
	result = strings.TrimSpace(result)

	return result
}
