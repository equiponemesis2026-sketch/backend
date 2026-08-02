package http

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/nemesis-project/api-nemesis/internal/audio/domain"
	"github.com/nemesis-project/api-nemesis/internal/audio/usecase"
	"github.com/nemesis-project/api-nemesis/internal/infrastructure/middleware"
	"github.com/nemesis-project/api-nemesis/pkg/response"
)

// AudioHandler expone el endpoint de recepción de fragmentos de audio.
type AudioHandler struct {
	uc domain.AudioUseCase
}

func NewAudioHandler(uc domain.AudioUseCase) *AudioHandler {
	return &AudioHandler{uc: uc}
}

// StreamChunk recibe un fragmento de audio por JSON (base64) o multipart.
func (h *AudioHandler) StreamChunk(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)

	input, err := decodeChunkRequest(r)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.uc.StreamChunk(r.Context(), userID, input); err != nil {
		switch {
		case errors.Is(err, usecase.ErrAlertNotFound):
			response.WriteError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, usecase.ErrForbidden):
			response.WriteError(w, http.StatusForbidden, err.Error())
		case errors.Is(err, usecase.ErrDuplicateChunk):
			response.WriteError(w, http.StatusConflict, err.Error())
		case errors.Is(err, usecase.ErrUnsupportedFormat),
			errors.Is(err, usecase.ErrInvalidChunk),
			errors.Is(err, usecase.ErrAudioTooLarge):
			response.WriteError(w, http.StatusBadRequest, err.Error())
		default:
			response.WriteError(w, http.StatusInternalServerError, "Failed to store audio chunk")
		}
		return
	}

	response.WriteJSON(w, http.StatusAccepted, map[string]string{
		"status":  "success",
		"message": "Audio chunk accepted for analysis",
	})
}

// jsonChunkRequest es el DTO JSON (audio_data en base64).
type jsonChunkRequest struct {
	AlertID    string `json:"alert_id"`
	ChunkIndex int    `json:"chunk_index"`
	Format     string `json:"format"`
	AudioData  string `json:"audio_data"`
	DurationMS int    `json:"duration_ms"`
	Timestamp  int64  `json:"timestamp"`
}

func decodeChunkRequest(r *http.Request) (*domain.StreamChunkInput, error) {
	contentType := r.Header.Get("Content-Type")
	if len(contentType) >= 9 && contentType[:9] == "multipart" {
		return decodeMultipart(r)
	}
	return decodeJSON(r)
}

func decodeJSON(r *http.Request) (*domain.StreamChunkInput, error) {
	var req jsonChunkRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<20)).Decode(&req); err != nil {
		return nil, errors.New("invalid request body")
	}
	if req.AudioData == "" {
		return nil, errors.New("audio_data is required")
	}
	data, err := base64.StdEncoding.DecodeString(req.AudioData)
	if err != nil {
		return nil, errors.New("audio_data must be base64")
	}
	return &domain.StreamChunkInput{
		AlertID:    req.AlertID,
		ChunkIndex: req.ChunkIndex,
		Format:     domain.AudioFormat(req.Format),
		AudioData:  data,
		DurationMS: req.DurationMS,
		Timestamp:  req.Timestamp,
	}, nil
}

func decodeMultipart(r *http.Request) (*domain.StreamChunkInput, error) {
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		return nil, errors.New("invalid multipart form")
	}

	input := &domain.StreamChunkInput{
		AlertID:    r.FormValue("alert_id"),
		Format:     domain.AudioFormat(r.FormValue("format")),
		DurationMS: parsePositiveInt(r.FormValue("duration_ms")),
		Timestamp:  parseInt64(r.FormValue("timestamp")),
	}
	if v := r.FormValue("chunk_index"); v != "" {
		input.ChunkIndex = parseInt(v)
	}

	file, _, err := r.FormFile("audio_data")
	if err != nil {
		return nil, errors.New("audio_data file is required")
	}
	defer func() { _ = file.Close() }()

	data, err := io.ReadAll(io.LimitReader(file, 8<<20))
	if err != nil {
		return nil, errors.New("failed to read audio_data")
	}
	input.AudioData = data
	return input, nil
}

func parseInt(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func parsePositiveInt(s string) int {
	n := parseInt(s)
	if n < 0 {
		return 0
	}
	return n
}

func parseInt64(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}
