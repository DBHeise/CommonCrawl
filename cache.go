package CommonCrawl

import (
	"net/url"
	"os"
	"path"
	"path/filepath"
)

type Cache struct {
	localFolder string
	cache       map[string]struct{}
}

func NewCache(folder string) *Cache {
	c := &Cache{
		localFolder: folder,
		cache:       make(map[string]struct{}),
	}
	c.Rebuild()
	return c
}

func (c *Cache) Rebuild() error {
	entries, err := os.ReadDir(c.localFolder)
	if err != nil {
		return err
	}

	files := make(map[string]struct{}, len(entries))

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		files[entry.Name()] = struct{}{}
	}

	c.cache = files
	return nil
}

func (c *Cache) Download(method, u string) (string, error) {

	u2, err := url.Parse(u)
	if err != nil {
		return "", err
	}
	filename := path.Base(u2.Path)

	local_file := filepath.Join(c.localFolder, filename)
	if _, exists := c.cache[filename]; !exists {
		err := DownloadToFile(method, u, local_file)
		if err != nil {
			return "", err
		}
		c.cache[filename] = struct{}{}
	}
	return local_file, nil
}

func (c *Cache) DownloadPage(method, u string) ([]byte, error) {
	local_file, err := c.Download(method, u)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(local_file)
}

func (c *Cache) DownloadAndUnGZip(method, u string) ([]byte, error) {

	content, err := c.DownloadPage(method, u)
	if err != nil {
		return nil, err
	}

	decompressed, err := DecompressGZIP(content)
	if err != nil {
		return nil, err
	}

	return decompressed, nil

}
