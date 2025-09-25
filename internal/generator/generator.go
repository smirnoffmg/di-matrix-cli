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

// createCombinedMatrix creates a combined matrix for all projects
func (g *Generator) createCombinedMatrix(projects []*domain.Project) ([]map[string]interface{}, [][]interface{}) {
	// Collect all unique dependencies across filtered projects
	allDependencySet := make(map[string]*domain.Dependency)
	for _, project := range projects {
		for _, dep := range project.Dependencies {
			// Keep the latest version if we have multiple instances of the same dependency
			if existingDep, exists := allDependencySet[dep.Name]; !exists ||
				dep.LatestVersion > existingDep.LatestVersion {
				allDependencySet[dep.Name] = dep
			}
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

	// Convert to dependency objects with name and latest_version
	var dependencyObjects []map[string]interface{}
	for _, depName := range allDependencies {
		dep := allDependencySet[depName]
		dependencyObjects = append(dependencyObjects, map[string]interface{}{
			"name":           dep.Name,
			"latest_version": dep.LatestVersion,
		})
	}

	// Create combined matrix data
	combinedMatrix := make([][]interface{}, len(projects))
	for i, project := range projects {
		combinedMatrix[i] = make([]interface{}, len(allDependencies))
		for j, depName := range allDependencies {
			if dep, exists := allProjectDeps[project.ID][depName]; exists {
				combinedMatrix[i][j] = map[string]interface{}{
					"version":        dep.Version,
					"latest_version": dep.LatestVersion,
					"constraint":     dep.Constraint,
					"is_internal":    dep.IsInternal,
					"ecosystem":      dep.Ecosystem,
				}
			} else {
				combinedMatrix[i][j] = nil
			}
		}
	}

	return dependencyObjects, combinedMatrix
}

// GenerateMatrix creates a simple dependency matrix for all projects
func (g *Generator) GenerateMatrix(ctx context.Context, projects []*domain.Project) map[string]interface{} {
	// Filter out projects with zero dependencies
	filteredProjects := g.filterProjectsWithDependencies(projects)

	// Create combined matrix
	allDependencies, combinedMatrix := g.createCombinedMatrix(filteredProjects)

	return map[string]interface{}{
		"dependencies": allDependencies,
		"projects":     filteredProjects,
		"matrix":       combinedMatrix,
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
