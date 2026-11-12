package CommonCrawl

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strconv"
	"strings"

	"github.com/joeguo/tldextract"
	"github.com/spf13/viper"
)

var extractor *tldextract.TLDExtract

func buildTLDExtractor() error {
	extr, err := tldextract.New(viper.GetString("TLDCacheFile"), false)
	if err != nil {
		return err
	}
	extractor = extr
	return nil
}

func ExtractTLD(url string) *tldextract.Result {
	info := extractor.Extract(url)
	if info.Tld == "" {
		switch info.Flag {
		case tldextract.Ip4:
			info.Tld = "ipv4"
		case tldextract.Ip6:
			info.Tld = "ipv6"
		default:
			info.Tld = "malformed"
		}
	}
	return info
}

func BuildMacroMap(rawURL string, headers map[string]string) map[string]string {
	macroMap := make(map[string]string)

	if extractor == nil {
		buildTLDExtractor()
	}

	//Add headers
	for key, val := range headers {
		macroMap["header."+key] = val
	}

	//Add URL Parts
	u, _ := url.Parse(rawURL)
	macroMap["url.host"] = u.Host
	macroMap["url.hostname"] = u.Hostname()
	macroMap["url.fragment"] = u.Fragment
	macroMap["url.opaque"] = u.Opaque
	macroMap["url.path"] = u.Path
	macroMap["url.rawfragment"] = u.RawFragment
	macroMap["url.rawpath"] = u.RawPath
	macroMap["url.rawquery"] = u.RawQuery
	macroMap["url.scheme"] = u.Scheme
	macroMap["url.user"] = u.User.String()
	macroMap["url.username"] = u.User.Username()
	macroMap["url.password"], _ = u.User.Password()

	// Hash the entire original URL.
	sum := sha256.Sum256([]byte(rawURL))
	hash := hex.EncodeToString(sum[:])
	macroMap["url.hash"] = hash
	addHashParts(macroMap, "urlhash", hash)

	//Add TLD info
	tldInfo := ExtractTLD(rawURL)
	macroMap["tld.tld"] = tldInfo.Tld
	macroMap["tld.domainroot"] = tldInfo.Root
	macroMap["tld.subdomain"] = tldInfo.Sub

	return macroMap
}

func addHashParts(m map[string]string, baseKey string, hash string) {
	for _, length := range []int{2, 3, 4} {
		for part, start := 0, 0; start < len(hash); part, start = part+1, start+length {
			end := min(start+length, len(hash))
			key := baseKey + "." + strconv.Itoa(length) + "_" + strconv.Itoa(part+1)
			m[key] = hash[start:end]
		}
	}
}

func ApplyMacros(format string, macroMap map[string]string) string {
	output := format
	macroChar := viper.GetString("MacroChar")
	for key, val := range macroMap {
		output = strings.ReplaceAll(output, macroChar+strings.ToLower(key)+macroChar, val)
	}
	return output
}
