package generator

import (
	"context"
	"di-matrix-cli/internal/domain"
	_ "embed"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strconv"
)

//go:embed template.html
var templateContent string

// Generator creates HTML reports from project dependencies
type Generator struct {
	outputPath string
}

// NewGenerator creates a new report generator
func NewGenerator(outputPath string) *Generator {
	return &Generator{
		outputPath: outputPath,
	}
}

// OutputPath returns the output path
func (g *Generator) OutputPath() string {
	return g.outputPath
}

// GenerateSummary creates aggregated statistics (template embedded)
func (g *Generator) GenerateSummary(ctx context.Context, projects []*domain.Project) map[string]interface{} {
	languages := make(map[string]int)
	internalExternal := map[string]int{"internal": 0, "external": 0}
	ecosystems := make(map[string]int)
	totalDependencies := 0

	// Count dependencies and categorize
	for _, project := range projects {
		// Count by language
		if project.Language != "" {
			languages[project.Language]++
		}

		// Count dependencies
		for _, dep := range project.Dependencies {
			totalDependencies++

			// Count internal/external
			if dep.IsInternal {
				internalExternal["internal"]++
			} else {
				internalExternal["external"]++
			}

			// Count by ecosystem
			if dep.Ecosystem != "" {
				ecosystems[dep.Ecosystem]++
			}
		}
	}

	return map[string]interface{}{
		"total_projects":     len(projects),
		"total_dependencies": totalDependencies,
		"languages":          languages,
		"internal_external":  internalExternal,
		"ecosystems":         ecosystems,
	}
}

// filterProjectsWithDependencies filters out projects with zero dependencies
func (g *Generator) filterProjectsWithDependencies(projects []*domain.Project) []*domain.Project {
	var filteredProjects []*domain.Project
	for _, project := range projects {
		if len(project.Dependencies) > 0 {
			filteredProjects = append(filteredProjects, project)
		}
	}
	return filteredProjects
}

// groupProjectsByLanguage groups projects by their language
func (g *Generator) groupProjectsByLanguage(projects []*domain.Project) map[string][]*domain.Project {
	projectsByLanguage := make(map[string][]*domain.Project)
	for _, project := range projects {
		projectsByLanguage[project.Language] = append(projectsByLanguage[project.Language], project)
	}
	return projectsByLanguage
}

// sortDependencies sorts dependencies by type (internal first, external after) and then alphabetically
func (g *Generator) sortDependencies(
	dependencies []string,
	projectDeps map[string]map[string]*domain.Dependency,
) []string {
	// Create a slice of dependency info for sorting
	type depInfo struct {
		name       string
		isInternal bool
	}

	var depInfos []depInfo
	for _, depName := range dependencies {
		// Find the first project that has this dependency to determine if it's internal
		isInternal := false
		for _, projectDeps := range projectDeps {
			if dep, exists := projectDeps[depName]; exists {
				isInternal = dep.IsInternal
				break
			}
		}
		depInfos = append(depInfos, depInfo{
			name:       depName,
			isInternal: isInternal,
		})
	}

	// Sort by type (internal first) and then alphabetically
	sort.Slice(depInfos, func(i, j int) bool {
		// First sort by type: internal dependencies come first
		if depInfos[i].isInternal != depInfos[j].isInternal {
			return depInfos[i].isInternal
		}
		// Then sort alphabetically
		return depInfos[i].name < depInfos[j].name
	})

	// Extract sorted dependency names
	var sortedDependencies []string
	for _, depInfo := range depInfos {
		sortedDependencies = append(sortedDependencies, depInfo.name)
	}

	return sortedDependencies
}

// createLanguageMatrix creates a matrix for a specific language
func (g *Generator) createLanguageMatrix(languageProjects []*domain.Project) map[string]interface{} {
	// Collect unique dependencies for this language
	dependencySet := make(map[string]bool)
	for _, project := range languageProjects {
		for _, dep := range project.Dependencies {
			dependencySet[dep.Name] = true
		}
	}

	// Convert to slice for sorting
	var dependencies []string
	for depName := range dependencySet {
		dependencies = append(dependencies, depName)
	}

	// Create project dependency map for quick lookup
	projectDeps := make(map[string]map[string]*domain.Dependency)
	for _, project := range languageProjects {
		projectDeps[project.ID] = make(map[string]*domain.Dependency)
		for _, dep := range project.Dependencies {
			projectDeps[project.ID][dep.Name] = dep
		}
	}

	// Sort dependencies by type (internal first) and then alphabetically
	dependencies = g.sortDependencies(dependencies, projectDeps)

	// Create matrix data for this language
	matrix := make([][]interface{}, len(languageProjects))
	for i, project := range languageProjects {
		matrix[i] = make([]interface{}, len(dependencies))
		for j, depName := range dependencies {
			if dep, exists := projectDeps[project.ID][depName]; exists {
				matrix[i][j] = map[string]interface{}{
					"version":     dep.Version,
					"constraint":  dep.Constraint,
					"is_internal": dep.IsInternal,
					"ecosystem":   dep.Ecosystem,
				}
			} else {
				matrix[i][j] = nil
			}
		}
	}

	return map[string]interface{}{
		"dependencies": dependencies,
		"projects":     languageProjects,
		"matrix":       matrix,
	}
}

// createCombinedMatrix creates a combined matrix for all projects
func (g *Generator) createCombinedMatrix(projects []*domain.Project) ([]string, [][]interface{}) {
	// Collect all unique dependencies across filtered projects
	allDependencySet := make(map[string]bool)
	for _, project := range projects {
		for _, dep := range project.Dependencies {
			allDependencySet[dep.Name] = true
		}
	}

	var allDependencies []string
	for depName := range allDependencySet {
		allDependencies = append(allDependencies, depName)
	}

	// Create project dependency map for quick lookup
	allProjectDeps := make(map[string]map[string]*domain.Dependency)
	for _, project := range projects {
		allProjectDeps[project.ID] = make(map[string]*domain.Dependency)
		for _, dep := range project.Dependencies {
			allProjectDeps[project.ID][dep.Name] = dep
		}
	}

	// Sort dependencies by type (internal first) and then alphabetically
	allDependencies = g.sortDependencies(allDependencies, allProjectDeps)

	// Create combined matrix data
	combinedMatrix := make([][]interface{}, len(projects))
	for i, project := range projects {
		combinedMatrix[i] = make([]interface{}, len(allDependencies))
		for j, depName := range allDependencies {
			if dep, exists := allProjectDeps[project.ID][depName]; exists {
				combinedMatrix[i][j] = map[string]interface{}{
					"version":     dep.Version,
					"constraint":  dep.Constraint,
					"is_internal": dep.IsInternal,
					"ecosystem":   dep.Ecosystem,
				}
			} else {
				combinedMatrix[i][j] = nil
			}
		}
	}

	return allDependencies, combinedMatrix
}

// GenerateMatrix creates language-separated dependency matrices for easier template handling
func (g *Generator) GenerateMatrix(ctx context.Context, projects []*domain.Project) map[string]interface{} {
	// Filter out projects with zero dependencies
	filteredProjects := g.filterProjectsWithDependencies(projects)

	// Group projects by language
	projectsByLanguage := g.groupProjectsByLanguage(filteredProjects)

	// Create language-specific matrices
	languageMatrices := make(map[string]map[string]interface{})
	for language, languageProjects := range projectsByLanguage {
		languageMatrices[language] = g.createLanguageMatrix(languageProjects)
	}

	// Create combined matrix
	allDependencies, combinedMatrix := g.createCombinedMatrix(filteredProjects)

	// Create JSON string for language matrices
	languageMatricesJSON, err := json.Marshal(languageMatrices)
	if err != nil {
		// Fallback to empty object if marshaling fails
		languageMatricesJSON = []byte("{}")
	}

	return map[string]interface{}{
		"languages":     languageMatrices,
		"languagesJSON": string(languageMatricesJSON),
		"dependencies":  allDependencies,
		"projects":      filteredProjects,
		"matrix":        combinedMatrix,
	}
}

// GenerateHTML creates an HTML report from projects
func (g *Generator) GenerateHTML(ctx context.Context, projects []*domain.Project) error {
	// Create output directory if it doesn't exist
	dir := filepath.Dir(g.outputPath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate summary statistics
	summary := g.GenerateSummary(ctx, projects)

	// Generate matrix data
	matrix := g.GenerateMatrix(ctx, projects)

	// Create template data
	data := struct {
		Projects []*domain.Project
		Summary  map[string]interface{}
		Matrix   map[string]interface{}
		Title    string
	}{
		Projects: projects,
		Summary:  summary,
		Matrix:   matrix,
		Title:    "Dependency Matrix Report",
	}

	// Parse embedded template
	tmpl, err := template.New("report").Parse(templateContent)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	// Create output file
	file, err := os.Create(g.outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	// Execute template
	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

// GenerateCSV creates a CSV report from projects
func (g *Generator) GenerateCSV(ctx context.Context, projects []*domain.Project) error {
	// Create output directory if it doesn't exist
	dir := filepath.Dir(g.outputPath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create output file
	file, err := os.Create(g.outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	// Create CSV writer
	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write CSV header
	header := []string{
		"Project ID",
		"Project Name",
		"Repository Name",
		"Language",
		"Dependency Name",
		"Version",
		"Constraint",
		"Is Internal",
		"Ecosystem",
	}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write project data
	for _, project := range projects {
		for _, dependency := range project.Dependencies {
			record := []string{
				project.ID,
				project.Name,
				project.Repository.Name,
				project.Language,
				dependency.Name,
				dependency.Version,
				dependency.Constraint,
				strconv.FormatBool(dependency.IsInternal),
				dependency.Ecosystem,
			}
			if err := writer.Write(record); err != nil {
				return fmt.Errorf("failed to write CSV record: %w", err)
			}
		}
	}

	return nil
}

// GenerateJSON creates a JSON report from projects
func (g *Generator) GenerateJSON(ctx context.Context, projects []*domain.Project) error {
	// Create output directory if it doesn't exist
	dir := filepath.Dir(g.outputPath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate summary statistics
	summary := g.GenerateSummary(ctx, projects)

	// Create report data structure
	reportData := struct {
		Projects []*domain.Project      `json:"projects"`
		Summary  map[string]interface{} `json:"summary"`
		Title    string                 `json:"title"`
	}{
		Projects: projects,
		Summary:  summary,
		Title:    "Dependency Matrix Report",
	}

	// Create output file
	file, err := os.Create(g.outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	// Create JSON encoder with indentation
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	// Encode data to JSON
	if err := encoder.Encode(reportData); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	return nil
}
