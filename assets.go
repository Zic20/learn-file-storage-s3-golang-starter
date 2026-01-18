package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
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
	url_string := base64.RawURLEncoding.EncodeToString(key)

	return fmt.Sprintf("%s.%s", url_string, contentType)
}

func (cfg apiConfig) getobjectUrl(key string) string {
	return fmt.Sprintf("%s,%s", cfg.s3Bucket, key)
	// return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, key)
}

func (cfg apiConfig) getAssetDiskPath(assetPath string) string {
	return filepath.Join(cfg.assetsRoot, assetPath)
}

func generatePresignedURL(s3Client *s3.Client, bucket, key string, expireTime time.Duration) (string, error) {
	client := s3.NewPresignClient(s3Client)
	presignedUrl, err := client.PresignGetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expireTime))

	if err != nil {
		return "", err
	}

	return presignedUrl.URL, nil
}

func (cfg *apiConfig) dbVideoToSignedVideo(video database.Video) (database.Video, error) {
	if video.VideoURL == nil {
		return video, nil
	}
	urlParts := strings.Split(*video.VideoURL, ",")
	if len(urlParts) < 2 {
		return video, errors.New("video url not properly formed")
	}
	presignedUrl, err := generatePresignedURL(cfg.s3client, urlParts[0], urlParts[1], time.Hour)
	if err != nil {
		return video, err
	}

	video.VideoURL = &presignedUrl
	return video, nil
}
