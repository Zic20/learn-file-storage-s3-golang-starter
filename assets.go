package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (cfg apiConfig) ensureAssetsDir() error {
	if _, err := os.Stat(cfg.assetsRoot); os.IsNotExist(err) {
		return os.Mkdir(cfg.assetsRoot, 0755)
	}
	return nil
}

func getAssetPath(filetype string) string {
	contentType := strings.Split(filetype, "/")[1]
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		panic("error generating random bytes")
	}
	url_string := base64.RawStdEncoding.EncodeToString(key)

	return strings.Replace(fmt.Sprintf("%s.%s", url_string, contentType), "/", "", -1)
}

func (cfg apiConfig) getobjectUrl(key string) string {
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, key)
}

func (cfg apiConfig) getAssetDiskPath(assetPath string) string {
	return filepath.Join(cfg.assetsRoot, assetPath)
}
