package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

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
	const maxMemory = 10 << 20 // 10 MB (in Bytes)
	if r.ParseMultipartForm(maxMemory) != nil {
		respondWithError(w, http.StatusBadRequest, "Could not parse the form file :(", err)
		return
	}

	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Could not parse the form file :(", err)
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")

	videoMetaData, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Could not read database", err)
		return
	}
	if videoMetaData.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "You not authorized to edit this video", nil)
		return
	}

	imageDataBytes, err := io.ReadAll(file)
	randomID := make([]byte, 32)
	rand.Read(randomID)
	thumbnailID := base64.RawURLEncoding.EncodeToString(randomID)

	videoThumbnails[videoID] = thumbnail{
		data:      imageDataBytes,
		mediaType: contentType,
	}
	//in_memory: thumbnailURL := fmt.Sprintf("http://localhost:%v/api/thumbnails/%v", cfg.port, videoID)
	//sql text: imageData_base64 := base64.StdEncoding.EncodeToString(imageDataBytes)
	//          dataURL := fmt.Sprintf("data:%s;base64,%s", contentType, imageData_base64)
	//file system:
	mediatype, _, err := mime.ParseMediaType(contentType)
	if (mediatype != "image/jpeg" && mediatype != "image/png") || err != nil {
		respondWithError(w, http.StatusBadRequest, "Thumbnail must be png or jpeg", err)
		return
	}
	file_ext := strings.Split(mediatype, "/")[1]
	img_path := filepath.Join(cfg.assetsRoot,
		// videoID.String()+ // replaced with thumbnailID
		thumbnailID+
			"."+strings.TrimSpace(file_ext))

	img_writer, err := os.Create(img_path)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Error when creating thumbnail file", err)
		return
	}
	defer img_writer.Close()
	file.Seek(0, io.SeekStart)
	_, err = io.Copy(img_writer, file)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Error copying thumbnail file", err)
		return
	}
	thumbnailURL := fmt.Sprintf("http://localhost:%v/assets/%v.%v",
		cfg.port,
		// videoID,
		thumbnailID,
		file_ext)

	videoMetaData.ThumbnailURL = &thumbnailURL
	if err := cfg.db.UpdateVideo(videoMetaData); err != nil {
		respondWithError(w, http.StatusBadRequest, "Could not read database", err)
		return
	}
	respondWithJSON(w, http.StatusOK, videoMetaData)
}
