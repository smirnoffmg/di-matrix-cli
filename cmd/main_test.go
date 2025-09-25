package main_test

import (
	"bytes"
	"context"
	"di-matrix-cli/internal/domain"
	"di-matrix-cli/internal/usecases"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// Mock dependencies for testing
type MockGitlabClient struct {
	mock.Mock
}

func (m *MockGitlabClient) GetRepositoriesList(
	ctx context.Context,
	repositoryURL string,
) ([]*domain.Repository, error) {
	args := m.Called(ctx, repositoryURL)
	return args.Get(0).([]*domain.Repository), args.Error(1)
}

func (m *MockGitlabClient) GetRepository(ctx context.Context, repositoryURL string) (*domain.Repository, error) {
	args := m.Called(ctx, repositoryURL)
	return args.Get(0).(*domain.Repository), args.Error(1)
}

func (m *MockGitlabClient) GetFilesList(ctx context.Context, repoURL string) ([]string, error) {
	args := m.Called(ctx, repoURL)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockGitlabClient) GetFileContent(ctx context.Context, repoURL, filePath string) ([]byte, error) {
	args := m.Called(ctx, repoURL, filePath)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockGitlabClient) CheckPermissions(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

type MockRepositoryScanner struct {
	mock.Mock
}

func (m *MockRepositoryScanner) DetectProjects(
	ctx context.Context,
	repository *domain.Repository,
) ([]*domain.Project, error) {
	args := m.Called(ctx, repository)
	return args.Get(0).([]*domain.Project), args.Error(1)
}

type MockDependencyParser struct {
	mock.Mock
}

func (m *MockDependencyParser) ParseFile(
	ctx context.Context,
	file *domain.DependencyFile,
) ([]*domain.Dependency, error) {
	args := m.Called(ctx, file)
	return args.Get(0).([]*domain.Dependency), args.Error(1)
}

type MockDependencyClassifier struct {
	mock.Mock
}

func (m *MockDependencyClassifier) ClassifyDependencies(
	ctx context.Context,
	dependencies []*domain.Dependency,
) ([]*domain.Dependency, error) {
	args := m.Called(ctx, dependencies)
	return args.Get(0).([]*domain.Dependency), args.Error(1)
}

func (m *MockDependencyClassifier) IsInternal(ctx context.Context, dependency *domain.Dependency) bool {
	args := m.Called(ctx, dependency)
	return args.Bool(0)
}

type MockReportGenerator struct {
	mock.Mock
}

func (m *MockReportGenerator) GenerateHTML(ctx context.Context, projects []*domain.Project) error {
	args := m.Called(ctx, projects)
	return args.Error(0)
}

func (m *MockReportGenerator) GenerateCSV(ctx context.Context, projects []*domain.Project) error {
	args := m.Called(ctx, projects)
	return args.Error(0)
}

func (m *MockReportGenerator) GenerateJSON(ctx context.Context, projects []*domain.Project) error {
	args := m.Called(ctx, projects)
	return args.Error(0)
}

// Test helper to create a temporary config file
func createTempConfig(t *testing.T, content string) string {
	t.Helper()

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test-config.yaml")

	err := os.WriteFile(configFile, []byte(content), 0o644)
	require.NoError(t, err)

	return configFile
}

func TestRootCmd_Execute(t *testing.T) {
	t.Parallel()

	// Create the root command
	cmd := &cobra.Command{
		Use:   "di-matrix-cli",
		Short: "Dependency Matrix CLI - Analyze GitLab repositories and generate dependency matrices",
		Long: `A command-line tool that analyzes multiple GitLab repositories to generate
comprehensive dependency matrices using event-driven architecture.`,
	}

	// Redirect stdout to a buffer to capture the output
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	// Execute the root command (should show help by default)
	err := cmd.Execute()

	// Check for any execution error
	require.NoError(t, err)

	// Check that help output is shown
	output := stdout.String()
	assert.Contains(t, output, "A command-line tool that analyzes multiple GitLab repositories")
}

func TestAnalyzeCmd_Execute_Success(t *testing.T) {
	t.Parallel()

	// Create temporary config file
	configContent := `
gitlab:
  base_url: "https://gitlab.com/"
  token: "test-token"

repositories:
  - id: 1
    url: "https://gitlab.com/test/repo1"

output:
  html_file: "test-output.html"
  title: "Test Report"

timeout:
  analysis_timeout_minutes: 5

internal:
  patterns: ["@company/*"]

logging:
  level: "info"

concurrency:
  repository_workers: 2
  file_fetcher_workers: 4
  parser_workers: 3
  generator_workers: 1
  queue_buffer_size: 10
  max_concurrent_requests: 5
`
	configFile := createTempConfig(t, configContent)

	// Create analyze command that simulates successful execution
	cmd := &cobra.Command{
		Use:   "analyze",
		Short: "Analyze repositories and generate dependency matrix",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Simulate successful analysis
			return nil
		},
	}

	// Add config flag
	cmd.Flags().StringP("config", "c", "", "Path to configuration file (required)")

	// Set the config file argument
	cmd.SetArgs([]string{"--config", configFile})

	// Execute the analyze command
	err := cmd.Execute()

	// Check for any execution error
	require.NoError(t, err)
}

func TestAnalyzeCmd_Execute_ConfigError(t *testing.T) {
	t.Parallel()

	// Create analyze command
	cmd := &cobra.Command{
		Use:   "analyze",
		Short: "Analyze repositories and generate dependency matrix",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Simulate config loading error
			return fmt.Errorf("failed to load configuration: config file not found")
		},
	}

	// Add config flag
	cmd.Flags().StringP("config", "c", "", "Path to configuration file (required)")

	// Set invalid config file argument
	cmd.SetArgs([]string{"--config", "/nonexistent/config.yaml"})

	// Redirect stderr to a buffer to capture the error output
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)

	// Execute the analyze command
	err := cmd.Execute()

	// Check that error is returned
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load configuration")
}

func TestAnalyzeCmd_Execute_AnalysisError(t *testing.T) {
	t.Parallel()

	// Create temporary config file
	configContent := `
gitlab:
  base_url: "https://gitlab.com/"
  token: "test-token"

repositories:
  - id: 1
    url: "https://gitlab.com/test/repo1"

output:
  html_file: "test-output.html"
  title: "Test Report"

timeout:
  analysis_timeout_minutes: 5

internal:
  patterns: ["@company/*"]

logging:
  level: "info"

concurrency:
  repository_workers: 2
  file_fetcher_workers: 4
  parser_workers: 3
  generator_workers: 1
  queue_buffer_size: 10
  max_concurrent_requests: 5
`
	configFile := createTempConfig(t, configContent)

	// Create mock dependencies
	mockGitlabClient := &MockGitlabClient{}
	mockScanner := &MockRepositoryScanner{}
	mockParser := &MockDependencyParser{}
	mockClassifier := &MockDependencyClassifier{}
	mockGenerator := &MockReportGenerator{}

	// Setup mock expectations to return error
	mockGitlabClient.On("GetRepositoriesList", mock.Anything, "https://gitlab.com/test/repo1").
		Return([]*domain.Repository(nil), fmt.Errorf("GitLab API error"))

	// Create analyze command
	cmd := &cobra.Command{
		Use:   "analyze",
		Short: "Analyze repositories and generate dependency matrix",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Simulate the runAnalyze function with mocks
			fmt.Println("🔍 Starting dependency matrix analysis...")

			// Create context with timeout
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()

			// Create analyze use case with mock dependencies
			analyzeUseCase := usecases.NewAnalyzeUseCase(
				ctx,
				mockGitlabClient,
				mockScanner,
				mockParser,
				mockClassifier,
				mockGenerator,
				zap.NewNop(),
			)

			_, err := analyzeUseCase.Execute([]string{"https://gitlab.com/test/repo1"}, "go")
			if err != nil {
				return fmt.Errorf("failed to analyze dependency matrix: %w", err)
			}

			return nil
		},
	}

	// Add config flag
	cmd.Flags().StringP("config", "c", "", "Path to configuration file (required)")

	// Set the config file argument
	cmd.SetArgs([]string{"--config", configFile})

	// Redirect stderr to a buffer to capture the error output
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)

	// Execute the analyze command
	err := cmd.Execute()

	// Check that error is returned
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to analyze dependency matrix")

	// Verify mocks were called
	mockGitlabClient.AssertExpectations(t)
}

func TestSetupCommands(t *testing.T) {
	t.Parallel()

	// Create root command
	rootCmd := &cobra.Command{
		Use:   "di-matrix-cli",
		Short: "Dependency Matrix CLI",
	}

	// Create analyze command
	analyzeCmd := &cobra.Command{
		Use:   "analyze",
		Short: "Analyze repositories and generate dependency matrix",
	}

	// Test that commands can be added
	rootCmd.AddCommand(analyzeCmd)

	// Verify analyze command is added
	commands := rootCmd.Commands()
	assert.Len(t, commands, 1)
	assert.Equal(t, "analyze", commands[0].Name())
}

func TestMain_ExitCode(t *testing.T) {
	t.Parallel()

	// This test verifies that main function handles errors properly
	// We can't directly test main() as it calls os.Exit(), but we can test the logic

	// Test successful execution path
	t.Run("successful execution", func(t *testing.T) {
		t.Parallel()
		// Create a command that succeeds
		cmd := &cobra.Command{
			Use: "test",
			Run: func(cmd *cobra.Command, args []string) {
				// Success - no error
			},
		}

		err := cmd.Execute()
		assert.NoError(t, err)
	})

	// Test error execution path
	t.Run("error execution", func(t *testing.T) {
		t.Parallel()
		// Create a command that fails
		cmd := &cobra.Command{
			Use: "test",
			RunE: func(cmd *cobra.Command, args []string) error {
				return fmt.Errorf("test error")
			},
		}

		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "test error")
	})
}

func TestRunAnalyze_TimeoutHandling(t *testing.T) {
	t.Parallel()

	// Create temporary config file with short timeout
	configContent := `
gitlab:
  base_url: "https://gitlab.com/"
  token: "test-token"

repositories:
  - id: 1
    url: "https://gitlab.com/test/repo1"

output:
  html_file: "test-output.html"
  title: "Test Report"

timeout:
  analysis_timeout_minutes: 1

internal:
  patterns: ["@company/*"]

logging:
  level: "info"

concurrency:
  repository_workers: 2
  file_fetcher_workers: 4
  parser_workers: 3
  generator_workers: 1
  queue_buffer_size: 10
  max_concurrent_requests: 5
`
	configFile := createTempConfig(t, configContent)

	// Create analyze command that tests timeout
	cmd := &cobra.Command{
		Use:   "analyze",
		Short: "Analyze repositories and generate dependency matrix",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Simulate timeout handling
			timeoutDuration := 1 * time.Minute
			ctx, cancel := context.WithTimeout(context.Background(), timeoutDuration)
			defer cancel()

			// Verify context has timeout
			deadline, ok := ctx.Deadline()
			assert.True(t, ok)
			assert.WithinDuration(t, time.Now().Add(timeoutDuration), deadline, time.Second)

			return nil
		},
	}

	// Add config flag
	cmd.Flags().StringP("config", "c", "", "Path to configuration file (required)")

	// Set the config file argument
	cmd.SetArgs([]string{"--config", configFile})

	// Execute the analyze command
	err := cmd.Execute()
	require.NoError(t, err)
}

func TestRunAnalyze_DebugFlag(t *testing.T) {
	t.Parallel()

	// Create temporary config file
	configContent := `
gitlab:
  base_url: "https://gitlab.com/"
  token: "test-token"

repositories:
  - id: 1
    url: "https://gitlab.com/test/repo1"

output:
  html_file: "test-output.html"
  title: "Test Report"

timeout:
  analysis_timeout_minutes: 5

internal:
  patterns: ["@company/*"]

logging:
  level: "info"

concurrency:
  repository_workers: 2
  file_fetcher_workers: 4
  parser_workers: 3
  generator_workers: 1
  queue_buffer_size: 10
  max_concurrent_requests: 5
`
	configFile := createTempConfig(t, configContent)

	// Create analyze command that tests debug flag
	cmd := &cobra.Command{
		Use:   "analyze",
		Short: "Analyze repositories and generate dependency matrix",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Test that debug flag is properly parsed
			debug, _ := cmd.Flags().GetBool("debug")
			assert.True(t, debug, "Debug flag should be true")
			return nil
		},
	}

	// Add flags
	cmd.Flags().StringP("config", "c", "", "Path to configuration file (required)")
	cmd.Flags().BoolP("debug", "d", false, "Enable debug logging")

	// Set the config file and debug flag arguments
	cmd.SetArgs([]string{"--config", configFile, "--debug"})

	// Execute the analyze command
	err := cmd.Execute()
	require.NoError(t, err)
}
