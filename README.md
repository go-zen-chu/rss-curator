# RSS Curator

[![Documentation](https://pkg.go.dev/badge/github.com/go-zen-chu/rss-curator)](http://pkg.go.dev/github.com/go-zen-chu/rss-curator)
[![Actions Status](https://github.com/go-zen-chu/rss-curator/workflows/main/badge.svg)](https://github.com/go-zen-chu/rss-curator/actions)
[![GitHub issues](https://img.shields.io/github/issues/go-zen-chu/rss-curator.svg)](https://github.com/go-zen-chu/rss-curator/issues)

RSS Curator is an intelligent RSS feed processor that automatically translates news articles to Japanese and posts summaries to Bluesky social network.

## Features

- **RSS Feed Processing**: Fetches articles from multiple RSS feeds
- **AI Translation**: Uses OpenAI API to translate articles to Japanese with intelligent summarization
- **Social Media Integration**: Automatically posts translated summaries to Bluesky
- **Duplicate Prevention**: Tracks processed articles to avoid duplicates
- **Configurable Limits**: Controls how many articles to process per run
- **Structured Storage**: Saves processed articles in organized JSON format

## Architecture

The application follows clean architecture principles with clear separation of concerns:

```
├── main.go              # Application entry point and RSS processing logic
├── pkg/
│   ├── bluesky/        # Bluesky API client
│   └── openai/         # OpenAI API client
├── testdata/           # Test fixtures and sample data
└── data/               # Processed articles storage (created at runtime)
```

## Configuration

Set the following environment variables:

```bash
export OPENAI_API_KEY="your-openai-api-key"
export BLUESKY_IDENTIFIER="your-bluesky-handle-or-email"
export BLUESKY_PASSWORD="your-bluesky-password"

# Optional configuration
export MAX_ARTICLES_PER_RUN="10"        # Default: 10
export DATA_DIR="./data/articles"       # Default: data/articles
```

## Usage

### Basic Usage

```bash
go run main.go
```

## How It Works

1. **RSS Parsing**: Fetches and parses RSS feeds from configured sources
2. **Article Filtering**: Processes only articles from the last 24 hours
3. **Duplicate Check**: Skips articles that have already been processed
4. **AI Translation**: Uses OpenAI GPT-4o-mini to translate titles and summarize content in Japanese
5. **Content Optimization**: Truncates content to fit Bluesky's character limits
6. **Social Posting**: Posts translated summaries with links to Bluesky
7. **Data Storage**: Saves processed articles in organized JSON files by year/month

## Development

### Project Structure

The codebase follows Go best practices and clean architecture:

- **Separation of Concerns**: External integrations isolated in `pkg/` packages
- **Dependency Injection**: Constructor-based dependency injection pattern
- **Interface Segregation**: Well-defined interfaces for testability
- **Error Handling**: Comprehensive error wrapping and context

### Adding New RSS Feeds

To add new RSS feeds, modify the `RSSFeeds` slice in the `loadConfig()` function:

```go
RSSFeeds: []string{
    "https://your-new-feed-url.com/rss",
    // ... existing feeds
},
```

### Extending Translation Logic

The translation logic can be customized by modifying the `TranslateArticle` method in `pkg/openai/client.go`.

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes following the coding standards
4. Add tests for new functionality
5. Commit your changes (`git commit -m 'Add amazing feature'`)
6. Push to the branch (`git push origin feature/amazing-feature`)
7. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- [gofeed](https://github.com/mmcdole/gofeed) for RSS parsing
- [OpenAI](https://openai.com/) for AI-powered translation
- [Bluesky](https://bsky.social/) for social media integration
a