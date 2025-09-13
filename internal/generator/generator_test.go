package generator_test

import (
	"context"
	"di-matrix-cli/internal/domain"
	"di-matrix-cli/internal/generator"
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGenerator(t *testing.T) {
	t.Parallel()
	outputPath := "/tmp/test-report.html"
	gen := generator.NewGenerator(outputPath)

	assert.NotNil(t, gen)
	assert.Equal(t, outputPath, gen.OutputPath())
}

func TestGenerateSummary(t *testing.T) {
	t.Parallel()
	gen := generator.NewGenerator("/tmp/test.html")
	ctx := context.Background()

	projects := createTestProjects()
	summary := gen.GenerateSummary(ctx, projects)

	// Test summary structure
	assert.NotNil(t, summary)
	assert.Contains(t, summary, "total_projects")
	assert.Contains(t, summary, "total_dependencies")
	assert.Contains(t, summary, "languages")
	assert.Contains(t, summary, "internal_external")

	// Test counts
	assert.Equal(t, 2, summary["total_projects"])
	assert.Equal(t, 4, summary["total_dependencies"])

	// Test language distribution
	languages := summary["languages"].(map[string]int)
	assert.Equal(t, 1, languages["go"])
	assert.Equal(t, 1, languages["nodejs"])

	// Test internal/external distribution
	internalExternal := summary["internal_external"].(map[string]int)
	assert.Equal(t, 1, internalExternal["internal"])
	assert.Equal(t, 3, internalExternal["external"])
}

func TestGenerateMatrix(t *testing.T) {
	t.Parallel()
	gen := generator.NewGenerator("/tmp/test.html")
	ctx := context.Background()

	projects := createTestProjects()
	matrix := gen.GenerateMatrix(ctx, projects)

	// Test matrix structure
	assert.NotNil(t, matrix)
	assert.Contains(t, matrix, "dependencies")
	assert.Contains(t, matrix, "projects")
	assert.Contains(t, matrix, "matrix")

	// Test dependencies list
	dependencies := matrix["dependencies"].([]string)
	assert.Len(t, dependencies, 4) // Should have 4 unique dependencies
	assert.Contains(t, dependencies, "github.com/gin-gonic/gin")
	assert.Contains(t, dependencies, "internal/company/auth")
	assert.Contains(t, dependencies, "express")
	assert.Contains(t, dependencies, "react")

	// Test sorting: internal first, then external alphabetically
	expectedOrder := []string{
		"internal/company/auth",    // internal (first)
		"express",                  // external (alphabetically first)
		"github.com/gin-gonic/gin", // external (alphabetically second)
		"react",                    // external (alphabetically third)
	}
	assert.Equal(
		t,
		expectedOrder,
		dependencies,
		"Dependencies should be sorted by type (internal first) and then alphabetically",
	)

	// Test projects list
	matrixProjects := matrix["projects"].([]*domain.Project)
	assert.Len(t, matrixProjects, 2)

	// Test matrix data
	matrixData := matrix["matrix"].([][]interface{})
	assert.Len(t, matrixData, 2)    // 2 projects
	assert.Len(t, matrixData[0], 4) // 4 dependencies

	// Test specific matrix cells
	// Project 1 should have gin and auth dependencies
	project1Row := matrixData[0]
	ginIndex := -1
	authIndex := -1
	for i, dep := range dependencies {
		if dep == "github.com/gin-gonic/gin" {
			ginIndex = i
		}
		if dep == "internal/company/auth" {
			authIndex = i
		}
	}
	assert.NotEqual(t, -1, ginIndex)
	assert.NotEqual(t, -1, authIndex)

	// With the new sorting, auth should be at index 0 and gin at index 2
	assert.Equal(t, 0, authIndex, "internal/company/auth should be at index 0 (first)")
	assert.Equal(t, 2, ginIndex, "github.com/gin-gonic/gin should be at index 2 (third)")

	// Check gin dependency in project 1
	ginCell := project1Row[ginIndex].(map[string]interface{})
	assert.Equal(t, "v1.9.1", ginCell["version"])
	assert.Equal(t, "^1.9.0", ginCell["constraint"])
	assert.Equal(t, false, ginCell["is_internal"])
	assert.Equal(t, "go-modules", ginCell["ecosystem"])

	// Check auth dependency in project 1
	authCell := project1Row[authIndex].(map[string]interface{})
	assert.Equal(t, "v1.0.0", authCell["version"])
	assert.Equal(t, "v1.0.0", authCell["constraint"])
	assert.Equal(t, true, authCell["is_internal"])
	assert.Equal(t, "go-modules", authCell["ecosystem"])

	// Check that project 1 doesn't have express or react
	expressIndex := -1
	reactIndex := -1
	for i, dep := range dependencies {
		if dep == "express" {
			expressIndex = i
		}
		if dep == "react" {
			reactIndex = i
		}
	}
	assert.Nil(t, project1Row[expressIndex])
	assert.Nil(t, project1Row[reactIndex])

	// With the new sorting, express should be at index 1 and react at index 3
	assert.Equal(t, 1, expressIndex, "express should be at index 1 (second)")
	assert.Equal(t, 3, reactIndex, "react should be at index 3 (fourth)")

	// Test project 2 row
	project2Row := matrixData[1]
	// Check express dependency in project 2
	expressCell := project2Row[expressIndex].(map[string]interface{})
	assert.Equal(t, "4.18.2", expressCell["version"])
	assert.Equal(t, "^4.18.0", expressCell["constraint"])
	assert.Equal(t, false, expressCell["is_internal"])
	assert.Equal(t, "npm", expressCell["ecosystem"])

	// Check react dependency in project 2
	reactCell := project2Row[reactIndex].(map[string]interface{})
	assert.Equal(t, "18.2.0", reactCell["version"])
	assert.Equal(t, "^18.0.0", reactCell["constraint"])
	assert.Equal(t, false, reactCell["is_internal"])
	assert.Equal(t, "npm", reactCell["ecosystem"])

	// Check that project 2 doesn't have gin or auth
	assert.Nil(t, project2Row[ginIndex])
	assert.Nil(t, project2Row[authIndex])
}

// Helper function to verify file creation and basic content
func verifyFileCreated(t *testing.T, outputPath string) string {
	// Check if file was created
	_, err := os.Stat(outputPath)
	require.NoError(t, err)

	// Read and verify file content
	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	return string(content)
}

// Helper function to verify project data in content
func verifyProjectData(t *testing.T, content string) {
	// Verify project data is included
	assert.Contains(t, content, "Test Project 1")
	assert.Contains(t, content, "Test Project 2")

	// Verify dependency data is included
	assert.Contains(t, content, "github.com/gin-gonic/gin")
	assert.Contains(t, content, "express")
}

func TestGenerateHTML(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "test-report.html")

	gen := generator.NewGenerator(outputPath)
	ctx := context.Background()

	projects := createTestProjects()
	err := gen.GenerateHTML(ctx, projects)

	require.NoError(t, err)

	htmlContent := verifyFileCreated(t, outputPath)

	// Verify HTML structure
	assert.Contains(t, htmlContent, "<!DOCTYPE html>")
	assert.Contains(t, htmlContent, "<html lang=\"en\">")
	assert.Contains(t, htmlContent, "<head>")
	assert.Contains(t, htmlContent, "<body class=\"bg-gray-50 font-sans\">")

	verifyProjectData(t, htmlContent)

	// Verify matrix table is included
	assert.Contains(t, htmlContent, "Dependency Matrix")
	assert.Contains(t, htmlContent, "tab-content")
}

func TestGenerateCSV(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "test-report.csv")

	gen := generator.NewGenerator(outputPath)
	ctx := context.Background()

	projects := createTestProjects()
	err := gen.GenerateCSV(ctx, projects)

	require.NoError(t, err)

	// Check if file was created
	_, err = os.Stat(outputPath)
	require.NoError(t, err)

	// Read and verify file content
	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	csvContent := string(content)

	// Verify CSV structure
	assert.Contains(
		t,
		csvContent,
		"Project ID,Project Name,Repository Name,Language,Dependency Name,Version,Constraint,Is Internal,Ecosystem",
	)
	assert.Contains(
		t,
		csvContent,
		"test-project-1,Test Project 1,test-repo-1,go,github.com/gin-gonic/gin,v1.9.1,^1.9.0,false,go-modules",
	)
	assert.Contains(
		t,
		csvContent,
		"test-project-1,Test Project 1,test-repo-1,go,internal/company/auth,v1.0.0,v1.0.0,true,go-modules",
	)
	assert.Contains(t, csvContent, "test-project-2,Test Project 2,test-repo-2,nodejs,express,4.18.2,^4.18.0,false,npm")
	assert.Contains(t, csvContent, "test-project-2,Test Project 2,test-repo-2,nodejs,react,18.2.0,^18.0.0,false,npm")
}

func TestGenerateJSON(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "test-report.json")

	gen := generator.NewGenerator(outputPath)
	ctx := context.Background()

	projects := createTestProjects()
	err := gen.GenerateJSON(ctx, projects)

	require.NoError(t, err)

	jsonContent := verifyFileCreated(t, outputPath)

	// Verify JSON structure
	assert.Contains(t, jsonContent, "\"projects\"")
	assert.Contains(t, jsonContent, "\"summary\"")
	assert.Contains(t, jsonContent, "\"total_projects\"")
	assert.Contains(t, jsonContent, "\"total_dependencies\"")
	assert.Contains(t, jsonContent, "\"languages\"")
	assert.Contains(t, jsonContent, "\"internal_external\"")
	assert.Contains(t, jsonContent, "\"ecosystems\"")

	verifyProjectData(t, jsonContent)
}

func TestGenerateHTML_EmptyProjects(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "empty-report.html")

	gen := generator.NewGenerator(outputPath)
	ctx := context.Background()

	err := gen.GenerateHTML(ctx, []*domain.Project{})

	require.NoError(t, err)

	// Check if file was created
	_, err = os.Stat(outputPath)
	require.NoError(t, err)

	// Read and verify file content
	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	htmlContent := string(content)
	assert.Contains(t, htmlContent, "Dependency Matrix")
	assert.Contains(t, htmlContent, "Total Projects")
	assert.Contains(t, htmlContent, "Total Dependencies")
}

func TestGenerateCSV_EmptyProjects(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "empty-report.csv")

	gen := generator.NewGenerator(outputPath)
	ctx := context.Background()

	err := gen.GenerateCSV(ctx, []*domain.Project{})

	require.NoError(t, err)

	// Check if file was created
	_, err = os.Stat(outputPath)
	require.NoError(t, err)

	// Read and verify file content
	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	csvContent := string(content)
	// Should only contain headers for empty projects
	assert.Contains(
		t,
		csvContent,
		"Project ID,Project Name,Repository Name,Language,Dependency Name,Version,Constraint,Is Internal,Ecosystem",
	)
}

func TestGenerateJSON_EmptyProjects(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "empty-report.json")

	gen := generator.NewGenerator(outputPath)
	ctx := context.Background()

	err := gen.GenerateJSON(ctx, []*domain.Project{})

	require.NoError(t, err)

	// Check if file was created
	_, err = os.Stat(outputPath)
	require.NoError(t, err)

	// Read and verify file content
	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	jsonContent := string(content)
	assert.Contains(t, jsonContent, "\"total_projects\": 0")
	assert.Contains(t, jsonContent, "\"total_dependencies\": 0")
}

func TestGenerateCSV_SpecialCharacters(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "special-chars-report.csv")

	gen := generator.NewGenerator(outputPath)
	ctx := context.Background()

	// Create test projects with special characters
	projects := []*domain.Project{
		{
			ID:   "test-project-special",
			Name: "Test Project with \"quotes\" and, commas",
			Repository: domain.Repository{
				ID:   123,
				Name: "test-repo\nwith\nnewlines",
				URL:  "https://gitlab.com/test/repo",
			},
			Path:     "backend/",
			Language: "go",
			Dependencies: []*domain.Dependency{
				{
					Name:       "github.com/company/\"special\"-package",
					Version:    "v1.0.0",
					Constraint: "^1.0.0",
					IsInternal: false,
					Ecosystem:  "go-modules",
				},
				{
					Name:       "package,with,commas",
					Version:    "2.0.0",
					Constraint: ">=2.0.0,<3.0.0",
					IsInternal: true,
					Ecosystem:  "npm",
				},
				{
					Name:       "package\nwith\nnewlines",
					Version:    "3.0.0",
					Constraint: "~3.0.0",
					IsInternal: false,
					Ecosystem:  "maven",
				},
			},
		},
	}

	err := gen.GenerateCSV(ctx, projects)
	require.NoError(t, err)

	// Check if file was created
	_, err = os.Stat(outputPath)
	require.NoError(t, err)

	// Read and verify file content
	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	csvContent := string(content)

	// Verify CSV structure is valid (no broken lines)
	lines := strings.Split(strings.TrimSpace(csvContent), "\n")
	assert.GreaterOrEqual(t, len(lines), 4) // Header + 3 dependency rows

	// Verify header is present
	assert.Contains(
		t,
		csvContent,
		"Project ID,Project Name,Repository Name,Language,Dependency Name,Version,Constraint,Is Internal,Ecosystem",
	)

	// Verify special characters are properly escaped
	// Quotes should be escaped with double quotes
	assert.Contains(t, csvContent, "\"Test Project with \"\"quotes\"\" and, commas\"")
	assert.Contains(t, csvContent, "\"test-repo\nwith\nnewlines\"")
	assert.Contains(t, csvContent, "\"github.com/company/\"\"special\"\"-package\"")
	assert.Contains(t, csvContent, "\"package,with,commas\"")
	assert.Contains(t, csvContent, "\"package\nwith\nnewlines\"")

	// Verify CSV can be parsed correctly
	reader := csv.NewReader(strings.NewReader(csvContent))
	records, err := reader.ReadAll()
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(records), 4) // Header + 3 dependency rows

	// Verify first record is header
	assert.Equal(t, []string{
		"Project ID",
		"Project Name",
		"Repository Name",
		"Language",
		"Dependency Name",
		"Version",
		"Constraint",
		"Is Internal",
		"Ecosystem",
	}, records[0])

	// Verify data integrity - check that special characters are preserved
	foundSpecialPackage := false
	for _, record := range records[1:] { // Skip header
		if len(record) < 5 || record[4] != "github.com/company/\"special\"-package" {
			continue
		}
		foundSpecialPackage = true
		assert.Equal(t, "test-project-special", record[0])
		assert.Equal(t, "Test Project with \"quotes\" and, commas", record[1])
		assert.Equal(t, "test-repo\nwith\nnewlines", record[2])
		assert.Equal(t, "go", record[3])
		assert.Equal(t, "v1.0.0", record[5])
		assert.Equal(t, "^1.0.0", record[6])
		assert.Equal(t, "false", record[7])
		assert.Equal(t, "go-modules", record[8])
	}
	assert.True(t, foundSpecialPackage, "Special package with quotes should be found in CSV")
}

func TestGenerateCSV_UnicodeCharacters(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "unicode-report.csv")

	gen := generator.NewGenerator(outputPath)
	ctx := context.Background()

	// Create test projects with unicode characters
	projects := []*domain.Project{
		{
			ID:   "test-project-unicode",
			Name: "测试项目 with émojis 🚀 and ñ characters",
			Repository: domain.Repository{
				ID:   123,
				Name: "répository-测试",
				URL:  "https://gitlab.com/test/repo",
			},
			Path:     "backend/",
			Language: "go",
			Dependencies: []*domain.Dependency{
				{
					Name:       "github.com/测试/unicode-package",
					Version:    "v1.0.0",
					Constraint: "^1.0.0",
					IsInternal: false,
					Ecosystem:  "go-modules",
				},
			},
		},
	}

	err := gen.GenerateCSV(ctx, projects)
	require.NoError(t, err)

	// Check if file was created
	_, err = os.Stat(outputPath)
	require.NoError(t, err)

	// Read and verify file content
	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	csvContent := string(content)

	// Verify unicode characters are preserved
	assert.Contains(t, csvContent, "测试项目 with émojis 🚀 and ñ characters")
	assert.Contains(t, csvContent, "répository-测试")
	assert.Contains(t, csvContent, "github.com/测试/unicode-package")

	// Verify CSV can be parsed correctly with unicode
	reader := csv.NewReader(strings.NewReader(csvContent))
	records, err := reader.ReadAll()
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(records), 2) // Header + 1 dependency row

	// Verify unicode data integrity
	foundUnicodePackage := false
	for _, record := range records[1:] { // Skip header
		if len(record) < 5 || record[4] != "github.com/测试/unicode-package" {
			continue
		}
		foundUnicodePackage = true
		assert.Equal(t, "测试项目 with émojis 🚀 and ñ characters", record[1])
		assert.Equal(t, "répository-测试", record[2])
	}
	assert.True(t, foundUnicodePackage, "Unicode package should be found in CSV")
}

// Helper function to create test projects
func createTestProjects() []*domain.Project {
	return []*domain.Project{
		{
			ID:   "test-project-1",
			Name: "Test Project 1",
			Repository: domain.Repository{
				ID:   123,
				Name: "test-repo-1",
				URL:  "https://gitlab.com/test/repo1",
			},
			Path:     "backend/",
			Language: "go",
			Dependencies: []*domain.Dependency{
				{
					Name:       "github.com/gin-gonic/gin",
					Version:    "v1.9.1",
					Constraint: "^1.9.0",
					IsInternal: false,
					Ecosystem:  "go-modules",
				},
				{
					Name:       "internal/company/auth",
					Version:    "v1.0.0",
					Constraint: "v1.0.0",
					IsInternal: true,
					Ecosystem:  "go-modules",
				},
			},
		},
		{
			ID:   "test-project-2",
			Name: "Test Project 2",
			Repository: domain.Repository{
				ID:   456,
				Name: "test-repo-2",
				URL:  "https://gitlab.com/test/repo2",
			},
			Path:     "frontend/",
			Language: "nodejs",
			Dependencies: []*domain.Dependency{
				{
					Name:       "express",
					Version:    "4.18.2",
					Constraint: "^4.18.0",
					IsInternal: false,
					Ecosystem:  "npm",
				},
				{
					Name:       "react",
					Version:    "18.2.0",
					Constraint: "^18.0.0",
					IsInternal: false,
					Ecosystem:  "npm",
				},
			},
		},
	}
}
