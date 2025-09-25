package main

import (
	"context"
	"di-matrix-cli/internal/classifier"
	"di-matrix-cli/internal/config"
	"di-matrix-cli/internal/generator"
	"di-matrix-cli/internal/gitlab"
	"di-matrix-cli/internal/logger"
	"di-matrix-cli/internal/parser"
	"di-matrix-cli/internal/scanner"
	"di-matrix-cli/internal/usecases"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// Version information - set by build-time ldflags
var (
	version   = "dev"
	commit    = "unknown"
	buildTime = "unknown"
)

var (
	configFile string
	outputFile string
	title      string
	debug      bool
	timeout    int
	language   string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "di-matrix-cli",
	Short: "Dependency Matrix CLI - Analyze GitLab repositories and generate dependency matrices",
	Long: `A command-line tool that analyzes multiple GitLab repositories to generate
comprehensive dependency matrices using event-driven architecture. The tool uses GitLab API
for repository access, leverages dependency parsers for multi-language support, handles
monorepos with mixed project types, and generates an interactive HTML report through
scalable worker pools.`,
}

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Long:  "Display version, commit hash, and build time information.",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("di-matrix-cli %s\n", version)
		fmt.Printf("Commit: %s\n", commit)
		fmt.Printf("Built: %s\n", buildTime)
	},
}

// analyzeCmd represents the analyze command
var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Analyze repositories and generate dependency matrix",
	Long: `Analyze the configured GitLab repositories and generate a comprehensive
dependency matrix report in HTML format using event-driven architecture.
The analysis runs asynchronously with real-time progress reporting through
event-driven worker pools.`,
	RunE: runAnalyze,
}

func setupCommands() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(analyzeCmd)

	// Global flags
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "Path to configuration file (required)")
	rootCmd.PersistentFlags().BoolP("version", "v", false, "Show version information")

	// Handle --version flag on root command
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if showVersion, _ := cmd.Flags().GetBool("version"); showVersion {
			fmt.Printf("di-matrix-cli %s\n", version)
			fmt.Printf("Commit: %s\n", commit)
			fmt.Printf("Built: %s\n", buildTime)
			os.Exit(0)
		}
		return nil
	}

	// Add pre-run validation for analyze command to check required config flag
	analyzeCmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if configFile == "" {
			return fmt.Errorf("config flag is required for analyze command")
		}
		return nil
	}

	// Analyze command flags
	analyzeCmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output HTML file path (overrides config)")
	analyzeCmd.Flags().StringVarP(&title, "title", "t", "", "Report title (overrides config)")
	analyzeCmd.Flags().BoolVarP(&debug, "debug", "d", false, "Enable debug logging with verbose output")
	analyzeCmd.Flags().IntVarP(&timeout, "timeout", "", 0,
		"Analysis timeout in minutes (overrides config, 0 = use config default)")
	analyzeCmd.Flags().
		StringVarP(&language, "language", "l", "python", "Programming language to analyze (go, nodejs, java, python)")
	if err := analyzeCmd.MarkFlagRequired("language"); err != nil {
		panic(fmt.Sprintf("failed to mark language flag as required: %v", err))
	}

	// Bind flags to viper
	if err := viper.BindPFlag("output.html_file", analyzeCmd.Flags().Lookup("output")); err != nil {
		panic(fmt.Sprintf("failed to bind output flag: %v", err))
	}
	if err := viper.BindPFlag("output.title", analyzeCmd.Flags().Lookup("title")); err != nil {
		panic(fmt.Sprintf("failed to bind title flag: %v", err))
	}
	if err := viper.BindPFlag("timeout.analysis_timeout_minutes", analyzeCmd.Flags().Lookup("timeout")); err != nil {
		panic(fmt.Sprintf("failed to bind timeout flag: %v", err))
	}
}

func main() {
	setupCommands()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runAnalyze(cmd *cobra.Command, args []string) error {
	fmt.Println("🔍 Starting dependency matrix analysis...")

	// Validate language flag
	validLanguages := map[string]bool{
		"go":     true,
		"nodejs": true,
		"java":   true,
		"python": true,
	}
	if !validLanguages[language] {
		return fmt.Errorf("invalid language '%s'. Supported languages: go, nodejs, java, python", language)
	}

	fmt.Printf("🎯 Analyzing %s projects only\n", language)

	// Handle debug flag manually since it's a boolean
	if debug {
		viper.Set("logging.level", "debug")
	}

	// Load configuration
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Determine timeout duration (CLI flag overrides config)
	timeoutMinutes := cfg.Timeout.AnalysisTimeoutMinutes
	if timeout > 0 {
		timeoutMinutes = timeout
	}
	timeoutDuration := time.Duration(timeoutMinutes) * time.Minute

	fmt.Printf("⏱️  Analysis timeout: %v\n", timeoutDuration)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeoutDuration)
	defer cancel()

	// Set debug level if debug flag is enabled
	if debug {
		logger.SetLevel(zap.DebugLevel)
	}

	// Create dependencies
	l := logger.GetLogger()

	// Initialize GitLab client
	gitlabClient, err := gitlab.NewClient(cfg.GitLab.BaseURL, cfg.GitLab.Token, l)
	if err != nil {
		return fmt.Errorf("failed to create GitLab client: %w", err)
	}

	// Initialize scanner
	fileScanner := scanner.NewScanner(gitlabClient, l)

	// Initialize parser
	dependencyParser := parser.NewParser()

	// Initialize classifier with internal patterns
	dependencyClassifier := classifier.NewClassifier(cfg.Internal.Patterns)

	// Initialize generator
	reportGenerator := generator.NewGenerator(cfg.Output.HTMLFile)

	// Create analyze use case with dependency injection
	analyzeUseCase := usecases.NewAnalyzeUseCase(
		ctx,
		gitlabClient,
		fileScanner,
		dependencyParser,
		dependencyClassifier,
		reportGenerator,
		l,
	)

	// Extract repository URLs from config
	repositoryURLs := make([]string, len(cfg.Repositories))
	for i, repo := range cfg.Repositories {
		repositoryURLs[i] = repo.URL
	}

	response, err := analyzeUseCase.Execute(repositoryURLs, language)
	if err != nil {
		return fmt.Errorf("failed to analyze dependency matrix: %w", err)
	}

	l.Info("Analysis completed successfully", zap.Any("response", response))

	// Print summary
	fmt.Println("\n🎉 Analysis completed successfully!")
	fmt.Printf("📈 Summary:\n")
	fmt.Printf("  • Total Projects: %d\n", response.TotalProjects)
	fmt.Printf("  • Total Dependencies: %d\n", response.TotalDependencies)
	fmt.Printf("  • Internal Dependencies: %d\n", response.InternalCount)
	fmt.Printf("  • External Dependencies: %d\n", response.ExternalCount)
	return nil
}
