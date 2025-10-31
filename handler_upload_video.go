package main

import (
	"crypto/rand"
	"encoding/base64"
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
	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "unable to get video", err)
		return
	}

	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "user s not owner of video", nil)
		return
	}

	const maxMemory = 10 << 20

	err = r.ParseMultipartForm(maxMemory)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "uable to parse thimbnail", err)
		return
	}

	file, header, err := r.FormFile("video")
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
	mediaType, _, err := mime.ParseMediaType(ctype)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "unable to parse content-type", err)
		return
	}
	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "file not an .mp4", nil)
		return
	}
	ext := ".mp4"
	tFile, err := os.CreateTemp("", "tubley-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "unable to create temp file", err)
		return
	}
	defer os.Remove(tFile.Name())
	defer tFile.Close()
	bytesCopied, err := io.Copy(tFile, file)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "error copying file", err)
		return
	}
	if bytesCopied == 0 {
		respondWithError(w, http.StatusBadRequest, "o bytes written", nil)
		return
	}
	ret, err := tFile.Seek(0, io.SeekStart)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "unable to move temp file pointer", err)
		return
	}
	if ret != 0 {
		respondWithError(w, http.StatusBadRequest, "temp file pointer not reset to 0", nil)
		return
	}
	tid := make([]byte, 32)
	rand.Read(tid)

	fname := base64.RawURLEncoding.EncodeToString(tid)

	filename := fname + ext

	params := s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &filename,
		Body:        tFile,
		ContentType: &ctype,
	}

	_, err = cfg.s3Client.PutObject(r.Context(), &params)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "unable to put the file in the bucket", err)
		return
	}
	videoURL := "https://" + cfg.s3Bucket + ".s3." + cfg.s3Region + ".amazonaws.com/" + filename
	video.VideoURL = &videoURL
	cfg.db.UpdateVideo(video)
}
