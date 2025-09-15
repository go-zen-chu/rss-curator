package bluesky

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
	return NewClient(context.Background(), logger)
}

func TestNewClient(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "If valid parameters given, it should create client instance",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

			got := NewClient(ctx, logger)

			if got == nil {
				t.Error("NewClient() returned nil")
				return
			}

			if got.httpClient == nil {
				t.Error("Expected httpClient to be initialized")
			}

			if got.logger == nil {
				t.Error("Expected logger to be initialized")
			}

			if got.session != nil {
				t.Error("Expected session to be nil initially")
			}
		})
	}
}

func TestClient_CreatePost_WithoutSession(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		wantErr bool
	}{
		{
			name:    "If no session given, it should return error",
			text:    "Test post content",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := createTestClient(t)

			ctx := context.Background()
			err := client.CreatePost(ctx, tt.text)

			if (err != nil) != tt.wantErr {
				t.Errorf("CreatePost() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}