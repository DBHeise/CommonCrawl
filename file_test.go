package CommonCrawl

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFileExists(t *testing.T) {
	dir := t.TempDir()
	existingFile := filepath.Join(dir, "existing.txt")
	assert.NoError(t, os.WriteFile(existingFile, []byte("content"), 0600))

	tests := []struct {
		name string
		path string
		want bool
		err  bool
	}{
		{
			name: "existing file",
			path: existingFile,
			want: true,
		},
		{
			name: "existing directory",
			path: dir,
			want: true,
		},
		{
			name: "missing file",
			path: filepath.Join(dir, "missing.txt"),
			want: false,
		},
		{
			name: "missing nested file",
			path: filepath.Join(dir, "missing", "nested.txt"),
			want: false,
		},
		{
			name: "invalid path",
			path: "\x00",
			want: false,
			err:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FileExists(tt.path)
			if tt.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}
