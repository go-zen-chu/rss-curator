//go:generate go run go.uber.org/mock/mockgen -source=$GOFILE -destination=mock_$GOFILE -package=$GOPACKAGE

package bluesky

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

// Session represents Bluesky session data
type Session struct {
	AccessJWT  string `json:"accessJwt"`
	RefreshJWT string `json:"refreshJwt"`
	Handle     string `json:"handle"`
	DID        string `json:"did"`
}

// CreateRecord represents a Bluesky post record
type CreateRecord struct {
	Repo       string      `json:"repo"`
	Collection string      `json:"collection"`
	Record     any `json:"record"`
}

// Post represents a Bluesky post content
type Post struct {
	Type      string    `json:"$type"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"createdAt"`
}

// ClientInterface defines the Bluesky client contract
type ClientInterface interface {
	Login(ctx context.Context, identifier, password string) error
	CreatePost(ctx context.Context, text string) error
}

// Client handles Bluesky API interactions
type Client struct {
	httpClient *http.Client
	logger     *slog.Logger
	session    *Session
}

// NewClient creates a new Bluesky client instance
func NewClient(ctx context.Context, logger *slog.Logger) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		logger:     logger,
	}
}

// Login authenticates with Bluesky
func (c *Client) Login(ctx context.Context, identifier, password string) error {
	loginData := map[string]string{
		"identifier": identifier,
		"password":   password,
	}

	jsonData, err := json.Marshal(loginData)
	if err != nil {
		return fmt.Errorf("failed to marshal login data: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://bsky.social/xrpc/com.atproto.server.createSession",
		bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create login request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to login to Bluesky: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Bluesky login error (status: %d): %s", resp.StatusCode, string(body))
	}

	var session Session
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return fmt.Errorf("failed to decode login response: %w", err)
	}

	c.session = &session
	c.logger.InfoContext(ctx, "Logged in to Bluesky successfully")

	return nil
}

// CreatePost creates a new post on Bluesky
func (c *Client) CreatePost(ctx context.Context, text string) error {
	if c.session == nil {
		return fmt.Errorf("not logged in to Bluesky")
	}

	// Create post record
	record := Post{
		Type:      "app.bsky.feed.post",
		Text:      text,
		CreatedAt: time.Now(),
	}

	createRecord := CreateRecord{
		Repo:       c.session.DID,
		Collection: "app.bsky.feed.post",
		Record:     record,
	}

	jsonData, err := json.Marshal(createRecord)
	if err != nil {
		return fmt.Errorf("failed to marshal post data: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://bsky.social/xrpc/com.atproto.repo.createRecord",
		bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.session.AccessJWT)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to post to Bluesky: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Bluesky API error (status: %d): %s", resp.StatusCode, string(body))
	}

	c.logger.InfoContext(ctx, "Posted to Bluesky successfully")

	return nil
}