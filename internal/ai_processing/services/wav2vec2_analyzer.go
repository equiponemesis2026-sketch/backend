// Package services implementa el cliente del servicio de IA (Wav2Vec2).
package services

import (
	"context"
	"encoding/binary"
	"errors"
	"math"

	aiDomain "github.com/nemesis-project/api-nemesis/internal/ai_processing/domain"
)

// wav2Vec2Analyzer es un proxy local determinista del modelo Wav2Vec2 (SER).
// Extrae la energía RMS por ventanas como representación acústica y clasifica
// el nivel de tensión vocal. La interfaz VocalTensionAnalyzer permite sustituir
// esta implementación por inferencia real (ONNX/servicio externo) sin tocar
// los consumidores.
type wav2Vec2Analyzer struct {
	criticalThreshold  float64
	emotionalThreshold float64
}

// NewWav2Vec2Analyzer crea un analizador con umbrales configurables.
// - criticalThreshold: a partir de este stress_score se marca distress.
// - emotionalThreshold: umbral intermedio para separar tensión leve de estrés.
func NewWav2Vec2Analyzer(criticalThreshold float64, emotionalThreshold float64) aiDomain.VocalTensionAnalyzer {
	if criticalThreshold <= 0 {
		criticalThreshold = 0.85
	}
	if emotionalThreshold <= 0 {
		emotionalThreshold = 0.6
	}
	return &wav2Vec2Analyzer{
		criticalThreshold:  criticalThreshold,
		emotionalThreshold: emotionalThreshold,
	}
}

// Analyze procesa el audio (wav o pcm 16-bit mono) y devuelve el resultado
// de tensión vocal.
func (a *wav2Vec2Analyzer) Analyze(_ context.Context, format string, audio []byte) (aiDomain.VocalTensionResult, error) {
	pcm, err := decodeAudio(format, audio)
	if err != nil {
		return aiDomain.VocalTensionResult{}, err
	}
	if len(pcm) == 0 {
		return aiDomain.VocalTensionResult{}, errors.New("empty audio buffer")
	}

	window := 512
	windEnergy := windowEnergy(pcm, window)

	meanRMS, peakRMS := energyStats(windEnergy)
	score := normalize(meanRMS)
	confidence := computeConfidence(peakRMS, meanRMS)
	emotion := classifyEmotion(score, peakRMS, meanRMS, a.emotionalThreshold)

	return aiDomain.VocalTensionResult{
		ChunkIndex:       0,
		StressScore:      score,
		DistressDetected: score > a.criticalThreshold,
		Confidence:       confidence,
		Embeddings:       embeddings(windEnergy),
		PrimaryEmotion:   emotion,
	}, nil
}

// decodeAudio devuelve muestras PCM 16-bit (mono) a partir de wav o pcm crudo.
func decodeAudio(format string, audio []byte) ([]int16, error) {
	switch format {
	case "wav", "":
		return decodeWAV(audio)
	case "pcm":
		if len(audio)%2 != 0 {
			return nil, errors.New("pcm buffer must be 16-bit aligned")
		}
		out := make([]int16, len(audio)/2)
		for i := range out {
			out[i] = int16(binary.LittleEndian.Uint16(audio[i*2:]))
		}
		return out, nil
	case "opus":
		return nil, errors.New("opus codec is not supported yet")
	default:
		return nil, errors.New("unsupported audio format")
	}
}

// decodeWAV extrae las muestras del chunk `data` de un archivo RIFF/WAVE.
func decodeWAV(audio []byte) ([]int16, error) {
	if len(audio) < 44 {
		return nil, errors.New("wav header too short")
	}
	if string(audio[0:4]) != "RIFF" || string(audio[8:12]) != "WAVE" {
		return nil, errors.New("invalid wav header")
	}
	// El resto del archivo son chunks; buscar "data".
	for pos := 12; pos+8 <= len(audio); {
		chunkID := string(audio[pos : pos+4])
		size := int(binary.LittleEndian.Uint32(audio[pos+4 : pos+8]))
		dataStart := pos + 8
		if chunkID == "data" {
			end := dataStart + size
			if end > len(audio) {
				end = len(audio)
			}
			raw := audio[dataStart:end]
			if len(raw)%2 != 0 {
				raw = raw[:len(raw)-1]
			}
			out := make([]int16, len(raw)/2)
			for i := range out {
				out[i] = int16(binary.LittleEndian.Uint16(raw[i*2:]))
			}
			return out, nil
		}
		// Paddings de 1 byte para chunks impares.
		step := size
		if step%2 != 0 {
			step++
		}
		pos = dataStart + step
	}
	return nil, errors.New("wav has no data chunk")
}

// windowEnergy calcula la energía RMS de cada ventana.
func windowEnergy(pcm []int16, window int) []float64 {
	if window < 1 {
		window = 1
	}
	n := (len(pcm) + window - 1) / window
	energy := make([]float64, 0, n)
	for start := 0; start < len(pcm); start += window {
		end := start + window
		if end > len(pcm) {
			end = len(pcm)
		}
		var sum float64
		for _, s := range pcm[start:end] {
			v := float64(s)
			sum += v * v
		}
		energy = append(energy, math.Sqrt(sum/float64(end-start)))
	}
	return energy
}

// energyStats devuelve el RMS medio y el pico de energía.
func energyStats(energy []float64) (mean float64, peak float64) {
	var sum float64
	for _, e := range energy {
		sum += e
		if e > peak {
			peak = e
		}
	}
	mean = sum / float64(len(energy))
	return mean, peak
}

// normalize escala el RMS medio (16-bit: amplitud máxima 32767) a 0..1.
func normalize(rms float64) float64 {
	maxAmp := float64(32767)
	score := rms / maxAmp
	if score > 1 {
		score = 1
	}
	return round4(score)
}

// computeConfidence modela la confianza según la consistencia de la señal.
func computeConfidence(peak float64, mean float64) float64 {
	if peak <= 0 {
		return 0
	}
	ratio := mean / peak
	base := 0.5 + 0.5*ratio
	if base > 0.95 {
		base = 0.95
	}
	return round4(base + 0.05)
}

// classifyEmotion mapea energía y variabilidad a una emoción primaria.
func classifyEmotion(score float64, peak float64, mean float64, emotionalThreshold float64) string {
	switch {
	case score <= 0.15:
		return "neutral"
	case score > emotionalThreshold && peak/mean >= 2.5:
		// Picos altos y variables: gritos/sobresalto.
		return "fear"
	case score > emotionalThreshold:
		return "anger"
	default:
		return "stress"
	}
}

// embeddings produce un vector numérico de longitud fija a partir de la
// energía de las ventanas (representación acústica del chunk).
func embeddings(energy []float64) []float64 {
	const bins = 8
	out := make([]float64, bins)
	if len(energy) == 0 {
		return out
	}
	perBin := float64(len(energy)) / float64(bins)
	for i := 0; i < bins; i++ {
		start := int(float64(i) * perBin)
		end := int(float64(i+1) * perBin)
		if end < len(energy)+1 && end > start {
			var sum float64
			for _, e := range energy[start:end] {
				sum += e
			}
			out[i] = round4(sum / float64(end-start))
		}
	}
	return out
}

func round4(v float64) float64 {
	return math.Round(v*10000) / 10000
}
