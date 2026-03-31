package transport

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"time"
)

type HTTPTransport struct {
	baseURL string
	client  *http.Client
}

func NewHTTPTransport(baseURL string, client *http.Client) *HTTPTransport {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	return &HTTPTransport{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  client,
	}
}

func (t *HTTPTransport) Mode() string {
	return "http"
}

func (t *HTTPTransport) Do(ctx context.Context, req Request) (Response, error) {
	httpReq, err := http.NewRequestWithContext(
		ctx,
		strings.ToUpper(strings.TrimSpace(req.Method)),
		t.baseURL+buildPath(req.Path, req.Query),
		bytes.NewReader(req.Body),
	)
	if err != nil {
		return Response{}, err
	}

	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}

	httpResp, err := t.client.Do(httpReq)
	if err != nil {
		return Response{}, err
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return Response{}, err
	}

	return Response{
		StatusCode: httpResp.StatusCode,
		Headers:    flattenHeaders(httpResp.Header),
		Body:       body,
	}, nil
}

func flattenHeaders(header http.Header) map[string]string {
	flattened := make(map[string]string, len(header))
	for key, values := range header {
		flattened[key] = strings.Join(values, ", ")
	}

	return flattened
}
