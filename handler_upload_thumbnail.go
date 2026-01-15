package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	// TODO: implement the upload here
	const maxMemory = 10 << 20
	r.ParseMultipartForm(maxMemory)
	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	imageData, err := io.ReadAll(file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to read bytes from thumbnail", err)
		return
	}

	videoMetadata, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "video not found", err)
		return
	}

	if videoMetadata.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "access denied", err)
		return
	}

	imageString := base64.StdEncoding.EncodeToString(imageData)
	thumbnail_url := fmt.Sprintf("data:%s;base64,%s", contentType, imageString)
	// thumbnailUrl := fmt.Sprintf("http://localhost:%s/api/thumbnails/{%s}", cfg.port, videoID.String())
	videoMetadata.ThumbnailURL = &thumbnail_url
	videoMetadata.UpdatedAt = time.Now()
	if err = cfg.db.UpdateVideo(videoMetadata); err != nil {
		respondWithError(w, http.StatusInternalServerError, "couldn't add video thumbnail", err)
		return
	}
	respondWithJSON(w, http.StatusOK, videoMetadata)
}
