package CommonCrawl

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContainsAnySub(t *testing.T) {
	tests := []struct {
		name string
		str  string
		subs []string
		want bool
	}{
		{
			name: "exact match",
			str:  "text/html",
			subs: []string{"text/html"},
			want: true,
		},
		{
			name: "substring match",
			str:  "application/json; charset=utf-8",
			subs: []string{"text/html", "json"},
			want: true,
		},
		{
			name: "no match",
			str:  "image/png",
			subs: []string{"text/html", "json"},
			want: false,
		},
		{
			name: "empty substring matches",
			str:  "anything",
			subs: []string{""},
			want: true,
		},
		{
			name: "empty string and empty substring match",
			str:  "",
			subs: []string{""},
			want: true,
		},
		{
			name: "empty substring list",
			str:  "anything",
			subs: []string{},
			want: false,
		},
		{
			name: "matching is case sensitive",
			str:  "Text/HTML",
			subs: []string{"text/html"},
			want: false,
		},
		{
			name: "unicode substring",
			str:  "text/日本語",
			subs: []string{"日本"},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ContainsAnySub(tt.str, tt.subs))
		})
	}
}

func TestSanitizeMime(t *testing.T) {
	tests := []struct {
		name string
		mime string
		want string
	}{
		{
			name: "plain MIME type",
			mime: "text/html",
			want: "text_html",
		},
		{
			name: "strip parameters",
			mime: "text/html; charset=utf-8",
			want: "text_html",
		},
		{
			name: "take first comma-separated value",
			mime: "text/html, application/xhtml+xml",
			want: "text_html",
		},
		{
			name: "first delimiter determines retained value",
			mime: "text/html, application/xhtml+xml; charset=utf-8",
			want: "text_html",
		},
		{
			name: "trim whitespace around MIME type",
			mime: " \tapplication/json \r\n; charset=utf-8",
			want: "application_json",
		},
		{
			name: "empty value",
			mime: "",
			want: "_",
		},
		{
			name: "delimiter with no preceding value",
			mime: "; charset=utf-8",
			want: "_",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, SanitizeMime(tt.mime))
		})
	}
}

func TestSanitizeComponent(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "trim surrounding whitespace",
			in:   " \tcomponent name\r\n",
			want: "component name",
		},
		{
			name: "replace Windows-invalid characters",
			in:   `a<b>c:d"e/f\g|h?i*j`,
			want: "a_b_c_d_e_f_g_h_i_j",
		},
		{
			name: "remove control characters",
			in:   "line\nbreak\tand\rreturn",
			want: "linebreakandreturn",
		},
		{
			name: "trim trailing dots and spaces",
			in:   "name...  ",
			want: "name",
		},
		{
			name: "empty input uses fallback",
			in:   "",
			want: "_",
		},
		{
			name: "whitespace input uses fallback",
			in:   " \t\r\n",
			want: "_",
		},
		{
			name: "only trailing dots uses fallback",
			in:   "... ",
			want: "_",
		},
		{
			name: "preserve Unicode and internal punctuation",
			in:   "東京.file-name",
			want: "東京.file-name",
		},
		{
			name: "preserve DEL control boundary",
			in:   "a\x7fb",
			want: "a\x7fb",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, SanitizeComponent(tt.in))
		})
	}
}
