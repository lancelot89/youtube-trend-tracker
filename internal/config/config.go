package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	// Application settings
	App AppConfig `yaml:"app"`

	// YouTube API settings
	YouTube YouTubeConfig `yaml:"youtube"`

	// Google Cloud settings
	GCP GCPConfig `yaml:"gcp"`

	// BigQuery settings
	BigQuery BigQueryConfig `yaml:"bigquery"`

	// Server settings
	Server ServerConfig `yaml:"server"`

	// Logging settings
	Logging LoggingConfig `yaml:"logging"`

	// Channel configuration
	Channels []ChannelConfig `yaml:"channels"`
}

// AppConfig contains application-level settings
type AppConfig struct {
	Environment         string        `yaml:"environment"`
	MaxVideosPerChannel int64         `yaml:"max_videos_per_channel"`
	FetchTimeout        time.Duration `yaml:"fetch_timeout"`
}

// YouTubeConfig contains YouTube API settings
type YouTubeConfig struct {
	APIKey         string        `yaml:"api_key"`
	QuotaLimit     int           `yaml:"quota_limit"`
	RequestTimeout time.Duration `yaml:"request_timeout"`
	MaxRetries     int           `yaml:"max_retries"`
	RetryDelay     time.Duration `yaml:"retry_delay"`
}

// GCPConfig contains Google Cloud Platform settings
type GCPConfig struct {
	ProjectID      string `yaml:"project_id"`
	Region         string `yaml:"region"`
	ServiceAccount string `yaml:"service_account"`
}

// BigQueryConfig contains BigQuery settings
type BigQueryConfig struct {
	DatasetID    string        `yaml:"dataset_id"`
	TableID      string        `yaml:"table_id"`
	Location     string        `yaml:"location"`
	BatchSize    int           `yaml:"batch_size"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
}

// ServerConfig contains HTTP server settings
type ServerConfig struct {
	Port            string        `yaml:"port"`
	ReadTimeout     time.Duration `yaml:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
	MaxHeaderBytes  int           `yaml:"max_header_bytes"`
}

// LoggingConfig contains logging settings
type LoggingConfig struct {
	Level      string `yaml:"level"`
	Format     string `yaml:"format"`
	OutputPath string `yaml:"output_path"`
}

// ChannelConfig represents a YouTube channel to monitor
type ChannelConfig struct {
	ID          string `yaml:"id"`
	Name        string `yaml:"name,omitempty"`
	Description string `yaml:"description,omitempty"`
	Enabled     bool   `yaml:"enabled"`
}

// DefaultConfig returns a configuration with default values
func DefaultConfig() *Config {
	return &Config{
		App: AppConfig{
			Environment:         "development",
			MaxVideosPerChannel: 10,
			FetchTimeout:        5 * time.Minute,
		},
		YouTube: YouTubeConfig{
			QuotaLimit:     10000,
			RequestTimeout: 30 * time.Second,
			MaxRetries:     5,
			RetryDelay:     1 * time.Second,
		},
		GCP: GCPConfig{
			Region: "asia-northeast1",
		},
		BigQuery: BigQueryConfig{
			DatasetID:    "youtube",
			TableID:      "video_trends",
			Location:     "asia-northeast1",
			BatchSize:    500,
			WriteTimeout: 30 * time.Second,
		},
		Server: ServerConfig{
			Port:            "8080",
			ReadTimeout:     10 * time.Second,
			WriteTimeout:    10 * time.Second,
			ShutdownTimeout: 30 * time.Second,
			MaxHeaderBytes:  1 << 20, // 1 MB
		},
		Logging: LoggingConfig{
			Level:      "info",
			Format:     "json",
			OutputPath: "stdout",
		},
		Channels: []ChannelConfig{},
	}
}

// Load loads configuration from multiple sources with priority:
// 1. Environment variables (highest priority)
// 2. Configuration file
// 3. Default values (lowest priority)
func Load(configPath string) (*Config, error) {
	// Start with default configuration
	cfg := DefaultConfig()

	// Load from configuration file if provided
	if configPath != "" {
		if err := loadFromFile(cfg, configPath); err != nil {
			return nil, fmt.Errorf("failed to load config file: %w", err)
		}
	}

	// Override with environment variables
	loadFromEnv(cfg)

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// loadFromFile loads configuration from a YAML file
func loadFromFile(cfg *Config, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(cfg); err != nil {
		return fmt.Errorf("failed to decode YAML: %w", err)
	}

	return nil
}

// loadFromEnv loads configuration from environment variables
func loadFromEnv(cfg *Config) {
	// Load .env file if in local environment
	if os.Getenv("GO_ENV") == "local" {
		godotenv.Load()
	}

	// App settings
	if env := os.Getenv("GO_ENV"); env != "" {
		cfg.App.Environment = env
	}
	if env := os.Getenv("MAX_VIDEOS_PER_CHANNEL"); env != "" {
		if val, err := strconv.ParseInt(env, 10, 64); err == nil {
			cfg.App.MaxVideosPerChannel = val
		}
	}

	// YouTube settings
	if env := os.Getenv("YOUTUBE_API_KEY"); env != "" {
		cfg.YouTube.APIKey = env
	}

	// GCP settings
	if env := os.Getenv("GOOGLE_CLOUD_PROJECT"); env != "" {
		cfg.GCP.ProjectID = env
	}
	if env := os.Getenv("PROJECT_ID"); env != "" && cfg.GCP.ProjectID == "" {
		cfg.GCP.ProjectID = env
	}
	if env := os.Getenv("REGION"); env != "" {
		cfg.GCP.Region = env
	}

	// BigQuery settings
	if env := os.Getenv("BIGQUERY_DATASET"); env != "" {
		cfg.BigQuery.DatasetID = env
	}
	if env := os.Getenv("BIGQUERY_TABLE"); env != "" {
		cfg.BigQuery.TableID = env
	}

	// Server settings
	if env := os.Getenv("PORT"); env != "" {
		cfg.Server.Port = env
	}

	// Logging settings
	if env := os.Getenv("LOG_LEVEL"); env != "" {
		cfg.Logging.Level = strings.ToLower(env)
	}
	if env := os.Getenv("LOG_FORMAT"); env != "" {
		cfg.Logging.Format = env
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Required fields
	if c.YouTube.APIKey == "" {
		return fmt.Errorf("YouTube API key is required")
	}
	if c.GCP.ProjectID == "" {
		return fmt.Errorf("GCP project ID is required")
	}

	// Validate numeric ranges
	if c.App.MaxVideosPerChannel <= 0 {
		return fmt.Errorf("max_videos_per_channel must be positive")
	}
	if c.YouTube.MaxRetries < 0 {
		return fmt.Errorf("max_retries cannot be negative")
	}
	if c.BigQuery.BatchSize <= 0 {
		return fmt.Errorf("batch_size must be positive")
	}

	// Validate log level
	validLogLevels := map[string]bool{
		"debug":   true,
		"info":    true,
		"warning": true,
		"error":   true,
		"fatal":   true,
	}
	if !validLogLevels[c.Logging.Level] {
		return fmt.Errorf("invalid log level: %s", c.Logging.Level)
	}

	// At least one channel must be configured
	enabledChannels := 0
	for _, ch := range c.Channels {
		if ch.Enabled {
			enabledChannels++
			if ch.ID == "" {
				return fmt.Errorf("channel ID is required")
			}
		}
	}
	if enabledChannels == 0 {
		return fmt.Errorf("at least one enabled channel is required")
	}

	return nil
}

// GetEnabledChannelIDs returns a list of enabled channel IDs
func (c *Config) GetEnabledChannelIDs() []string {
	var ids []string
	for _, ch := range c.Channels {
		if ch.Enabled {
			ids = append(ids, ch.ID)
		}
	}
	return ids
}

// IsProduction returns true if running in production environment
func (c *Config) IsProduction() bool {
	return c.App.Environment == "production" || c.App.Environment == "prod"
}

// IsDevelopment returns true if running in development environment
func (c *Config) IsDevelopment() bool {
	return c.App.Environment == "development" || c.App.Environment == "dev"
}

// IsLocal returns true if running in local environment
func (c *Config) IsLocal() bool {
	return c.App.Environment == "local"
}
