package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// Config represents the main configuration structure
type Config struct {
	GitLab       GitLabConfig       `yaml:"gitlab"       mapstructure:"gitlab"`
	Repositories []RepositoryConfig `yaml:"repositories" mapstructure:"repositories"`
	Internal     InternalConfig     `yaml:"internal"     mapstructure:"internal"`
	Output       OutputConfig       `yaml:"output"       mapstructure:"output"`
	Timeout      TimeoutConfig      `yaml:"timeout"      mapstructure:"timeout"`
}

// GitLabConfig represents GitLab connection settings
type GitLabConfig struct {
	BaseURL string `yaml:"base_url" mapstructure:"base_url"`
	Token   string `yaml:"token"    mapstructure:"token"`
}

// RepositoryConfig represents a repository to analyze
type RepositoryConfig struct {
	URL    string   `yaml:"url"              mapstructure:"url"`
	ID     int      `yaml:"id,omitempty"     mapstructure:"id"`
	Name   string   `yaml:"name,omitempty"   mapstructure:"name"`
	Branch string   `yaml:"branch,omitempty" mapstructure:"branch"`
	Paths  []string `yaml:"paths,omitempty"  mapstructure:"paths"`
}

// InternalConfig represents internal dependency classification settings
type InternalConfig struct {
	Domains  []string `yaml:"domains"  mapstructure:"domains"`
	Patterns []string `yaml:"patterns" mapstructure:"patterns"`
}

// OutputConfig represents output settings
type OutputConfig struct {
	HTMLFile string `yaml:"html_file" mapstructure:"html_file"`
	Title    string `yaml:"title"     mapstructure:"title"`
}

// TimeoutConfig represents timeout configuration
type TimeoutConfig struct {
	AnalysisTimeoutMinutes int `yaml:"analysis_timeout_minutes" mapstructure:"analysis_timeout_minutes"`
}

// LoadConfig loads configuration from file and environment variables
func LoadConfig(configPath string) (*Config, error) {
	if configPath == "" {
		return nil, fmt.Errorf("config path is required")
	}

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file does not exist: %s", configPath)
	}

	// Create a new Viper instance to avoid data races in concurrent tests
	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	// Set default values
	setDefaultValues(v)

	// Enable reading from environment variables
	v.AutomaticEnv()

	// Set environment variable key replacer for nested config
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Bind environment variables to config keys
	_ = v.BindEnv("gitlab.base_url", "GITLAB_BASE_URL")
	_ = v.BindEnv("gitlab.token", "GITLAB_TOKEN")
	_ = v.BindEnv("output.html_file", "OUTPUT_HTML_FILE")
	_ = v.BindEnv("output.title", "OUTPUT_TITLE")
	_ = v.BindEnv("timeout.analysis_timeout_minutes", "ANALYSIS_TIMEOUT_MINUTES")

	// Read config file
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Unmarshal into struct
	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate configuration
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &config, nil
}

// setDefaultValues sets default configuration values
func setDefaultValues(v *viper.Viper) {
	// Output defaults
	v.SetDefault("output.html_file", "dependency-matrix.html")
	v.SetDefault("output.title", "Dependency Matrix Report")

	// Repository defaults
	v.SetDefault("repositories", []RepositoryConfig{})

	// Internal classification defaults
	v.SetDefault("internal.domains", []string{})
	v.SetDefault("internal.patterns", []string{})

	// Logging defaults
	v.SetDefault("logging.level", "info")

	// Concurrency defaults
	v.SetDefault("concurrency.repository_workers", 4)
	v.SetDefault("concurrency.file_fetcher_workers", 8)
	v.SetDefault("concurrency.parser_workers", 6)
	v.SetDefault("concurrency.generator_workers", 2)
	v.SetDefault("concurrency.queue_buffer_size", 50)
	v.SetDefault("concurrency.max_concurrent_repos", 10)
	v.SetDefault("concurrency.max_concurrent_files", 20)
	v.SetDefault("concurrency.max_concurrent_parsers", 15)

	// Timeout defaults (10 minutes as per user preference for console operations)
	v.SetDefault("timeout.analysis_timeout_minutes", 10)
}

// validateConfig validates the configuration
func validateConfig(config Config) error {
	if config.GitLab.BaseURL == "" {
		return fmt.Errorf("gitlab.base_url is required")
	}

	if config.GitLab.Token == "" {
		return fmt.Errorf("gitlab.token is required")
	}

	if len(config.Repositories) == 0 {
		return fmt.Errorf("at least one repository must be configured")
	}

	if config.Output.HTMLFile == "" {
		return fmt.Errorf("output.html_file is required")
	}

	if config.Output.Title == "" {
		return fmt.Errorf("output.title is required")
	}

	// Validate repositories
	for i, repo := range config.Repositories {
		if repo.URL == "" && repo.ID <= 0 {
			return fmt.Errorf("repository[%d] must have either url or id specified", i)
		}
		if repo.URL != "" && repo.ID > 0 {
			return fmt.Errorf("repository[%d] should not have both url and id specified", i)
		}
	}

	return nil
}
