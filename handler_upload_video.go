package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"os/exec"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	const maxMemory = 1 << 30
	r.ParseMultipartForm(maxMemory)
	r.Body = http.MaxBytesReader(w, r.Body, maxMemory)

	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid id", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "couldn't find token", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "couldn't validate token", err)
		return
	}

	videoMetadata, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "video not found", err)
		return
	}

	if videoMetadata.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "user does not have access to this resource", err)
		return
	}

	file, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "video not found", err)
		return
	}

	defer file.Close()
	contentType, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "couldn't parse file media type", err)
		return
	}

	if contentType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "unsupported file format. video/mp4 file expected", nil)
		return
	}

	tmpFile, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error uploading file", err)
		return
	}

	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()
	_, err = io.Copy(tmpFile, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "couldn't upload file to disk", err)
		return
	}

	_, err = tmpFile.Seek(0, io.SeekStart)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "couldn't reset file pointer", err)
		return
	}
	key := getAssetPath(contentType)

	aspectRatio, err := getVideoAspectRatio(tmpFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "couldn't get the video's aspect ratio", err)
		return
	}

	switch aspectRatio {
	case "16:9":
		key = "landscape/" + key
	case "9:16":
		key = "portrait/" + key
	default:
		key = "other/" + key
	}

	faststartVideoPath, err := processVideoForFastStart(tmpFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "couldn't process video for fast start", err)
		return
	}

	faststartVideo, err := os.Open(faststartVideoPath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error fetching fast start video", err)
		return
	}
	defer os.Remove(faststartVideoPath)

	_, err = cfg.s3client.PutObject(r.Context(), &s3.PutObjectInput{
		Bucket:      aws.String(cfg.s3Bucket),
		Key:         aws.String(key),
		Body:        faststartVideo,
		ContentType: aws.String(contentType),
	})

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error uploading file to s3", err)
		return
	}

	url := cfg.getobjectUrl(key)
	videoMetadata.VideoURL = &url
	err = cfg.db.UpdateVideo(videoMetadata)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "couldn't update video", err)
		return
	}

	presignedVideo, err := cfg.dbVideoToSignedVideo(videoMetadata)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error generating presigned video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, presignedVideo)

}

func getVideoAspectRatio(filepath string) (string, error) {
	buffer := bytes.Buffer{}
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filepath)
	cmd.Stdout = &buffer

	if err := cmd.Run(); err != nil {
		return "", err
	}

	type aspectRatioInfo struct {
		Streams []struct {
			DisplayAspectRatio string `json:"display_aspect_ratio"`
		} `json:"streams"`
	}

	var aspectRatio aspectRatioInfo

	if err := json.Unmarshal(buffer.Bytes(), &aspectRatio); err != nil {
		return "", err
	}

	if len(aspectRatio.Streams) == 0 {
		return "", fmt.Errorf("no streams found in media file")
	}

	return aspectRatio.Streams[0].DisplayAspectRatio, nil
}

func processVideoForFastStart(filePath string) (string, error) {
	processedFilePath := fmt.Sprintf("%s.processing", filePath)

	cmd := exec.Command("ffmpeg", "-i", filePath, "-movflags", "faststart", "-codec", "copy", "-f", "mp4", processedFilePath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("error processing video: %s, %v", stderr.String(), err)
	}

	fileInfo, err := os.Stat(processedFilePath)
	if err != nil {
		return "", fmt.Errorf("could not stat processed file: %v", err)
	}
	if fileInfo.Size() == 0 {
		return "", fmt.Errorf("processed file is empty")
	}

	return processedFilePath, nil
}
