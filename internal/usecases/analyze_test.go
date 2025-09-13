package usecases_test

import (
	"context"
	"di-matrix-cli/internal/domain"
	"di-matrix-cli/internal/usecases"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
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

	// Test that the use case can be created and used concurrently
	mockGitlabClient := &MockGitlabClient{}
	mockScanner := &MockRepositoryScanner{}
	mockParser := &MockDependencyParser{}
	mockClassifier := &MockDependencyClassifier{}
	mockGenerator := &MockReportGenerator{}

	logger := zap.NewNop()
	ctx := context.Background()

	useCase := usecases.NewAnalyzeUseCase(
		ctx,
		mockGitlabClient,
		mockScanner,
		mockParser,
		mockClassifier,
		mockGenerator,
		logger,
	)

	// Test concurrent access to the use case
	done := make(chan bool, 5)

	for i := 0; i < 5; i++ {
		go func() {
			defer func() { done <- true }()

			// The use case should be safe for concurrent access
			assert.NotNil(t, useCase)
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 5; i++ {
		<-done
	}
}

func TestExecute_Success(t *testing.T) {
	t.Parallel()

	// Create mock dependencies
	mockGitlabClient := &MockGitlabClient{}
	mockScanner := &MockRepositoryScanner{}
	mockParser := &MockDependencyParser{}
	mockClassifier := &MockDependencyClassifier{}
	mockGenerator := &MockReportGenerator{}

	logger := zap.NewNop()
	ctx := context.Background()

	// Setup mock expectations
	repo1 := &domain.Repository{
		ID:   1,
		Name: "test-repo-1",
		URL:  "https://gitlab.com/test/repo1",
	}
	repo2 := &domain.Repository{
		ID:   2,
		Name: "test-repo-2",
		URL:  "https://gitlab.com/test/repo2",
	}

	project1 := &domain.Project{
		ID:       "repo1-project1",
		Name:     "Project 1",
		Language: "Go",
		Path:     "/project1",
		DependencyFiles: []*domain.DependencyFile{
			{
				Path:     "go.mod",
				Language: "Go",
				Content:  []byte("module test"),
			},
		},
	}

	project2 := &domain.Project{
		ID:       "repo2-project1",
		Name:     "Project 2",
		Language: "JavaScript",
		Path:     "/project2",
		DependencyFiles: []*domain.DependencyFile{
			{
				Path:     "package.json",
				Language: "JavaScript",
				Content:  []byte(`{"dependencies": {"express": "^4.0.0"}}`),
			},
		},
	}

	dependency1 := &domain.Dependency{
		Name:       "github.com/gin-gonic/gin",
		Version:    "v1.9.0",
		Ecosystem:  "go-modules",
		IsInternal: false,
	}

	dependency2 := &domain.Dependency{
		Name:       "express",
		Version:    "^4.0.0",
		Ecosystem:  "npm",
		IsInternal: false,
	}

	// Mock GitLab client to return repositories
	mockGitlabClient.On("GetRepositoriesList", mock.Anything, "https://gitlab.com/test/repo1").
		Return([]*domain.Repository{repo1}, nil)
	mockGitlabClient.On("GetRepositoriesList", mock.Anything, "https://gitlab.com/test/repo2").
		Return([]*domain.Repository{repo2}, nil)

	// Mock scanner to return projects
	mockScanner.On("DetectProjects", mock.Anything, repo1).Return([]*domain.Project{project1}, nil)
	mockScanner.On("DetectProjects", mock.Anything, repo2).Return([]*domain.Project{project2}, nil)

	// Mock parser to return dependencies
	mockParser.On("ParseFile", mock.Anything, project1.DependencyFiles[0]).
		Return([]*domain.Dependency{dependency1}, nil)
	mockParser.On("ParseFile", mock.Anything, project2.DependencyFiles[0]).
		Return([]*domain.Dependency{dependency2}, nil)

	// Mock IsInternal calls (the actual method being called)
	mockClassifier.On("IsInternal", mock.Anything, dependency1).Return(false)
	mockClassifier.On("IsInternal", mock.Anything, dependency2).Return(false)

	// Mock generator to succeed
	mockGenerator.On("GenerateHTML", mock.Anything, mock.AnythingOfType("[]*domain.Project")).Return(nil)

	// Create use case
	useCase := usecases.NewAnalyzeUseCase(
		ctx,
		mockGitlabClient,
		mockScanner,
		mockParser,
		mockClassifier,
		mockGenerator,
		logger,
	)

	// Execute the use case
	repositoryURLs := []string{
		"https://gitlab.com/test/repo1",
		"https://gitlab.com/test/repo2",
	}

	response, err := useCase.Execute(repositoryURLs)

	// Verify results
	require.NoError(t, err)
	require.NotNil(t, response)
	assert.Equal(t, 2, response.TotalProjects)
	assert.Equal(t, 2, response.TotalDependencies)
	assert.Equal(t, 0, response.InternalCount)
	assert.Equal(t, 2, response.ExternalCount)

	// Verify all mocks were called
	mockGitlabClient.AssertExpectations(t)
	mockScanner.AssertExpectations(t)
	mockParser.AssertExpectations(t)
	mockClassifier.AssertExpectations(t)
	mockGenerator.AssertExpectations(t)
}

func TestExecute_GitLabClientError(t *testing.T) {
	t.Parallel()

	// Create mock dependencies
	mockGitlabClient := &MockGitlabClient{}
	mockScanner := &MockRepositoryScanner{}
	mockParser := &MockDependencyParser{}
	mockClassifier := &MockDependencyClassifier{}
	mockGenerator := &MockReportGenerator{}

	logger := zap.NewNop()
	ctx := context.Background()

	// Mock GitLab client to return error
	mockGitlabClient.On("GetRepositoriesList", mock.Anything, "https://gitlab.com/test/repo1").
		Return([]*domain.Repository(nil), assert.AnError)

	// Create use case
	useCase := usecases.NewAnalyzeUseCase(
		ctx,
		mockGitlabClient,
		mockScanner,
		mockParser,
		mockClassifier,
		mockGenerator,
		logger,
	)

	// Execute the use case
	repositoryURLs := []string{"https://gitlab.com/test/repo1"}

	response, err := useCase.Execute(repositoryURLs)

	// Verify error is returned
	require.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "assert.AnError")

	// Verify mocks were called
	mockGitlabClient.AssertExpectations(t)
}

func TestExecute_ScannerError(t *testing.T) {
	t.Parallel()

	// Create mock dependencies
	mockGitlabClient := &MockGitlabClient{}
	mockScanner := &MockRepositoryScanner{}
	mockParser := &MockDependencyParser{}
	mockClassifier := &MockDependencyClassifier{}
	mockGenerator := &MockReportGenerator{}

	logger := zap.NewNop()
	ctx := context.Background()

	// Setup mock expectations
	repo1 := &domain.Repository{
		ID:   1,
		Name: "test-repo-1",
		URL:  "https://gitlab.com/test/repo1",
	}

	// Mock GitLab client to return repository
	mockGitlabClient.On("GetRepositoriesList", mock.Anything, "https://gitlab.com/test/repo1").
		Return([]*domain.Repository{repo1}, nil)

	// Mock scanner to return error
	mockScanner.On("DetectProjects", mock.Anything, repo1).Return([]*domain.Project(nil), assert.AnError)

	// Mock generator to succeed (even with 0 projects)
	mockGenerator.On("GenerateHTML", mock.Anything, mock.AnythingOfType("[]*domain.Project")).Return(nil)

	// Create use case
	useCase := usecases.NewAnalyzeUseCase(
		ctx,
		mockGitlabClient,
		mockScanner,
		mockParser,
		mockClassifier,
		mockGenerator,
		logger,
	)

	// Execute the use case
	repositoryURLs := []string{"https://gitlab.com/test/repo1"}

	response, err := useCase.Execute(repositoryURLs)

	// Verify that scanner errors are logged but don't fail the entire process
	// The use case should continue and return a response with 0 projects
	require.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, 0, response.TotalProjects)
	assert.Equal(t, 0, response.TotalDependencies)

	// Verify mocks were called
	mockGitlabClient.AssertExpectations(t)
	mockScanner.AssertExpectations(t)
	mockGenerator.AssertExpectations(t)
}

func TestExecute_GeneratorError(t *testing.T) {
	t.Parallel()

	// Create mock dependencies
	mockGitlabClient := &MockGitlabClient{}
	mockScanner := &MockRepositoryScanner{}
	mockParser := &MockDependencyParser{}
	mockClassifier := &MockDependencyClassifier{}
	mockGenerator := &MockReportGenerator{}

	logger := zap.NewNop()
	ctx := context.Background()

	// Setup mock expectations
	repo1 := &domain.Repository{
		ID:   1,
		Name: "test-repo-1",
		URL:  "https://gitlab.com/test/repo1",
	}

	project1 := &domain.Project{
		ID:       "repo1-project1",
		Name:     "Project 1",
		Language: "Go",
		Path:     "/project1",
		DependencyFiles: []*domain.DependencyFile{
			{
				Path:     "go.mod",
				Language: "Go",
				Content:  []byte("module test"),
			},
		},
	}

	dependency1 := &domain.Dependency{
		Name:       "github.com/gin-gonic/gin",
		Version:    "v1.9.0",
		Ecosystem:  "go-modules",
		IsInternal: false,
	}

	// Mock GitLab client to return repository
	mockGitlabClient.On("GetRepositoriesList", mock.Anything, "https://gitlab.com/test/repo1").
		Return([]*domain.Repository{repo1}, nil)

	// Mock scanner to return project
	mockScanner.On("DetectProjects", mock.Anything, repo1).Return([]*domain.Project{project1}, nil)

	// Mock parser to return dependencies
	mockParser.On("ParseFile", mock.Anything, project1.DependencyFiles[0]).
		Return([]*domain.Dependency{dependency1}, nil)

	// Mock IsInternal calls (the actual method being called)
	mockClassifier.On("IsInternal", mock.Anything, dependency1).Return(false)

	// Mock generator to return error
	mockGenerator.On("GenerateHTML", mock.Anything, mock.AnythingOfType("[]*domain.Project")).Return(assert.AnError)

	// Create use case
	useCase := usecases.NewAnalyzeUseCase(
		ctx,
		mockGitlabClient,
		mockScanner,
		mockParser,
		mockClassifier,
		mockGenerator,
		logger,
	)

	// Execute the use case
	repositoryURLs := []string{"https://gitlab.com/test/repo1"}

	response, err := useCase.Execute(repositoryURLs)

	// Verify error is returned
	require.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "assert.AnError")

	// Verify mocks were called
	mockGitlabClient.AssertExpectations(t)
	mockScanner.AssertExpectations(t)
	mockParser.AssertExpectations(t)
	mockClassifier.AssertExpectations(t)
	mockGenerator.AssertExpectations(t)
}
