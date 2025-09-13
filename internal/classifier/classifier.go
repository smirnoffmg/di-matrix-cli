package classifier

import (
	"context"
	"di-matrix-cli/internal/domain"
	"path/filepath"
	"strings"
)

// Classifier determines if dependencies are internal or external
type Classifier struct {
	internalPatterns []string
}

// NewClassifier creates a new dependency classifier
func NewClassifier(internalPatterns []string) *Classifier {
	return &Classifier{
		internalPatterns: internalPatterns,
	}
}

// ClassifyDependencies classifies a list of dependencies
func (c *Classifier) ClassifyDependencies(
	ctx context.Context,
	dependencies []*domain.Dependency,
) ([]*domain.Dependency, error) {
	if dependencies == nil {
		return nil, nil
	}

	// Classify each dependency
	for _, dep := range dependencies {
		if dep != nil {
			dep.IsInternal = c.IsInternal(ctx, dep)
		}
	}

	return dependencies, nil
}

// IsInternal checks if a single dependency is internal
func (c *Classifier) IsInternal(ctx context.Context, dependency *domain.Dependency) bool {
	if dependency == nil || dependency.Name == "" {
		return false
	}

	// Check against all internal patterns
	for _, pattern := range c.internalPatterns {
		if c.matchesPattern(dependency.Name, pattern) {
			return true
		}
	}

	return false
}

// matchesPattern checks if a dependency name matches a given pattern
func (c *Classifier) matchesPattern(name, pattern string) bool {
	// Handle exact matches
	if name == pattern {
		return true
	}

	// Handle wildcard patterns
	if c.matchesWildcardPattern(name, pattern) {
		return true
	}

	// Handle prefix patterns
	if c.matchesPrefixPattern(name, pattern) {
		return true
	}

	// Handle suffix patterns
	if c.matchesSuffixPattern(name, pattern) {
		return true
	}

	// Handle contains patterns
	if c.matchesContainsPattern(name, pattern) {
		return true
	}

	return false
}

// matchesWildcardPattern checks if name matches a wildcard pattern
func (c *Classifier) matchesWildcardPattern(name, pattern string) bool {
	if !strings.Contains(pattern, "*") {
		return false
	}

	matched, err := filepath.Match(pattern, name)
	return err == nil && matched
}

// matchesPrefixPattern checks if name matches a prefix pattern
func (c *Classifier) matchesPrefixPattern(name, pattern string) bool {
	if !strings.HasSuffix(pattern, "/") && !strings.HasSuffix(pattern, ".") {
		return false
	}

	prefix := strings.TrimSuffix(pattern, "/")
	prefix = strings.TrimSuffix(prefix, ".")
	return strings.HasPrefix(name, prefix)
}

// matchesSuffixPattern checks if name matches a suffix pattern
func (c *Classifier) matchesSuffixPattern(name, pattern string) bool {
	if !strings.HasPrefix(pattern, "/") && !strings.HasPrefix(pattern, ".") {
		return false
	}

	suffix := strings.TrimPrefix(pattern, "/")
	suffix = strings.TrimPrefix(suffix, ".")
	return strings.HasSuffix(name, suffix)
}

// matchesContainsPattern checks if name contains the pattern
func (c *Classifier) matchesContainsPattern(name, pattern string) bool {
	// Only match contains if pattern doesn't have special characters
	hasSpecialChars := strings.Contains(pattern, "*") ||
		strings.HasSuffix(pattern, "/") ||
		strings.HasSuffix(pattern, ".") ||
		strings.HasPrefix(pattern, "/") ||
		strings.HasPrefix(pattern, ".")

	return !hasSpecialChars && strings.Contains(name, pattern)
}
