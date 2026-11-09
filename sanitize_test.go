package CommonCrawl

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizePath(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		replacement rune
		want        string
		wantErr     string
	}{
		{
			name:        "preserve a relative path",
			path:        "archive/pages/index.html",
			replacement: '_',
			want:        filepath.Join("archive", "pages", "index.html"),
		},
		{
			name:        "normalize Windows separators",
			path:        `archive\pages\index.html`,
			replacement: '_',
			want:        filepath.Join("archive", "pages", "index.html"),
		},
		{
			name:        "preserve a fully qualified Windows drive path",
			path:        `D:\archive\pages\index.html`,
			replacement: '_',
			want:        filepath.Join(`D:\`, "archive", "pages", "index.html"),
		},
		{
			name:        "discard empty and current-directory components",
			path:        "./archive//./pages/",
			replacement: '_',
			want:        filepath.Join("archive", "pages"),
		},
		{
			name:        "replace invalid characters in each component",
			path:        `dir/a<b>/file:name?.txt`,
			replacement: '_',
			want:        filepath.Join("dir", "a_b_", "file_name_.txt"),
		},
		{
			name:        "use custom replacement character",
			path:        "dir/file:name",
			replacement: '-',
			want:        filepath.Join("dir", "file-name"),
		},
		{
			name:        "trim component whitespace and trailing dots",
			path:        " dir / name... /file ",
			replacement: '_',
			want:        filepath.Join("dir", "name", "file"),
		},
		{
			name:        "replace control characters",
			path:        "dir/line\nbreak",
			replacement: '_',
			want:        filepath.Join("dir", "line_break"),
		},
		{
			name:        "prefix a Windows reserved filename",
			path:        "dir/CON.txt",
			replacement: '_',
			want:        filepath.Join("dir", "_CON.txt"),
		},
		{
			name:        "reserved filename matching is case insensitive",
			path:        "aux",
			replacement: '-',
			want:        "-aux",
		},
		{
			name:        "allow names that only begin with a reserved name",
			path:        "COM10",
			replacement: '_',
			want:        "COM10",
		},
		{
			name:        "reject an empty path",
			path:        "",
			replacement: '_',
			wantErr:     "empty path",
		},
		{
			name:        "reject forward-slash traversal",
			path:        "dir/../file",
			replacement: '_',
			wantErr:     "path traversal detected",
		},
		{
			name:        "reject backslash traversal",
			path:        `dir\..\file`,
			replacement: '_',
			wantErr:     "path traversal detected",
		},
		{
			name:        "reject a path with no remaining components",
			path:        "./",
			replacement: '_',
			wantErr:     "path became empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SanitizePath(tt.path, tt.replacement)

			if tt.wantErr != "" {
				if assert.EqualError(t, err, tt.wantErr) {
					assert.Empty(t, got)
				}
				return
			}

			if assert.NoError(t, err) {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
