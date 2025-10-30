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

	err = r.ParseMultipartForm(maxMemory)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "uable to parse thimbnail", err)
		return
	}

	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "unable to form file", err)
		return
	}

	defer file.Close()

	ctype := header.Header.Get("Content-Type")
	if ctype == "" {
		respondWithError(w, http.StatusBadRequest, "no content type header", nil)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "unable to get video", err)
		return
	}

	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "user s not owner of video", nil)
		return
	}

	var ext string

	mediaType, _, err := mime.ParseMediaType(ctype)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "unable to parse content-type", err)
		return
	}
	if mediaType != "image/png" {
		if mediaType != "image/jpeg" {
			respondWithError(w, http.StatusBadRequest, "not a png/jpeg", nil)
			return
		}
	}
	if mediaType == "image/png" {
		ext = ".png"
	} else {
		ext = ".jpg"
	}

	tid := make([]byte, 32)

	rand.Read(tid)

	fname := base64.RawURLEncoding.EncodeToString(tid)

	filename := fname + ext

	fpath := filepath.Join(cfg.assetsRoot, filename)

	nfile, err := os.Create(fpath)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "unable to write thumbnail", err)
		return
	}

	defer nfile.Close()

	numBytes, err := io.Copy(nfile, file)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "unable to copy tumbnail", err)
		return
	}
	if numBytes == 0 {
		respondWithError(w, http.StatusBadRequest, "0 bytes copied", nil)
		return
	}

	tURL := "http://localhost:" + cfg.port + "/assets/" + filename

	video.ThumbnailURL = &tURL

	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "unable to update video in db", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
