package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-zen-chu/rss-curator/pkg/bluesky"
	"github.com/go-zen-chu/rss-curator/pkg/openai"
	"github.com/google/go-cmp/cmp"
)

// Test helper to create a test processor
func createTestProcessor(t *testing.T) (*RSSProcessor, string) {
	t.Helper()

	// Create temporary directory for test data
	tempDir := t.TempDir()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError, // Reduce log noise during tests
	}))

	config := Config{
		OpenAIAPIKey:      "test-api-key",
		BlueskyIdentifier: "test@example.com",
		BlueskyPassword:   "test-password",
		RSSFeeds:          []string{"http://example.com/rss"},
		DataDir:           tempDir,
		MaxArticlesPerRun: 5,
	}

	processor := &RSSProcessor{
		config:        config,
		logger:        logger,
		blueskyClient: bluesky.NewClient(context.Background(), logger),
		openaiClient:  openai.NewClient(context.Background(), config.OpenAIAPIKey, logger),
	}

	return processor, tempDir
}

// Test helper to create test articles
func createTestArticle(id string) Article {
	return Article{
		ID:              id,
		OriginalTitle:   "Test Article " + id,
		OriginalSummary: "This is a test summary for article " + id,
		Link:            "https://example.com/article/" + id,
		Published:       time.Now().Add(-time.Hour),
		ProcessedAt:     time.Now(),
		Source:          "Test Source",
		Language:        "en",
		RSSFeedURL:      "https://example.com/rss",
	}
}

func TestRSSProcessor_generateArticleID(t *testing.T) {
	tests := []struct {
		name      string
		link      string
		published time.Time
		want      string
	}{
		{
			name:      "If valid link and time given, it should generate consistent ID",
			link:      "https://example.com/article1",
			published: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			want:      "20240115-100000-6874747073",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor, _ := createTestProcessor(t)
			got := processor.generateArticleID(tt.link, tt.published)
			if got != tt.want {
				t.Errorf("generateArticleID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRSSProcessor_truncateText(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		maxLength int
		want      string
	}{
		{
			name:      "If text is shorter than max length given, it should return original text",
			text:      "Short text",
			maxLength: 20,
			want:      "Short text",
		},
		{
			name:      "If text is longer than max length given, it should truncate with ellipsis",
			text:      "This is a very long text that needs to be truncated",
			maxLength: 10,
			want:      "This is a ...",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor, _ := createTestProcessor(t)
			got := processor.truncateText(tt.text, tt.maxLength)
			if got != tt.want {
				t.Errorf("truncateText() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRSSProcessor_saveArticleToJSON(t *testing.T) {
	tests := []struct {
		name    string
		article Article
		wantErr bool
	}{
		{
			name:    "If valid article given, it should save to JSON file",
			article: createTestArticle("test001"),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor, tempDir := createTestProcessor(t)
			ctx := context.Background()

			err := processor.saveArticleToJSON(ctx, tt.article)
			if (err != nil) != tt.wantErr {
				t.Errorf("saveArticleToJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify file was created
				yearMonth := tt.article.Published.Format("2006/01")
				expectedPath := filepath.Join(tempDir, yearMonth, tt.article.ID+".json")

				if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
					t.Errorf("Expected file %s was not created", expectedPath)
					return
				}

				// Verify file content
				data, err := os.ReadFile(expectedPath)
				if err != nil {
					t.Errorf("Failed to read saved file: %v", err)
					return
				}

				var savedArticle Article
				if err := json.Unmarshal(data, &savedArticle); err != nil {
					t.Errorf("Failed to unmarshal saved article: %v", err)
					return
				}

				if diff := cmp.Diff(tt.article, savedArticle); diff != "" {
					t.Errorf("Saved article differs from original (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestRSSProcessor_loadExistingArticles(t *testing.T) {
	tests := []struct {
		name         string
		setupArticles []Article
		want         map[string]bool
		wantErr      bool
	}{
		{
			name: "If existing articles given, it should load article IDs",
			setupArticles: []Article{
				createTestArticle("article001"),
				createTestArticle("article002"),
			},
			want: map[string]bool{
				"article001": true,
				"article002": true,
			},
			wantErr: false,
		},
		{
			name:          "If no existing articles given, it should return empty map",
			setupArticles: []Article{},
			want:          map[string]bool{},
			wantErr:       false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor, _ := createTestProcessor(t)
			ctx := context.Background()

			// Setup test articles
			for _, article := range tt.setupArticles {
				if err := processor.saveArticleToJSON(ctx, article); err != nil {
					t.Fatalf("Failed to setup test article: %v", err)
				}
			}

			got, err := processor.loadExistingArticles(ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("loadExistingArticles() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("loadExistingArticles() differs (-want +got):\n%s", diff)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name    string
		envVars map[string]string
		wantErr bool
		setup   func()
		cleanup func()
	}{
		{
			name: "If all required environment variables given, it should load config successfully",
			envVars: map[string]string{
				"OPENAI_API_KEY":      "test-openai-key",
				"BLUESKY_IDENTIFIER":  "test@example.com",
				"BLUESKY_PASSWORD":    "test-password",
				"MAX_ARTICLES_PER_RUN": "15",
				"DATA_DIR":            "/tmp/test",
			},
			wantErr: false,
			setup: func() {
				os.Setenv("OPENAI_API_KEY", "test-openai-key")
				os.Setenv("BLUESKY_IDENTIFIER", "test@example.com")
				os.Setenv("BLUESKY_PASSWORD", "test-password")
				os.Setenv("MAX_ARTICLES_PER_RUN", "15")
				os.Setenv("DATA_DIR", "/tmp/test")
			},
			cleanup: func() {
				os.Unsetenv("OPENAI_API_KEY")
				os.Unsetenv("BLUESKY_IDENTIFIER")
				os.Unsetenv("BLUESKY_PASSWORD")
				os.Unsetenv("MAX_ARTICLES_PER_RUN")
				os.Unsetenv("DATA_DIR")
			},
		},
		{
			name:    "If required environment variables missing, it should return error",
			envVars: map[string]string{},
			wantErr: true,
			setup:   func() {},
			cleanup: func() {},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			defer tt.cleanup()

			got, err := loadConfig()
			if (err != nil) != tt.wantErr {
				t.Errorf("loadConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if got.OpenAIAPIKey != "test-openai-key" {
					t.Errorf("Expected OpenAIAPIKey to be 'test-openai-key', got %s", got.OpenAIAPIKey)
				}
				if got.BlueskyIdentifier != "test@example.com" {
					t.Errorf("Expected BlueskyIdentifier to be 'test@example.com', got %s", got.BlueskyIdentifier)
				}
				if got.MaxArticlesPerRun != 15 {
					t.Errorf("Expected MaxArticlesPerRun to be 15, got %d", got.MaxArticlesPerRun)
				}
				if got.DataDir != "/tmp/test" {
					t.Errorf("Expected DataDir to be '/tmp/test', got %s", got.DataDir)
				}
			}
		})
	}
}

func TestNewRSSProcessor(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{
			name: "If valid config given, it should create processor with all dependencies",
			config: Config{
				OpenAIAPIKey:      "test-api-key",
				BlueskyIdentifier: "test@example.com",
				BlueskyPassword:   "test-password",
				RSSFeeds:          []string{"http://example.com/rss"},
				DataDir:           "/tmp/test",
				MaxArticlesPerRun: 10,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			got := NewRSSProcessor(ctx, tt.config)

			if got == nil {
				t.Error("NewRSSProcessor() returned nil")
				return
			}

			if got.config.OpenAIAPIKey != tt.config.OpenAIAPIKey {
				t.Errorf("Expected OpenAIAPIKey to be %s, got %s", tt.config.OpenAIAPIKey, got.config.OpenAIAPIKey)
			}

			if got.blueskyClient == nil {
				t.Error("Expected blueskyClient to be initialized")
			}

			if got.openaiClient == nil {
				t.Error("Expected openaiClient to be initialized")
			}

			if got.logger == nil {
				t.Error("Expected logger to be initialized")
			}
		})
	}
}