package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-zen-chu/rss-curator/pkg/bluesky"
	"github.com/go-zen-chu/rss-curator/pkg/openai"
	"github.com/mmcdole/gofeed"
)

// Article represents a processed news article
type Article struct {
	ID              string    `json:"id"`
	OriginalTitle   string    `json:"original_title"`
	TranslatedTitle string    `json:"translated_title"`
	OriginalSummary string    `json:"original_summary"`
	JapaneseSummary string    `json:"japanese_summary"`
	Link            string    `json:"link"`
	Published       time.Time `json:"published"`
	ProcessedAt     time.Time `json:"processed_at"`
	Source          string    `json:"source"`
	Language        string    `json:"language"`
	RSSFeedURL      string    `json:"rss_feed_url"`
}

// Config holds application configuration
type Config struct {
	OpenAIAPIKey      string
	BlueskyIdentifier string
	BlueskyPassword   string
	RSSFeeds          []string
	DataDir           string
	MaxArticlesPerRun int
}

// RSSProcessor handles RSS feed processing and Bluesky posting
type RSSProcessor struct {
	config        Config
	httpClient    *http.Client
	logger        *slog.Logger
	blueskyClient *bluesky.Client
	openaiClient  *openai.Client
}

// NewRSSProcessor creates a new RSSProcessor instance
func NewRSSProcessor(ctx context.Context, config Config) *RSSProcessor {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	return &RSSProcessor{
		config:        config,
		httpClient:    &http.Client{Timeout: 30 * time.Second},
		logger:        logger,
		blueskyClient: bluesky.NewClient(ctx, logger),
		openaiClient:  openai.NewClient(ctx, config.OpenAIAPIKey, logger),
	}
}

// FetchAndProcessRSSFeeds fetches and processes RSS feeds
func (rp *RSSProcessor) FetchAndProcessRSSFeeds(ctx context.Context) error {
	// Ensure data directory exists
	if err := os.MkdirAll(rp.config.DataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Load existing articles to avoid duplicates
	existingArticles, err := rp.loadExistingArticles(ctx)
	if err != nil {
		rp.logger.WarnContext(ctx, "Failed to load existing articles", slog.String("error", err.Error()))
		existingArticles = make(map[string]bool)
	}

	var allNewArticles []Article

	for _, feedURL := range rp.config.RSSFeeds {
		rp.logger.InfoContext(ctx, "Processing RSS feed", slog.String("url", feedURL))

		articles, err := rp.fetchRSSFeed(ctx, feedURL)
		if err != nil {
			rp.logger.WarnContext(ctx, "Failed to fetch RSS feed",
				slog.String("url", feedURL),
				slog.String("error", err.Error()))
			continue
		}

		// Filter out existing articles
		var newArticles []Article
		for _, article := range articles {
			if !existingArticles[article.ID] {
				newArticles = append(newArticles, article)
			}
		}

		rp.logger.InfoContext(ctx, "Found new articles",
			slog.String("feed_url", feedURL),
			slog.Int("new_count", len(newArticles)),
			slog.Int("total_count", len(articles)))

		allNewArticles = append(allNewArticles, newArticles...)
	}

	// Sort by publication date (oldest first for processing)
	sort.Slice(allNewArticles, func(i, j int) bool {
		return allNewArticles[i].Published.Before(allNewArticles[j].Published)
	})

	// Limit articles per run
	if len(allNewArticles) > rp.config.MaxArticlesPerRun {
		allNewArticles = allNewArticles[:rp.config.MaxArticlesPerRun]
	}

	if len(allNewArticles) == 0 {
		rp.logger.InfoContext(ctx, "No new articles to process")
		return nil
	}

	rp.logger.InfoContext(ctx, "Processing new articles", slog.Int("count", len(allNewArticles)))

	// Process each article
	for _, article := range allNewArticles {
		processedArticle, err := rp.processArticle(ctx, article)
		if err != nil {
			rp.logger.ErrorContext(ctx, "Failed to process article",
				slog.String("article_id", article.ID),
				slog.String("error", err.Error()))
			continue
		}

		// Save to JSON file
		if err := rp.saveArticleToJSON(ctx, processedArticle); err != nil {
			rp.logger.ErrorContext(ctx, "Failed to save article",
				slog.String("article_id", processedArticle.ID),
				slog.String("error", err.Error()))
			continue
		}

		// Post to Bluesky
		if err := rp.postToBluesky(ctx, processedArticle); err != nil {
			rp.logger.ErrorContext(ctx, "Failed to post to Bluesky",
				slog.String("article_id", processedArticle.ID),
				slog.String("error", err.Error()))
		}

		// Add delay to avoid rate limiting
		time.Sleep(2 * time.Second)
	}

	return nil
}

// fetchRSSFeed fetches articles from a single RSS feed
func (rp *RSSProcessor) fetchRSSFeed(ctx context.Context, feedURL string) ([]Article, error) {
	fp := gofeed.NewParser()
	fp.Client = &http.Client{Timeout: 15 * time.Second}

	feed, err := fp.ParseURL(feedURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse RSS feed: %w", err)
	}

	sourceName := feed.Title
	if sourceName == "" {
		sourceName = feedURL
	}

	var articles []Article
	cutoffTime := time.Now().Add(-24 * time.Hour) // Only process articles from last 24 hours

	for _, item := range feed.Items {
		// Parse publication date
		published := time.Now()
		if item.PublishedParsed != nil {
			published = *item.PublishedParsed
		} else if item.UpdatedParsed != nil {
			published = *item.UpdatedParsed
		}

		// Skip old articles
		if published.Before(cutoffTime) {
			continue
		}

		// Generate unique ID
		articleID := rp.generateArticleID(item.Link, published)

		summary := item.Description
		if summary == "" {
			summary = item.Content
		}

		article := Article{
			ID:              articleID,
			OriginalTitle:   item.Title,
			OriginalSummary: rp.truncateText(summary, 1000),
			Link:            item.Link,
			Published:       published,
			ProcessedAt:     time.Now(),
			Source:          sourceName,
			RSSFeedURL:      feedURL,
		}

		articles = append(articles, article)
	}

	return articles, nil
}

// processArticle translates and summarizes an article using OpenAI
func (rp *RSSProcessor) processArticle(ctx context.Context, article Article) (Article, error) {
	result, err := rp.openaiClient.TranslateArticle(ctx, article.OriginalTitle, article.OriginalSummary)
	if err != nil {
		return article, fmt.Errorf("failed to translate article: %w", err)
	}

	// Update article with translation results
	article.TranslatedTitle = result.TranslatedTitle
	article.JapaneseSummary = result.JapaneseSummary
	article.Language = result.Language

	rp.logger.InfoContext(ctx, "Article processed successfully",
		slog.String("article_id", article.ID),
		slog.String("language", article.Language))

	return article, nil
}

// saveArticleToJSON saves article to JSON file
func (rp *RSSProcessor) saveArticleToJSON(ctx context.Context, article Article) error {
	// Create year/month directory structure
	yearMonth := article.Published.Format("2006/01")
	dirPath := filepath.Join(rp.config.DataDir, yearMonth)

	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Generate filename
	filename := fmt.Sprintf("%s.json", article.ID)
	filePath := filepath.Join(dirPath, filename)

	// Save to JSON file
	jsonData, err := json.MarshalIndent(article, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal article: %w", err)
	}

	if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	rp.logger.InfoContext(ctx, "Article saved to JSON",
		slog.String("article_id", article.ID),
		slog.String("file_path", filePath))

	return nil
}

// postToBluesky posts article summary to Bluesky
func (rp *RSSProcessor) postToBluesky(ctx context.Context, article Article) error {
	// Login to Bluesky if not already done
	if err := rp.blueskyClient.Login(ctx, rp.config.BlueskyIdentifier, rp.config.BlueskyPassword); err != nil {
		return fmt.Errorf("failed to login to Bluesky: %w", err)
	}

	// Create post text (max 300 characters for Bluesky)
	postText := fmt.Sprintf("%s\n\n%s", article.TranslatedTitle, article.JapaneseSummary)
	if len(postText) > 250 {
		// Truncate summary if too long
		availableChars := 250 - len(article.TranslatedTitle) - 3 // 3 for newlines
		if availableChars > 0 {
			summary := []rune(article.JapaneseSummary)
			if len(summary) > availableChars {
				postText = fmt.Sprintf("%s\n\n%s...", article.TranslatedTitle, string(summary[:availableChars-3]))
			}
		} else {
			postText = article.TranslatedTitle
		}
	}

	postText += fmt.Sprintf("\n\n🔗 %s", article.Link)

	if err := rp.blueskyClient.CreatePost(ctx, postText); err != nil {
		return fmt.Errorf("failed to post to Bluesky: %w", err)
	}

	rp.logger.InfoContext(ctx, "Posted to Bluesky successfully",
		slog.String("article_id", article.ID))

	return nil
}

// loadExistingArticles loads existing article IDs to avoid duplicates
func (rp *RSSProcessor) loadExistingArticles(ctx context.Context) (map[string]bool, error) {
	existing := make(map[string]bool)

	err := filepath.Walk(rp.config.DataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue walking even if there's an error
		}

		if !strings.HasSuffix(path, ".json") {
			return nil
		}

		filename := filepath.Base(path)
		articleID := strings.TrimSuffix(filename, ".json")
		existing[articleID] = true

		return nil
	})

	if err != nil {
		return nil, err
	}

	rp.logger.InfoContext(ctx, "Loaded existing articles", slog.Int("count", len(existing)))
	return existing, nil
}

// generateArticleID generates a unique ID for an article
func (rp *RSSProcessor) generateArticleID(link string, published time.Time) string {
	// Use URL hash + timestamp for uniqueness
	timestamp := published.Format("20060102-150405")
	linkHash := fmt.Sprintf("%x", []byte(link))
	if len(linkHash) > 10 {
		linkHash = linkHash[:10]
	}
	return fmt.Sprintf("%s-%s", timestamp, linkHash)
}

// truncateText truncates text to specified length
func (rp *RSSProcessor) truncateText(text string, maxLength int) string {
	// Simple HTML tag removal
	text = strings.ReplaceAll(text, "<", "&lt;")
	text = strings.ReplaceAll(text, ">", "&gt;")

	runes := []rune(text)
	if len(runes) <= maxLength {
		return text
	}
	return string(runes[:maxLength]) + "..."
}

// loadConfig loads configuration from environment variables
func loadConfig() (Config, error) {
	config := Config{
		DataDir:           "data/articles",
		MaxArticlesPerRun: 10,
		RSSFeeds: []string{
			"https://feeds.feedburner.com/itmedia/news", // ITmedia NEWS
			"https://www3.nhk.or.jp/rss/news/cat0.xml",  // NHK ニュース
			"https://rss.cnn.com/rss/edition.rss",       // CNN
			"https://feeds.feedburner.com/TechCrunch",   // TechCrunch
			"https://www.reddit.com/r/programming/.rss", // Reddit Programming
			"https://hnrss.org/frontpage",               // Hacker News
		},
	}

	config.OpenAIAPIKey = os.Getenv("OPENAI_API_KEY")
	if config.OpenAIAPIKey == "" {
		return config, fmt.Errorf("OPENAI_API_KEY environment variable is required")
	}

	config.BlueskyIdentifier = os.Getenv("BLUESKY_IDENTIFIER")
	if config.BlueskyIdentifier == "" {
		return config, fmt.Errorf("BLUESKY_IDENTIFIER environment variable is required")
	}

	config.BlueskyPassword = os.Getenv("BLUESKY_PASSWORD")
	if config.BlueskyPassword == "" {
		return config, fmt.Errorf("BLUESKY_PASSWORD environment variable is required")
	}

	// Override defaults with environment variables
	if val := os.Getenv("MAX_ARTICLES_PER_RUN"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			config.MaxArticlesPerRun = parsed
		}
	}

	if val := os.Getenv("DATA_DIR"); val != "" {
		config.DataDir = val
	}

	return config, nil
}

func main() {
	ctx := context.Background()

	// Load configuration
	config, err := loadConfig()
	if err != nil {
		slog.Error("Configuration error", slog.String("error", err.Error()))
		os.Exit(1)
	}

	processor := NewRSSProcessor(ctx, config)

	processor.logger.InfoContext(ctx, "Starting RSS processing",
		slog.Int("max_articles_per_run", config.MaxArticlesPerRun),
		slog.String("data_dir", config.DataDir),
		slog.Int("rss_feeds_count", len(config.RSSFeeds)))

	// Process RSS feeds
	if err := processor.FetchAndProcessRSSFeeds(ctx); err != nil {
		processor.logger.ErrorContext(ctx, "RSS processing failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	processor.logger.InfoContext(ctx, "RSS processing completed successfully")
}
