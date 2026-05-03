package contract

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
)

const maxUploadBytes = 10 * 1024 * 1024

type httpResponse struct {
	StatusCode  int
	Header      http.Header
	Body        []byte
	ContentType string
}

func (r *Runner) do(ctx context.Context, method, path string, body io.Reader, headers map[string]string) (*httpResponse, error) {
	req, err := http.NewRequestWithContext(ctx, method, r.URL(path), body)
	if err != nil {
		return nil, err
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return &httpResponse{
		StatusCode:  resp.StatusCode,
		Header:      resp.Header.Clone(),
		Body:        data,
		ContentType: resp.Header.Get("Content-Type"),
	}, nil
}

func multipartBody(fieldName, fileName, contentType string, data []byte) (io.Reader, string, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, fieldName, fileName))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	header.Set("Content-Type", contentType)
	part, err := writer.CreatePart(header)
	if err != nil {
		return nil, "", err
	}
	if _, err := part.Write(data); err != nil {
		return nil, "", err
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return &buf, writer.FormDataContentType(), nil
}

func expectStatus(resp *httpResponse, allowed ...int) error {
	for _, code := range allowed {
		if resp.StatusCode == code {
			return nil
		}
	}
	return fmt.Errorf("unexpected status %d, want one of %v; body=%s", resp.StatusCode, allowed, trimBody(resp.Body))
}

func expectJSONError(resp *httpResponse, status int) error {
	if err := expectStatus(resp, status); err != nil {
		return err
	}
	return validateJSONError(resp.Body)
}

func validateJSONError(body []byte) error {
	var payload struct {
		Error struct {
			Code    string          `json:"code"`
			Message string          `json:"message"`
			Details json.RawMessage `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("error response is not JSON: %w; body=%s", err, trimBody(body))
	}
	if payload.Error.Code == "" {
		return fmt.Errorf("error.code is required; body=%s", trimBody(body))
	}
	if payload.Error.Message == "" {
		return fmt.Errorf("error.message is required; body=%s", trimBody(body))
	}
	return nil
}

func decodeJSON(body []byte, target any) error {
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("decode JSON: %w; body=%s", err, trimBody(body))
	}
	return nil
}

func trimBody(body []byte) string {
	text := strings.TrimSpace(string(body))
	if len(text) > 300 {
		return text[:300] + "..."
	}
	return text
}
