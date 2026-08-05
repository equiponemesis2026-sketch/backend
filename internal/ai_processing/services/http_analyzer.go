package services

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	aiDomain "github.com/nemesis-project/api-nemesis/internal/ai_processing/domain"
)

var (
	// ErrUnsupportedAudioFormat indica un formato de audio que no se puede
	// normalizar para el microservicio de IA.
	ErrUnsupportedAudioFormat = errors.New("unsupported audio format")
	// ErrAIResponse indica una respuesta inválida o error del microservicio.
	ErrAIResponse = errors.New("ai service error")
)

// aiServiceResponse es el contrato con el microservicio Python (nemesis-ai).
type aiServiceResponse struct {
	StressScore float64 `json:"stress_score"`
	Emotion     string  `json:"emotion"`
	Confidence  float64 `json:"confidence"`
	IsDistress  bool    `json:"is_distress"`
}

// httpAnalyzer implementa VocalTensionAnalyzer delegando la inferencia al
// microservicio de IA (FastAPI) vía HTTP. Es best-effort: un fallo del
// servicio se propaga como error para que el consumidor (pool de audio) lo
// registre sin bloquear la ingesta.
type httpAnalyzer struct {
	client             *http.Client
	url                string
	criticalThreshold  float64
	emotionalThreshold float64
}

// NewWav2Vec2Analyzer crea el analizador que habla con el microservicio de
// IA en `url`. `timeout` limita cada llamada; los umbrales (0..1) determinan
// cuándo un score se considera distress. El nombre se conserva para no
// romper el cableado previo del módulo de audio.
func NewWav2Vec2Analyzer(url string, timeout time.Duration, criticalThreshold float64, emotionalThreshold float64) aiDomain.VocalTensionAnalyzer {
	if criticalThreshold <= 0 {
		criticalThreshold = 0.85
	}
	if emotionalThreshold <= 0 {
		emotionalThreshold = 0.6
	}
	return &httpAnalyzer{
		client:             &http.Client{Timeout: timeout},
		url:                url,
		criticalThreshold:  criticalThreshold,
		emotionalThreshold: emotionalThreshold,
	}
}

// Analyze normaliza el audio a WAV y lo envía al microservicio de IA. El
// score de estrés lo calcula Python; la decisión de distress (umbral) la
// aplica Go con la configuración del backend.
func (a *httpAnalyzer) Analyze(ctx context.Context, format string, audio []byte) (aiDomain.VocalTensionResult, error) {
	payload, err := prepareAudio(format, audio)
	if err != nil {
		return aiDomain.VocalTensionResult{}, err
	}
	if len(payload) == 0 {
		return aiDomain.VocalTensionResult{}, errors.New("empty audio buffer")
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "chunk.wav")
	if err != nil {
		return aiDomain.VocalTensionResult{}, fmt.Errorf("create multipart part: %w", err)
	}
	if _, err := part.Write(payload); err != nil {
		return aiDomain.VocalTensionResult{}, fmt.Errorf("write audio payload: %w", err)
	}
	if err := writer.Close(); err != nil {
		return aiDomain.VocalTensionResult{}, fmt.Errorf("close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.url+"/analyze", body)
	if err != nil {
		return aiDomain.VocalTensionResult{}, fmt.Errorf("build ai request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := a.client.Do(req)
	if err != nil {
		return aiDomain.VocalTensionResult{}, fmt.Errorf("%w: %v", ErrAIResponse, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return aiDomain.VocalTensionResult{}, fmt.Errorf("%w: status %d: %s", ErrAIResponse, resp.StatusCode, string(msg))
	}

	var out aiServiceResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return aiDomain.VocalTensionResult{}, fmt.Errorf("invalid ai response: %w", err)
	}

	return aiDomain.VocalTensionResult{
		StressScore:      out.StressScore,
		DistressDetected: out.StressScore >= a.criticalThreshold,
		Confidence:       out.Confidence,
		PrimaryEmotion:   out.Emotion,
	}, nil
}

// prepareAudio normaliza el chunk a un WAV válido para el microservicio. El
// PCM crudo (16-bit LE mono) se envuelve en un encabezado RIFF/WAVE de 16 kHz;
// el WAV se pasa tal cual. Opus aún no es compatible.
func prepareAudio(format string, audio []byte) ([]byte, error) {
	switch format {
	case "", "wav":
		return audio, nil
	case "pcm":
		return encodeWAV(audio), nil
	case "opus":
		return nil, fmt.Errorf("%w: opus", ErrUnsupportedAudioFormat)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedAudioFormat, format)
	}
}

// encodeWAV envuelve muestras PCM 16-bit mono en un encabezado RIFF/WAVE a
// 16 kHz (el sample rate que exige Wav2Vec2).
func encodeWAV(pcm []byte) []byte {
	if len(pcm)%2 != 0 {
		pcm = pcm[:len(pcm)-1]
	}
	header := make([]byte, 44)
	copy(header[0:4], "RIFF")
	binary.LittleEndian.PutUint32(header[4:8], uint32(36+len(pcm)))
	copy(header[8:12], "WAVE")
	copy(header[12:16], "fmt ")
	binary.LittleEndian.PutUint32(header[16:20], 16)
	binary.LittleEndian.PutUint16(header[20:22], 1) // PCM
	binary.LittleEndian.PutUint16(header[22:24], 1) // mono
	binary.LittleEndian.PutUint32(header[24:28], 16000)
	binary.LittleEndian.PutUint32(header[28:32], 32000)
	binary.LittleEndian.PutUint16(header[32:34], 2)
	binary.LittleEndian.PutUint16(header[34:36], 16)
	copy(header[36:40], "data")
	binary.LittleEndian.PutUint32(header[40:44], uint32(len(pcm)))
	return append(header, pcm...)
}
