package config_test

import (
	"di-matrix-cli/internal/config"
	"os"
	"testing"
)

//nolint:paralleltest // Cannot use t.Parallel() with t.Setenv()
func TestLoadConfig_ValidConfig(t *testing.T) {
	// Clear environment variables that might interfere with tests
	clearConfigEnvVars(t)
	defer restoreConfigEnvVars(t)

	// Create a temporary config file
	configContent := `
gitlab:
  base_url: "https://gitlab.com"
  token: "test-token"

repositories:
  - id: 1
    name: "test-repo"
    branch: "main"
    paths: ["/backend", "/frontend"]

internal:
  domains: ["gitlab.company.com/group"]
  patterns: ["@company/", "com.company."]

output:
  html_file: "test-report.html"
  title: "Test Dependency Matrix"
`

	tmpFile := createTempConfigFile(t, configContent)
	defer os.Remove(tmpFile)

	cfg, err := config.LoadConfig(tmpFile)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if cfg.GitLab.BaseURL != "https://gitlab.com" {
		t.Errorf("Expected base_url 'https://gitlab.com', got '%s'", cfg.GitLab.BaseURL)
	}

	if cfg.GitLab.Token != "test-token" {
		t.Errorf("Expected token 'test-token', got '%s'", cfg.GitLab.Token)
	}

	if len(cfg.Repositories) != 1 {
		t.Errorf("Expected 1 repository, got %d", len(cfg.Repositories))
	}

	if cfg.Repositories[0].ID != 1 {
		t.Errorf("Expected repository ID 1, got %d", cfg.Repositories[0].ID)
	}

	if cfg.Output.HTMLFile != "test-report.html" {
		t.Errorf("Expected html_file 'test-report.html', got '%s'", cfg.Output.HTMLFile)
	}

	// Test timeout default value
	if cfg.Timeout.AnalysisTimeoutMinutes != 10 {
		t.Errorf("Expected default timeout 10 minutes, got %d", cfg.Timeout.AnalysisTimeoutMinutes)
	}
}

func TestLoadConfig_InvalidPath(t *testing.T) {
	t.Parallel()
	_, err := config.LoadConfig("nonexistent.yaml")
	if err == nil {
		t.Fatal("Expected error for nonexistent config file")
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	t.Parallel()
	configContent := `invalid: yaml: content: [`
	tmpFile := createTempConfigFile(t, configContent)
	defer os.Remove(tmpFile)

	_, err := config.LoadConfig(tmpFile)
	if err == nil {
		t.Fatal("Expected error for invalid YAML")
	}
}

//nolint:paralleltest // Cannot use t.Parallel() with t.Setenv()
func TestLoadConfig_MissingRequiredFields(t *testing.T) {
	// Clear environment variables that might interfere with tests
	clearConfigEnvVars(t)
	defer restoreConfigEnvVars(t)

	configContent := `
gitlab:
  base_url: "https://gitlab.com"
  # Missing token

repositories:
  - id: 1

output:
  html_file: "test.html"
  title: "Test"
`

	tmpFile := createTempConfigFile(t, configContent)
	defer os.Remove(tmpFile)

	_, err := config.LoadConfig(tmpFile)
	if err == nil {
		t.Fatal("Expected error for missing required fields")
	}
}

func TestLoadConfig_EmptyRepositories(t *testing.T) {
	t.Parallel()
	configContent := `
gitlab:
  base_url: "https://gitlab.com"
  token: "test-token"

repositories: []

output:
  html_file: "test.html"
  title: "Test"
`

	tmpFile := createTempConfigFile(t, configContent)
	defer os.Remove(tmpFile)

	_, err := config.LoadConfig(tmpFile)
	if err == nil {
		t.Fatal("Expected error for empty repositories")
	}
}

func TestLoadConfig_InvalidRepositoryID(t *testing.T) {
	t.Parallel()
	configContent := `
gitlab:
  base_url: "https://gitlab.com"
  token: "test-token"

repositories:
  - id: 0  # Invalid ID

output:
  html_file: "test.html"
  title: "Test"
`

	tmpFile := createTempConfigFile(t, configContent)
	defer os.Remove(tmpFile)

	_, err := config.LoadConfig(tmpFile)
	if err == nil {
		t.Fatal("Expected error for invalid repository ID")
	}
}

// Environment variable backup for tests
var envBackup = make(map[string]string)

// clearConfigEnvVars clears environment variables that might interfere with config tests
func clearConfigEnvVars(t *testing.T) {
	envVars := []string{
		"GITLAB_BASE_URL",
		"GITLAB_TOKEN",
		"OUTPUT_HTML_FILE",
		"OUTPUT_TITLE",
		"ANALYSIS_TIMEOUT_MINUTES",
	}

	for _, envVar := range envVars {
		if value := os.Getenv(envVar); value != "" {
			envBackup[envVar] = value
			os.Unsetenv(envVar)
		}
	}
}

// restoreConfigEnvVars restores environment variables after tests
func restoreConfigEnvVars(t *testing.T) {
	for envVar, value := range envBackup {
		t.Setenv(envVar, value)
	}
	// Clear the backup
	envBackup = make(map[string]string)
}

func createTempConfigFile(t *testing.T, content string) string {
	tmpFile, err := os.CreateTemp(t.TempDir(), "test-config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}

	if err := tmpFile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	return tmpFile.Name()
}

//nolint:paralleltest // Cannot use t.Parallel() with t.Setenv()
func TestLoadConfig_TimeoutConfiguration(t *testing.T) {
	// Clear environment variables that might interfere with tests
	clearConfigEnvVars(t)
	defer restoreConfigEnvVars(t)

	// Create a config with custom timeout
	configContent := `
gitlab:
  base_url: "https://gitlab.com"
  token: "test-token"

repositories:
  - id: 1
    name: "test-repo"

output:
  html_file: "test.html"
  title: "Test"

timeout:
  analysis_timeout_minutes: 15
`

	tmpFile := createTempConfigFile(t, configContent)
	defer os.Remove(tmpFile)

	cfg, err := config.LoadConfig(tmpFile)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if cfg.Timeout.AnalysisTimeoutMinutes != 15 {
		t.Errorf("Expected timeout 15 minutes, got %d", cfg.Timeout.AnalysisTimeoutMinutes)
	}
}

//nolint:paralleltest // Cannot use t.Parallel() with t.Setenv()
func TestLoadConfig_TimeoutEnvironmentVariable(t *testing.T) {
	// Clear environment variables that might interfere with tests
	clearConfigEnvVars(t)
	defer restoreConfigEnvVars(t)

	// Set timeout environment variable
	t.Setenv("ANALYSIS_TIMEOUT_MINUTES", "20")

	// Create a config without timeout section
	configContent := `
gitlab:
  base_url: "https://gitlab.com"
  token: "test-token"

repositories:
  - id: 1
    name: "test-repo"

output:
  html_file: "test.html"
  title: "Test"
`

	tmpFile := createTempConfigFile(t, configContent)
	defer os.Remove(tmpFile)

	cfg, err := config.LoadConfig(tmpFile)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if cfg.Timeout.AnalysisTimeoutMinutes != 20 {
		t.Errorf("Expected timeout 20 minutes from environment variable, got %d", cfg.Timeout.AnalysisTimeoutMinutes)
	}
}
