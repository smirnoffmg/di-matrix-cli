package scanner_test

import (
	"context"
	"di-matrix-cli/internal/domain"
	"di-matrix-cli/internal/scanner"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// MockGitlabClient is a mock implementation of the GitlabClient interface
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

func TestNewScanner(t *testing.T) {
	t.Parallel()
	mockClient := &MockGitlabClient{}
	logger := zap.NewNop()

	s := scanner.NewScanner(mockClient, logger)

	assert.NotNil(t, s)
}

func TestDetectProjects_Success(t *testing.T) {
	t.Parallel()
	mockClient := &MockGitlabClient{}
	logger := zap.NewNop()
	s := scanner.NewScanner(mockClient, logger)

	ctx := context.Background()
	repo := &domain.Repository{
		ID:            123,
		Name:          "test-repo",
		URL:           "https://gitlab.com/test/repo",
		DefaultBranch: "main",
		WebURL:        "https://gitlab.com/test/repo",
	}

	// Mock GetFilesList to return dependency files
	files := []string{
		"go.mod",
		"backend/go.mod",
		"frontend/package.json",
		"frontend/package-lock.json",
		"python/requirements.txt",
		"README.md",   // Should be filtered out
		"docs/api.md", // Should be filtered out
	}
	mockClient.On("GetFilesList", ctx, repo.URL).Return(files, nil)

	// Mock GetFileContent for each dependency file
	mockClient.On("GetFileContent", ctx, repo.URL, "go.mod").Return([]byte("module test"), nil)
	mockClient.On("GetFileContent", ctx, repo.URL, "backend/go.mod").Return([]byte("module backend"), nil)
	mockClient.On("GetFileContent", ctx, repo.URL, "frontend/package.json").Return([]byte(`{"name": "frontend"}`), nil)
	mockClient.On("GetFileContent", ctx, repo.URL, "frontend/package-lock.json").
		Return([]byte(`{"lockfileVersion": 1}`), nil)
	mockClient.On("GetFileContent", ctx, repo.URL, "python/requirements.txt").Return([]byte("requests==2.25.1"), nil)

	projects, err := s.DetectProjects(ctx, repo)

	require.NoError(t, err)
	assert.Len(t, projects, 4)

	// Check Go project in root
	goProject := findProjectByLanguage(projects, "go", "")
	assert.NotNil(t, goProject)
	assert.Equal(t, "repo-123-root-go", goProject.ID)
	assert.Equal(t, "test-repo Go", goProject.Name)
	assert.Empty(t, goProject.Path)
	assert.Len(t, goProject.DependencyFiles, 1)

	// Check Go project in backend
	backendGoProject := findProjectByLanguage(projects, "go", "backend")
	assert.NotNil(t, backendGoProject)
	assert.Equal(t, "repo-123-backend-go", backendGoProject.ID)
	assert.Equal(t, "test-repo Go (backend)", backendGoProject.Name)
	assert.Equal(t, "backend", backendGoProject.Path)
	assert.Len(t, backendGoProject.DependencyFiles, 1)

	// Check Node.js project in frontend
	nodejsProject := findProjectByLanguage(projects, "nodejs", "frontend")
	assert.NotNil(t, nodejsProject)
	assert.Equal(t, "repo-123-frontend-nodejs", nodejsProject.ID)
	assert.Equal(t, "test-repo Nodejs (frontend)", nodejsProject.Name)
	assert.Equal(t, "frontend", nodejsProject.Path)
	assert.Len(t, nodejsProject.DependencyFiles, 2)

	// Check Python project
	pythonProject := findProjectByLanguage(projects, "python", "python")
	assert.NotNil(t, pythonProject)
	assert.Equal(t, "repo-123-python-python", pythonProject.ID)
	assert.Equal(t, "test-repo Python (python)", pythonProject.Name)
	assert.Equal(t, "python", pythonProject.Path)
	assert.Len(t, pythonProject.DependencyFiles, 1)

	mockClient.AssertExpectations(t)
}

func TestDetectProjects_NoDependencyFiles(t *testing.T) {
	t.Parallel()
	mockClient := &MockGitlabClient{}
	logger := zap.NewNop()
	s := scanner.NewScanner(mockClient, logger)

	ctx := context.Background()
	repo := &domain.Repository{
		ID:            123,
		Name:          "empty-repo",
		URL:           "https://gitlab.com/test/empty",
		DefaultBranch: "main",
		WebURL:        "https://gitlab.com/test/empty",
	}

	// Mock GetFilesList to return no dependency files
	files := []string{"README.md", "docs/api.md", "src/main.cpp"}
	mockClient.On("GetFilesList", ctx, repo.URL).Return(files, nil)

	projects, err := s.DetectProjects(ctx, repo)

	require.NoError(t, err)
	assert.Empty(t, projects)

	mockClient.AssertExpectations(t)
}

func TestDetectProjects_GetFilesListError(t *testing.T) {
	t.Parallel()
	mockClient := &MockGitlabClient{}
	logger := zap.NewNop()
	s := scanner.NewScanner(mockClient, logger)

	ctx := context.Background()
	repo := &domain.Repository{
		ID:            123,
		Name:          "error-repo",
		URL:           "https://gitlab.com/test/error",
		DefaultBranch: "main",
		WebURL:        "https://gitlab.com/test/error",
	}

	// Mock GetFilesList to return an error
	mockClient.On("GetFilesList", ctx, repo.URL).Return([]string{}, assert.AnError)

	projects, err := s.DetectProjects(ctx, repo)

	require.Error(t, err)
	assert.Nil(t, projects)
	assert.Contains(t, err.Error(), "failed to get files list")

	mockClient.AssertExpectations(t)
}

func TestDetectProjects_GetFileContentError(t *testing.T) {
	t.Parallel()
	mockClient := &MockGitlabClient{}
	logger := zap.NewNop()
	s := scanner.NewScanner(mockClient, logger)

	ctx := context.Background()
	repo := &domain.Repository{
		ID:            123,
		Name:          "content-error-repo",
		URL:           "https://gitlab.com/test/content-error",
		DefaultBranch: "main",
		WebURL:        "https://gitlab.com/test/content-error",
	}

	// Mock GetFilesList to return dependency files
	files := []string{"go.mod", "package.json"}
	mockClient.On("GetFilesList", ctx, repo.URL).Return(files, nil)

	// Mock GetFileContent to return error for go.mod but success for package.json
	mockClient.On("GetFileContent", ctx, repo.URL, "go.mod").Return([]byte{}, assert.AnError)
	mockClient.On("GetFileContent", ctx, repo.URL, "package.json").Return([]byte(`{"name": "test"}`), nil)

	projects, err := s.DetectProjects(ctx, repo)

	// Should not return error, but should skip the file that failed
	require.NoError(t, err)
	assert.Len(t, projects, 1) // Only the nodejs project should be created

	// Check that the Go project was not created (because go.mod failed)
	goProject := findProjectByLanguage(projects, "go", "")
	assert.Nil(t, goProject)

	// Check that the nodejs project exists and has dependency files
	nodejsProject := findProjectByLanguage(projects, "nodejs", "")
	assert.NotNil(t, nodejsProject)
	assert.Len(t, nodejsProject.DependencyFiles, 1)

	mockClient.AssertExpectations(t)
}

func TestDetectProjects_MultiProjectRepository(t *testing.T) {
	t.Parallel()
	mockClient := &MockGitlabClient{}
	logger := zap.NewNop()
	s := scanner.NewScanner(mockClient, logger)

	ctx := context.Background()
	repo := &domain.Repository{
		ID:            456,
		Name:          "multi-project-repo",
		URL:           "https://gitlab.com/test/multi-project",
		DefaultBranch: "main",
		WebURL:        "https://gitlab.com/test/multi-project",
	}

	// Mock GetFilesList to return the specific dependency files you mentioned
	files := []string{
		"autotests/poetry.lock",
		"backend/uv.lock",
		"frontend/package-lock.json",
		"README.md",   // Should be filtered out
		"docs/api.md", // Should be filtered out
	}
	mockClient.On("GetFilesList", ctx, repo.URL).Return(files, nil)

	// Mock GetFileContent for each dependency file
	mockClient.On("GetFileContent", ctx, repo.URL, "autotests/poetry.lock").
		Return([]byte("[[package]]\nname = \"pytest\"\nversion = \"7.4.0\""), nil)
	mockClient.On("GetFileContent", ctx, repo.URL, "backend/uv.lock").
		Return([]byte("# This file is autogenerated by uv.\nversion = 1\n"), nil)
	mockClient.On("GetFileContent", ctx, repo.URL, "frontend/package-lock.json").
		Return([]byte(`{"name": "frontend", "lockfileVersion": 2}`), nil)

	projects, err := s.DetectProjects(ctx, repo)

	require.NoError(t, err)
	assert.Len(t, projects, 3) // Exactly 3 projects as expected

	// Check Python project in autotests
	pythonProject := findProjectByLanguage(projects, "python", "autotests")
	assert.NotNil(t, pythonProject)
	assert.Equal(t, "repo-456-autotests-python", pythonProject.ID)
	assert.Equal(t, "multi-project-repo Python (autotests)", pythonProject.Name)
	assert.Equal(t, "autotests", pythonProject.Path)
	assert.Len(t, pythonProject.DependencyFiles, 1)
	assert.Equal(t, "autotests/poetry.lock", pythonProject.DependencyFiles[0].Path)

	// Check Python project in backend (uv.lock is also Python)
	backendPythonProject := findProjectByLanguage(projects, "python", "backend")
	assert.NotNil(t, backendPythonProject)
	assert.Equal(t, "repo-456-backend-python", backendPythonProject.ID)
	assert.Equal(t, "multi-project-repo Python (backend)", backendPythonProject.Name)
	assert.Equal(t, "backend", backendPythonProject.Path)
	assert.Len(t, backendPythonProject.DependencyFiles, 1)
	assert.Equal(t, "backend/uv.lock", backendPythonProject.DependencyFiles[0].Path)

	// Check Node.js project in frontend
	nodejsProject := findProjectByLanguage(projects, "nodejs", "frontend")
	assert.NotNil(t, nodejsProject)
	assert.Equal(t, "repo-456-frontend-nodejs", nodejsProject.ID)
	assert.Equal(t, "multi-project-repo Nodejs (frontend)", nodejsProject.Name)
	assert.Equal(t, "frontend", nodejsProject.Path)
	assert.Len(t, nodejsProject.DependencyFiles, 1)
	assert.Equal(t, "frontend/package-lock.json", nodejsProject.DependencyFiles[0].Path)

	mockClient.AssertExpectations(t)
}

func TestSupportedFileTypes(t *testing.T) {
	t.Parallel()
	s := &scanner.Scanner{}
	fileTypes := s.SupportedFileTypes()

	expectedTypes := []string{
		"go.mod", "go.sum",
		"package.json", "package-lock.json", "yarn.lock",
		"pom.xml", "build.gradle", "gradle.lockfile",
		"requirements.txt", "Pipfile", "poetry.lock", "uv.lock", "setup.py",
	}

	assert.ElementsMatch(t, expectedTypes, fileTypes)
}

func TestDetectLanguageFromFile(t *testing.T) {
	t.Parallel()
	s := &scanner.Scanner{}

	tests := []struct {
		fileName string
		expected string
	}{
		{"go.mod", "go"},
		{"go.sum", "go"},
		{"GO.MOD", "go"}, // Test case insensitivity
		{"package.json", "nodejs"},
		{"package-lock.json", "nodejs"},
		{"yarn.lock", "nodejs"},
		{"pom.xml", "java"},
		{"build.gradle", "java"},
		{"gradle.lockfile", "java"},
		{"requirements.txt", "python"},
		{"Pipfile", "python"},
		{"poetry.lock", "python"},
		{"uv.lock", "python"},
		{"setup.py", "python"},
		{"unknown.txt", "unknown"},
		{"README.md", "unknown"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.fileName, func(t *testing.T) {
			t.Parallel()
			result := s.DetectLanguageFromFile(test.fileName)
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestExtractProjectPath(t *testing.T) {
	t.Parallel()
	s := &scanner.Scanner{}

	tests := []struct {
		filePath string
		expected string
	}{
		{"go.mod", ""},
		{"./go.mod", ""},
		{"/go.mod", ""},
		{"backend/go.mod", "backend"},
		{"frontend/package.json", "frontend"},
		{"nested/deep/path/pom.xml", "nested/deep/path"},
		{"backend/src/main/go.mod", "backend/src/main"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.filePath, func(t *testing.T) {
			t.Parallel()
			result := s.ExtractProjectPath(test.filePath)
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestCapitalizeFirst(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"a", "A"},
		{"go", "Go"},
		{"nodejs", "Nodejs"},
		{"python", "Python"},
		{"java", "Java"},
		{"Golang", "Golang"}, // Already capitalized
		{"ñ", "Ñ"},           // Unicode test
		{"测试", "测试"},         // Unicode test (no change needed)
	}

	for _, test := range tests {
		test := test
		t.Run(test.input, func(t *testing.T) {
			t.Parallel()
			result := scanner.CapitalizeFirst(test.input)
			assert.Equal(t, test.expected, result)
		})
	}
}

// Helper function to find a project by language and path
func findProjectByLanguage(projects []*domain.Project, language, path string) *domain.Project {
	for _, project := range projects {
		if project.Language == language && project.Path == path {
			return project
		}
	}
	return nil
}
