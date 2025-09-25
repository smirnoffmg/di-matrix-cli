package parser

import (
	"bytes"
	"context"
	"di-matrix-cli/internal/domain"
	"fmt"
	"strings"

	"github.com/aquasecurity/trivy/pkg/dependency/parser/golang/mod"
	"github.com/aquasecurity/trivy/pkg/dependency/parser/java/pom"
	"github.com/aquasecurity/trivy/pkg/dependency/parser/nodejs/npm"
	"github.com/aquasecurity/trivy/pkg/dependency/parser/nodejs/packagejson"
	"github.com/aquasecurity/trivy/pkg/dependency/parser/nodejs/yarn"
	"github.com/aquasecurity/trivy/pkg/dependency/parser/python/pip"
	"github.com/aquasecurity/trivy/pkg/dependency/parser/python/pipenv"
	"github.com/aquasecurity/trivy/pkg/dependency/parser/python/poetry"
	"github.com/aquasecurity/trivy/pkg/dependency/parser/python/pyproject"
	"github.com/aquasecurity/trivy/pkg/dependency/parser/python/uv"
	ftypes "github.com/aquasecurity/trivy/pkg/fanal/types"
	xio "github.com/aquasecurity/trivy/pkg/x/io"
)

// Parser handles dependency file parsing using Trivy
type Parser struct{}

// NewParser creates a new dependency parser
func NewParser() *Parser {
	return &Parser{}
}

// ParseFile parses a dependency file and extracts dependencies
func (p *Parser) ParseFile(ctx context.Context, file *domain.DependencyFile) ([]*domain.Dependency, error) {
	// Create a reader from the file content
	reader, err := xio.NewReadSeekerAt(bytes.NewReader(file.Content))
	if err != nil {
		return nil, fmt.Errorf("failed to create reader: %w", err)
	}

	var trivyPackages []ftypes.Package
	var trivyDeps []ftypes.Dependency

	switch file.Language {
	case "go":
		trivyPackages, trivyDeps, err = p.parseGoFileWithTrivy(reader, file.Path)
	case "nodejs":
		trivyPackages, trivyDeps, err = p.parseNodeJSFileWithTrivy(reader, file.Path)
	case "java":
		trivyPackages, trivyDeps, err = p.parseJavaFileWithTrivy(reader, file.Path)
	case "python":
		trivyPackages, trivyDeps, err = p.parsePythonFileWithTrivy(reader, file.Path)
	default:
		return nil, fmt.Errorf("unsupported language: %s", file.Language)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to parse %s file %s: %w", file.Language, file.Path, err)
	}

	// Convert Trivy packages to domain dependencies
	var dependencies []*domain.Dependency
	for i := range trivyPackages {
		pkg := &trivyPackages[i]
		dependencies = append(dependencies, &domain.Dependency{
			Name:          pkg.Name,
			Version:       pkg.Version,
			LatestVersion: pkg.Version, // TODO: Fetch actual latest version from package registry
			Constraint:    p.extractConstraint(pkg),
			MinVersion:    p.extractMinVersion(pkg),
			MaxVersion:    p.extractMaxVersion(pkg),
			IsInternal:    p.isInternalDependency(pkg.Name),
			Ecosystem:     p.getEcosystem(file.Language),
		})
	}

	// Log dependencies for debugging (we don't use them in the domain model yet)
	_ = trivyDeps

	return dependencies, nil
}

// CanParse checks if this parser can handle the given file type
func (p *Parser) CanParse(filePath string) bool {
	fileName := p.getFileName(filePath)

	supportedFiles := map[string][]string{
		"go":     {"go.mod", "go.sum"},
		"nodejs": {"package.json", "package-lock.json", "yarn.lock"},
		"java":   {"pom.xml"},
		"python": {"requirements.txt", "Pipfile", "poetry.lock", "uv.lock", "pyproject.toml"},
	}

	for _, files := range supportedFiles {
		for _, file := range files {
			if fileName == file {
				return true
			}
		}
	}
	return false
}

// parseGoFileWithTrivy parses Go dependencies using Trivy's Go parser
func (p *Parser) parseGoFileWithTrivy(
	reader xio.ReadSeekerAt,
	fileName string,
) ([]ftypes.Package, []ftypes.Dependency, error) {
	fileName = p.getFileName(fileName)

	switch fileName {
	case "go.mod":
		parser := mod.NewParser(false, false)
		packages, deps, err := parser.Parse(reader)
		if err != nil {
			return nil, nil, fmt.Errorf("go.mod parser error: %w", err)
		}
		return packages, deps, nil
	case "go.sum":
		// go.sum files don't contain dependency information, they contain checksums
		// Return empty results instead of an error
		return []ftypes.Package{}, []ftypes.Dependency{}, nil
	default:
		return nil, nil, fmt.Errorf("unsupported Go file: %s", fileName)
	}
}

// parseNodeJSFileWithTrivy parses Node.js dependencies using Trivy's Node.js parsers
func (p *Parser) parseNodeJSFileWithTrivy(
	reader xio.ReadSeekerAt,
	fileName string,
) ([]ftypes.Package, []ftypes.Dependency, error) {
	fileName = p.getFileName(fileName)

	switch fileName {
	case "package-lock.json":
		// package-lock.json is more important - contains exact versions of all dependencies
		parser := npm.NewParser()
		return parser.Parse(reader)
	case "package.json":
		// Use Trivy's package.json parser
		parser := packagejson.NewParser()
		pkg, err := parser.Parse(reader)
		if err != nil {
			return nil, nil, err
		}
		// Trivy's packagejson parser returns a single package
		// The dependencies are stored in the Package's dependencies field
		var packages []ftypes.Package
		if pkg.Name != "" {
			packages = append(packages, pkg.Package)
		}
		return packages, nil, nil
	case "yarn.lock":
		parser := yarn.NewParser()
		packages, deps, _, err := parser.Parse(reader)
		return packages, deps, err
	default:
		return nil, nil, fmt.Errorf("unsupported Node.js file: %s", fileName)
	}
}

// parseJavaFileWithTrivy parses Java dependencies using Trivy's Java parser
func (p *Parser) parseJavaFileWithTrivy(
	reader xio.ReadSeekerAt,
	fileName string,
) ([]ftypes.Package, []ftypes.Dependency, error) {
	fileName = p.getFileName(fileName)

	if fileName == "pom.xml" {
		parser := pom.NewParser("") // Use default options
		return parser.Parse(reader)
	}
	return nil, nil, fmt.Errorf("unsupported Java file: %s", fileName)
}

// parsePythonFileWithTrivy parses Python dependencies using Trivy's Python parsers
func (p *Parser) parsePythonFileWithTrivy(
	reader xio.ReadSeekerAt,
	fileName string,
) ([]ftypes.Package, []ftypes.Dependency, error) {
	fileName = p.getFileName(fileName)

	switch fileName {
	case "requirements.txt":
		parser := pip.NewParser(false)
		return parser.Parse(reader)
	case "Pipfile":
		parser := pipenv.NewParser()
		return parser.Parse(reader)
	case "poetry.lock":
		parser := poetry.NewParser()
		return parser.Parse(reader)
	case "uv.lock":
		parser := uv.NewParser()
		return parser.Parse(reader)
	case "pyproject.toml":
		// For pyproject.toml, we need to handle it differently since it doesn't return packages directly
		// We'll parse it to get dependency names but won't have versions
		parser := pyproject.NewParser()
		pyprojectData, err := parser.Parse(reader)
		if err != nil {
			return nil, nil, err
		}

		// Convert pyproject.toml dependencies to packages (without versions)
		var packages []ftypes.Package
		mainDeps := pyprojectData.MainDeps()
		for _, depName := range mainDeps.Items() {
			packages = append(packages, ftypes.Package{
				Name:    depName,
				Version: "", // pyproject.toml doesn't contain exact versions
			})
		}

		return packages, nil, nil
	default:
		return nil, nil, fmt.Errorf("unsupported Python file: %s", fileName)
	}
}

// Helper methods

func (p *Parser) getFileName(filePath string) string {
	parts := strings.Split(filePath, "/")
	return parts[len(parts)-1]
}

func (p *Parser) extractConstraint(pkg *ftypes.Package) string {
	// For now, use version as constraint
	// In a more sophisticated implementation, we could parse the original constraint
	return pkg.Version
}

func (p *Parser) extractMinVersion(pkg *ftypes.Package) string {
	// For now, use version as min version
	// In a more sophisticated implementation, we could parse version ranges
	return pkg.Version
}

func (p *Parser) extractMaxVersion(pkg *ftypes.Package) string {
	// For now, return empty
	// In a more sophisticated implementation, we could parse version ranges
	return ""
}

func (p *Parser) isInternalDependency(name string) bool {
	// For now, consider everything external
	// In a more sophisticated implementation, we could check against internal domains
	return false
}

func (p *Parser) getEcosystem(language string) string {
	switch language {
	case "go":
		return "go-modules"
	case "nodejs":
		return "npm"
	case "java":
		return "maven"
	case "python":
		return "pip"
	default:
		return language
	}
}
