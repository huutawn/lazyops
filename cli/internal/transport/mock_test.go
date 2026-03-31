package transport

import (
	"context"
	"strings"
	"testing"
)

func TestMockTransportReturnsFixture(t *testing.T) {
	client := NewMockTransport(DefaultFixtures())

	response, err := client.Do(context.Background(), Request{
		Method: "GET",
		Path:   "/api/v1/projects",
	})
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}

	if response.FixtureName != "projects-list" {
		t.Fatalf("expected fixture %q, got %q", "projects-list", response.FixtureName)
	}
}

func TestMockTransportErrorsWhenFixtureMissing(t *testing.T) {
	client := NewMockTransport(DefaultFixtures())

	_, err := client.Do(context.Background(), Request{
		Method: "GET",
		Path:   "/missing",
	})
	if err == nil {
		t.Fatal("expected missing fixture error, got nil")
	}

	if !strings.Contains(err.Error(), "mock fixture not found") {
		t.Fatalf("expected missing fixture error message, got %v", err)
	}
}
