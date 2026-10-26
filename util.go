package CommonCrawl

import "strings"

func ContainsAnySub(str string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(str, sub) {
			return true // Short-circuit as soon as a match is found
		}
	}
	return false
}

func SanitizeMime(mime string) string {
	splitters := []rune{';', ','}
	for _, s := range splitters {
		if strings.ContainsRune(mime, s) {
			mime = strings.Split(mime, string(s))[0]
		}
	}
	return SanitizeComponent(mime)
}

func SanitizeComponent(s string) string {
	s = strings.TrimSpace(s)

	// Windows-invalid characters.
	replacer := strings.NewReplacer(
		"<", "_",
		">", "_",
		":", "_",
		"\"", "_",
		"/", "_",
		"\\", "_",
		"|", "_",
		"?", "_",
		"*", "_",
	)

	s = replacer.Replace(s)

	// Remove control chars.
	var b strings.Builder
	for _, r := range s {
		if r < 32 {
			continue
		}
		b.WriteRune(r)
	}

	s = b.String()

	// Windows doesn't like trailing dots/spaces.
	s = strings.TrimRight(s, ". ")

	if s == "" {
		s = "_"
	}

	return s
}
