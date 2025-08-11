package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	const uploadLimit = 1 << 30
	r.Body = http.MaxBytesReader(w, r.Body, uploadLimit)

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

	fmt.Println("uploading video", videoID, "by user", userID)

	videoMetaData, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Database error", err)
		return
	}
	if videoMetaData.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "You not authorized to edit this video", nil)
		return
	}

	file, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't parse the video file", err)
		return
	}
	defer file.Close()

	mediaType, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if mediaType != "video/mp4" || err != nil {
		respondWithError(w, http.StatusBadRequest, "Upload must be a mp4 video", err)
		return
	}

	tmpFile, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't open upload file", err)
		return
	}
	defer os.Remove(tmpFile.Name()) // clean up
	defer tmpFile.Close()           // defer is LIFO, order: close then remove

	if _, err := io.Copy(tmpFile, file); err != nil {
		respondWithError(w, http.StatusBadRequest, "Error saving tmpFile", err)
		return
	}
	tmpFile.Seek(0, io.SeekStart)

	aspectRatio, err := getVideoAspectRatio(tmpFile.Name())
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't get aspect ratio", err)
		return
	}

	randomID := make([]byte, 32)
	rand.Read(randomID)
	fileKey := aspectRatio + "/" + base64.RawURLEncoding.EncodeToString(randomID) + ".mp4"

	putObjInput := s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &fileKey,
		Body:        tmpFile,
		ContentType: &mediaType,
	}
	if _, err := cfg.s3Client.PutObject(r.Context(), &putObjInput); err != nil {
		respondWithError(w, http.StatusBadRequest, "Error putting object to s3 bucket", err)
		return
	}

	videoURL := fmt.Sprintf("https://%v.s3.%v.amazonaws.com/%v",
		cfg.s3Bucket,
		cfg.s3Region,
		fileKey)
	videoMetaData.VideoURL = &videoURL
	if err := cfg.db.UpdateVideo(videoMetaData); err != nil {
		respondWithError(w, http.StatusBadRequest, "Error updating video data", err)
		return
	}

	respondWithJSON(w, http.StatusOK, videoMetaData)
}
