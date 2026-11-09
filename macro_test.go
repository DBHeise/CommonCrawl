package CommonCrawl

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/joeguo/tldextract"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestExtractTLD(t *testing.T) {
	setupMacroTestTLDs(t)

	tests := []struct {
		name string
		url  string
		want tldextract.Result
	}{
		{
			name: "extract subdomain and common suffix",
			url:  "https://www.example.com/articles",
			want: tldextract.Result{Flag: tldextract.Domain, Sub: "www", Root: "example", Tld: "com"},
		},
		{
			name: "extract multi-label suffix",
			url:  "shop.example.co.uk",
			want: tldextract.Result{Flag: tldextract.Domain, Sub: "shop", Root: "example", Tld: "co.uk"},
		},
		{
			name: "extract IPv4 address",
			url:  "http://192.0.2.10/foo/bar/baz.ext?query=blah#werga",
			want: tldextract.Result{Flag: tldextract.Ip4, Tld: "ipv4", Root: "192.0.2.10"},
		},
		// {
		// 	name: "extract IPv6 address",
		// 	url:  "http://2001:db8:1::ab9:C0A8:102/",
		// 	want: tldextract.Result{Flag: tldextract.Ip6, Tld: "ipv6", Root: "2001:0db8:0000:2f3b:02aa:00ff:f328:9c5a"},
		// },
		{
			name: "return malformed result for unknown suffix",
			url:  "www.example.invalid",
			want: tldextract.Result{Flag: tldextract.Malformed, Tld: "malformed"},
		},
		{
			name: "return malformed result for hostname without suffix",
			url:  "localhost",
			want: tldextract.Result{Flag: tldextract.Malformed, Tld: "malformed"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractTLD(tt.url)
			if assert.NotNil(t, got) {
				assert.Equal(t, tt.want, *got)
			}
		})
	}
}

func TestBuildMacroMap(t *testing.T) {
	setupMacroTestTLDs(t)

	tests := []struct {
		name    string
		rawURL  string
		headers map[string]string
		urlWant map[string]string
		tldWant map[string]string
	}{
		{
			name:   "include URL components, user info, headers and hash macros",
			rawURL: "https://user:p%40ss@www.example.com:8443/path%2Fto/page.html?b=two&a=one#frag%2fment",
			headers: map[string]string{
				"Content-Type": "text/html",
				"X-Request-ID": "req-123",
			},
			urlWant: map[string]string{
				"url.host":        "www.example.com:8443",
				"url.hostname":    "www.example.com",
				"url.fragment":    "frag/ment",
				"url.opaque":      "",
				"url.path":        "/path/to/page.html",
				"url.rawfragment": "frag%2fment",
				"url.rawpath":     "/path%2Fto/page.html",
				"url.rawquery":    "b=two&a=one",
				"url.scheme":      "https",
				"url.user":        "user:p%40ss",
				"url.username":    "user",
				"url.password":    "p@ss",
			},
			tldWant: map[string]string{
				"tld.tld":        "com",
				"tld.domainroot": "example",
				"tld.subdomain":  "www",
			},
		},
		{
			name:   "include empty components and IP TLD result",
			rawURL: "http://192.0.2.10/",
			urlWant: map[string]string{
				"url.host":        "192.0.2.10",
				"url.hostname":    "192.0.2.10",
				"url.fragment":    "",
				"url.opaque":      "",
				"url.path":        "/",
				"url.rawfragment": "",
				"url.rawpath":     "",
				"url.rawquery":    "",
				"url.scheme":      "http",
				"url.user":        "",
				"url.username":    "",
				"url.password":    "",
			},
			tldWant: map[string]string{
				"tld.tld":        "ipv4",
				"tld.domainroot": "192.0.2.10",
				"tld.subdomain":  "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildMacroMap(tt.rawURL, tt.headers)

			want := make(map[string]string, len(tt.urlWant)+len(tt.tldWant)+len(tt.headers))
			for key, value := range tt.urlWant {
				want[key] = value
			}
			for key, value := range tt.tldWant {
				want[key] = value
			}
			for key, value := range tt.headers {
				want["header."+key] = value
			}

			hash := sha256.Sum256([]byte(tt.rawURL))
			hashString := hex.EncodeToString(hash[:])
			want["url.hash"] = hashString
			addWantHashParts(want, "urlhash", hashString)

			assert.Equal(t, want, got)
		})
	}
}

func TestApplyMacros(t *testing.T) {
	tests := []struct {
		name     string
		format   string
		macroMap map[string]string
		want     string
	}{
		{
			name:   "replace several macros",
			format: ":url.scheme:://:url.hostname:/:url.path:",
			macroMap: map[string]string{
				"url.scheme":   "https",
				"url.hostname": "example.com",
				"url.path":     "page.html",
			},
			want: "https://example.com/page.html",
		},
		{
			name:   "replace repeated macros",
			format: ":x:/:x:",
			macroMap: map[string]string{
				"x": "value",
			},
			want: "value/value",
		},
		{
			name:   "match macro keys without regard to key case",
			format: ":header.content-type:",
			macroMap: map[string]string{
				"Header.Content-Type": "text/html",
			},
			want: "text/html",
		},
		{
			name:   "leave unknown macros unchanged",
			format: ":known:/:unknown:",
			macroMap: map[string]string{
				"known": "replaced",
			},
			want: "replaced/:unknown:",
		},
		{
			name:     "leave a partial token unchanged",
			format:   ":url.hostname",
			macroMap: map[string]string{"url.hostname": "example.com"},
			want:     ":url.hostname",
		},
		{
			name:     "handle empty format and macro map",
			format:   "",
			macroMap: map[string]string{},
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ApplyMacros(tt.format, tt.macroMap))
		})
	}
}

func TestAddHashParts(t *testing.T) {
	tests := []struct {
		name    string
		baseKey string
		hash    string
		want    map[string]string
	}{
		{
			name:    "split into full chunks",
			baseKey: "url",
			hash:    "0123456789",
			want: map[string]string{
				"url.2_1": "01", "url.2_2": "23", "url.2_3": "45", "url.2_4": "67", "url.2_5": "89",
				"url.3_1": "012", "url.3_2": "345", "url.3_3": "678", "url.3_4": "9",
				"url.4_1": "0123", "url.4_2": "4567", "url.4_3": "89",
			},
		},
		{
			name:    "include trailing partial chunks",
			baseKey: "custom",
			hash:    "abcdefg",
			want: map[string]string{
				"custom.2_1": "ab", "custom.2_2": "cd", "custom.2_3": "ef", "custom.2_4": "g",
				"custom.3_1": "abc", "custom.3_2": "def", "custom.3_3": "g",
				"custom.4_1": "abcd", "custom.4_2": "efg",
			},
		},
		{
			name:    "hash shorter than some chunk lengths",
			baseKey: "url",
			hash:    "abc",
			want: map[string]string{
				"url.2_1": "ab", "url.2_2": "c",
				"url.3_1": "abc",
				"url.4_1": "abc",
			},
		},
		{
			name:    "empty hash",
			baseKey: "url",
			hash:    "",
			want:    map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := make(map[string]string)
			addHashParts(got, tt.baseKey, tt.hash)
			assert.Equal(t, tt.want, got)
		})
	}
}

func setupMacroTestTLDs(t *testing.T) {
	t.Helper()

	const tldCacheFileKey = "TLDCacheFile"
	previousTLDCacheFile := viper.GetString(tldCacheFileKey)
	previousExtractor := extractor
	t.Cleanup(func() {
		viper.Set(tldCacheFileKey, previousTLDCacheFile)
		extractor = previousExtractor
	})

	tldCacheFile := filepath.Join(t.TempDir(), "tld.cache")
	if err := os.WriteFile(tldCacheFile, []byte("com\norg\nnet\nuk\nco.uk\n"), 0600); err != nil {
		t.Fatalf("write TLD cache: %v", err)
	}
	viper.Set(tldCacheFileKey, tldCacheFile)
	extractor = nil
	if err := buildTLDExtractor(); err != nil {
		t.Fatalf("build TLD extractor: %v", err)
	}
}

func addWantHashParts(want map[string]string, baseKey, hash string) {
	for _, length := range []int{2, 3, 4} {
		for part, start := 0, 0; start < len(hash); part, start = part+1, start+length {
			end := min(start+length, len(hash))
			key := baseKey + "." + strconv.Itoa(length) + "_" + strconv.Itoa(part+1)
			want[key] = hash[start:end]
		}
	}
}
