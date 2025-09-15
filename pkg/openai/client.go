//go:generate go run go.uber.org/mock/mockgen -source=$GOFILE -destination=mock_$GOFILE -package=$GOPACKAGE

package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// TranslationRequest represents the request to OpenAI for translation
type TranslationRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature"`
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Response represents the response from OpenAI API
type Response struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// TranslationResult represents the result from OpenAI translation
type TranslationResult struct {
	TranslatedTitle string `json:"translated_title"`
	JapaneseSummary string `json:"japanese_summary"`
	Language        string `json:"language"`
}

// ClientInterface defines the OpenAI client contract
type ClientInterface interface {
	TranslateArticle(ctx context.Context, title, content string) (*TranslationResult, error)
}

// Client handles OpenAI API interactions
type Client struct {
	apiKey     string
	httpClient *http.Client
	logger     *slog.Logger
}

// NewClient creates a new OpenAI client instance
func NewClient(ctx context.Context, apiKey string, logger *slog.Logger) *Client {
	return &Client{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		logger:     logger,
	}
}

// TranslateArticle translates and summarizes an article using OpenAI
func (c *Client) TranslateArticle(ctx context.Context, title, content string) (*TranslationResult, error) {
	prompt := fmt.Sprintf(`
以下のニュース記事を日本語に翻訳し、要約してください。

タイトル: %s
内容: %s

以下のJSON形式で回答してください：
{
    "translated_title": "翻訳されたタイトル（日本語）",
    "japanese_summary": "記事の要約（日本語、150文字以内）",
    "language": "元の言語（例：en, ko, zh など）"
}
`, title, content)

	request := TranslationRequest{
		Model:       "gpt-4o-mini",
		Temperature: 0.3,
		Messages: []Message{
			{
				Role:    "system",
				Content: "あなたは優秀な翻訳者です。正確で自然な日本語翻訳と、簡潔で分かりやすい要約を作成してください。",
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
	}

	response, err := c.makeRequest(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to translate article: %w", err)
	}

	var result TranslationResult
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, fmt.Errorf("failed to parse translation result: %w", err)
	}

	return &result, nil
}

// makeRequest makes a request to OpenAI API
func (c *Client) makeRequest(ctx context.Context, request TranslationRequest) (string, error) {
	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to encode request JSON: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://api.openai.com/v1/chat/completions",
		bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("OpenAI API error (status: %d): %s", resp.StatusCode, string(body))
	}

	var response Response
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("failed to parse response JSON: %w", err)
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("empty response from OpenAI API")
	}

	return response.Choices[0].Message.Content, nil
}