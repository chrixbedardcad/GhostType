package stt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"time"
)

// WhisperCloud transcribes audio via the OpenAI Whisper API.
// Used as a cloud fallback when Ghost Voice (local whisper.cpp) is not available.
type WhisperCloud struct {
	apiKey   string
	endpoint string
	model    string
	client   *http.Client
}

// NewWhisperCloud creates a Whisper API client.
func NewWhisperCloud(apiKey, endpoint, model string) *WhisperCloud {
	if endpoint == "" {
		endpoint = "https://api.openai.com/v1/audio/transcriptions"
	}
	if model == "" {
		model = "whisper-1"
	}
	return &WhisperCloud{
		apiKey:   apiKey,
		endpoint: endpoint,
		model:    model,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (w *WhisperCloud) Name() string { return "Whisper (OpenAI)" }

func (w *WhisperCloud) Transcribe(ctx context.Context, wavData []byte, language string) (string, error) {
	slog.Info("[stt] Whisper cloud transcription", "size", len(wavData), "language", language)

	// Build multipart request.
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("file", "recording.wav")
	if err != nil {
		return "", fmt.Errorf("multipart create: %w", err)
	}
	if _, err := part.Write(wavData); err != nil {
		return "", fmt.Errorf("multipart write: %w", err)
	}

	writer.WriteField("model", w.model)
	if language != "" {
		writer.WriteField("language", language)
	}
	writer.WriteField("response_format", "json")
	writer.Close()

	req, err := http.NewRequestWithContext(ctx, "POST", w.endpoint, &body)
	if err != nil {
		return "", fmt.Errorf("request create: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+w.apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := w.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("whisper API error %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	slog.Info("[stt] Whisper transcription complete", "text_len", len(result.Text))
	return result.Text, nil
}
