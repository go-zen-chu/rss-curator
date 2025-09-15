package openai

import (
	"context"
	"log/slog"
	"os"
	"testing"
)

// Test helper to create a test client
func createTestClient(t *testing.T) *Client {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError, // Reduce log noise during tests
	}))
	return NewClient(context.Background(), "test-api-key", logger)
}

func TestNewClient(t *testing.T) {
	tests := []struct {
		name   string
		apiKey string
	}{
		{
			name:   "If valid parameters given, it should create client instance",
			apiKey: "test-api-key",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

			got := NewClient(ctx, tt.apiKey, logger)

			if got == nil {
				t.Error("NewClient() returned nil")
				return
			}

			if got.apiKey != tt.apiKey {
				t.Errorf("Expected apiKey to be %s, got %s", tt.apiKey, got.apiKey)
			}

			if got.httpClient == nil {
				t.Error("Expected httpClient to be initialized")
			}

			if got.logger == nil {
				t.Error("Expected logger to be initialized")
			}
		})
	}
}