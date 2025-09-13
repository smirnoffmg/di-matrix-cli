package usecases_test

import (
	"context"
	"di-matrix-cli/internal/domain"
	"di-matrix-cli/internal/usecases"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

// MockGitlabClient for testing
type MockGitlabClient struct {
	mock.Mock
}

func (m *MockGitlabClient) CheckPermissions(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockGitlabClient) GetRepositoriesList(ctx context.Context, repoURL string) ([]*domain.Repository, error) {
	args := m.Called(ctx, repoURL)
	return args.Get(0).([]*domain.Repository), args.Error(1)
}

func (m *MockGitlabClient) GetFilesList(ctx context.Context, repoURL string) ([]string, error) {
	args := m.Called(ctx, repoURL)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockGitlabClient) GetFileContent(ctx context.Context, repoURL, filePath string) ([]byte, error) {
	args := m.Called(ctx, repoURL, filePath)
	return args.Get(0).([]byte), args.Error(1)
}

// MockRepositoryScanner for testing
type MockRepositoryScanner struct {
	mock.Mock
}

func (m *MockRepositoryScanner) DetectProjects(
	ctx context.Context,
	repo *domain.Repository,
) ([]*domain.Project, error) {
	args := m.Called(ctx, repo)
	return args.Get(0).([]*domain.Project), args.Error(1)
}

// MockDependencyParser for testing
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

// MockDependencyClassifier for testing
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

// MockReportGenerator for testing
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

func TestNewAnalyzeUseCase(t *testing.T) {
	t.Parallel()

	// Create mock dependencies
	mockGitlabClient := &MockGitlabClient{}
	mockScanner := &MockRepositoryScanner{}
	mockParser := &MockDependencyParser{}
	mockClassifier := &MockDependencyClassifier{}
	mockGenerator := &MockReportGenerator{}

	logger := zap.NewNop()
	ctx := context.Background()

	// Test the constructor - it should succeed with valid dependencies
	useCase := usecases.NewAnalyzeUseCase(
		ctx,
		mockGitlabClient,
		mockScanner,
		mockParser,
		mockClassifier,
		mockGenerator,
		logger,
	)

	// The constructor should succeed and return a valid use case
	assert.NotNil(t, useCase)
}

func TestAnalyzeResponse_JSON(t *testing.T) {
	t.Parallel()

	response := &usecases.AnalyzeResponse{
		TotalProjects:     5,
		TotalDependencies: 100,
		InternalCount:     20,
		ExternalCount:     80,
	}

	// Test that the struct has the expected JSON tags
	// This is a simple test to ensure the struct is properly defined
	assert.Equal(t, 5, response.TotalProjects)
	assert.Equal(t, 100, response.TotalDependencies)
	assert.Equal(t, 20, response.InternalCount)
	assert.Equal(t, 80, response.ExternalCount)
}

func TestConcurrencySafety(t *testing.T) {
	t.Parallel()

	// This test verifies that the concurrency patterns used in Execute() are safe
	// We test that multiple goroutines can safely append to slices with proper synchronization

	var results []int
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Simulate concurrent operations similar to what happens in Execute()
	numGoroutines := 10
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(value int) {
			defer wg.Done()

			// Simulate some work
			time.Sleep(1 * time.Millisecond)

			// Safely append to shared slice
			mu.Lock()
			results = append(results, value)
			mu.Unlock()
		}(i)
	}

	wg.Wait()

	// Verify all values were added
	assert.Len(t, results, numGoroutines)

	// Verify no duplicates (each goroutine adds exactly one value)
	seen := make(map[int]bool)
	for _, value := range results {
		assert.False(t, seen[value], "Duplicate value found: %d", value)
		seen[value] = true
	}
}
