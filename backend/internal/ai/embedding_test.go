package ai

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPEmbeddingClientRetriesAndSucceeds(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(`{"embedding":[0.1,0.2,0.3]}`))
	}))
	defer server.Close()

	client := NewHTTPEmbeddingClient(server.URL, "", "demo")
	client.MaxRetries = 1
	embedding, err := client.EmbedText("postgres timeout")
	if err != nil {
		t.Fatalf("expected retry success, got %v", err)
	}
	if embedding == "" {
		t.Fatal("expected embedding output")
	}
}
