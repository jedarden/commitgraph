// Package clierror provides standard error handling infrastructure for CLI entry points.
//
// This file contains comprehensive examples of error wrapping patterns
// that can be used as reference when implementing or refactoring CLI commands.
package clierror

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Example 1: Basic usage error handling
// Use this pattern for validating command-line flags and arguments
func example1_UsageError() error {
	inputPath := "" // would come from flag.String("input", "", "...")

	// Usage errors should use CategoryUsage (exit code 2)
	if inputPath == "" {
		return NewUsage(
			"missing required flag",
			errors.New("--input is required"),
		)
	}

	// For invalid flag values
	if inputPath == "." {
		return NewUsage(
			"invalid flag value",
			fmt.Errorf("--input cannot be '.'"),
		)
	}

	return nil
}

// Example 2: File reading with context
// Use this pattern when reading files - include the file path in context
func example2_FileReadingWithContext(filepath string) error {
	// Input errors should use CategoryInput (exit code 3)
	// Always include the file path in context for better error messages
	data, err := os.ReadFile(filepath)
	if err != nil {
		return NewInputWithContext(
			"failed to read input file",
			filepath,
			err,
		)
	}

	_ = data // Process file contents
	return nil
}

// Example 3: JSON parsing with structured error
// Use this pattern when parsing structured data
func example3_JSONParsing(data []byte) error {
	var config struct {
		Host string `json:"host"`
		Port int    `json:"port"`
	}

	// Parsing errors are input errors
	if err := json.Unmarshal(data, &config); err != nil {
		return WrapInput(
			"failed to parse configuration JSON",
			err,
		)
	}

	// Validate parsed configuration
	if config.Host == "" {
		return NewInput(
			"missing required field in configuration",
			errors.New("host cannot be empty"),
		)
	}

	if config.Port < 1 || config.Port > 65535 {
		return NewInput(
			"invalid port number in configuration",
			fmt.Errorf("port %d out of range [1-65535]", config.Port),
		)
	}

	return nil
}

// Example 4: Network operation with status code handling
// Use this pattern for HTTP requests and network operations
func example4_NetworkOperation(url string) error {
	// Network errors should use CategoryNetwork (exit code 4)
	// This would typically use http.Get, but we'll simulate it
	_, err := fmt.Sprintf("simulated HTTP GET to %s", url)
	if err != nil {
		return WrapNetwork(
			"failed to fetch data from server",
			err,
		)
	}

	// Simulating a non-200 status code
	statusCode := 500
	if statusCode != 200 {
		return NewNetwork(
			"server returned error status",
			fmt.Errorf("HTTP %d", statusCode),
		)
	}

	return nil
}

// Example 5: Database connection with multiple error types
// Use this pattern when errors could have different causes
func example5_DatabaseConnection(host, user, password string) error {
	// Missing required fields are usage errors
	if host == "" {
		return NewUsage(
			"missing required database parameter",
			errors.New("--db-host is required"),
		)
	}

	if user == "" {
		return NewUsage(
			"missing required database parameter",
			errors.New("--db-user is required"),
		)
	}

	if password == "" {
		return NewUsage(
			"missing required database parameter",
			errors.New("--db-password is required"),
		)
	}

	// Connection failures are network errors
	err := fmt.Sprintf("connect to %s@%s", user, host)
	if err != nil {
		return WrapNetwork(
			"failed to connect to database",
			err,
		)
	}

	// Authentication failures are permission errors
	if user != "admin" {
		return NewPermission(
			"database authentication failed",
			errors.New("invalid credentials"),
		)
	}

	return nil
}

// Example 6: Directory walking with error accumulation
// Use this pattern when processing multiple files and want to continue on errors
func example6_DirectoryWalk(dir string) error {
	var firstError error
	fileCount := 0
	errorCount := 0

	// Walk directory and collect errors
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			errorCount++
			// Store the first error but continue processing
			if firstError == nil {
				firstError = WrapInputWithContext(
					"failed to access directory entry",
					path,
					err,
				)
			}
			return nil // Continue walking
		}

		if !d.IsDir() {
			fileCount++
		}

		return nil
	})

	// Directory walk itself failed
	if err != nil {
		return WrapInput(
			"failed to walk directory",
			err,
		)
	}

	// Report accumulated errors
	if errorCount > 0 {
		fmt.Printf("Warning: processed %d files with %d errors\n", fileCount, errorCount)
		return firstError
	}

	return nil
}

// Example 7: Conditional error wrapping
// Use this pattern to add context only when an error occurs
func example7_ConditionalWrapping(data []byte) error {
	// These helpers only wrap if err != nil
	var result map[string]interface{}

	if err := json.Unmarshal(data, &result); err != nil {
		// WrapInput returns nil if err is nil, otherwise returns wrapped error
		return WrapInput("failed to parse JSON data", err)
	}

	// Can chain multiple operations
	if result["timeout"] != nil {
		timeout, ok := result["timeout"].(float64)
		if !ok {
			return WrapInput("invalid timeout type in configuration",
				errors.New("timeout must be a number"))
		}
		_ = timeout
	}

	return nil
}

// Example 8: Error with custom exit code
// Use this pattern when you need a specific exit code for an error
func example8_CustomExitCode() error {
	// Override default exit code for a specific error
	return NewWrapWithExitCode(
		CategoryInput,
		"validation failed",
		10, // Custom exit code instead of default 3
		errors.New("data format invalid"),
	)
}

// Example 9: Internal error for bugs
// Use this pattern for errors that should never occur (invariant violations)
func example9_InternalError(slice []int, index int) error {
	// This should never happen if the caller provides valid input
	if index < 0 || index >= len(slice) {
		return NewInternal(
			"internal invariant violation",
			fmt.Errorf("index %d out of bounds [0, %d)", index, len(slice)),
		)
	}

	_ = slice[index]
	return nil
}

// Example 10: Transient error for retryable failures
// Use this pattern for temporary failures that might succeed on retry
func example10_TransientError(attempt int) error {
	if attempt < 3 {
		return NewTransient(
			"temporary failure",
			errors.New("service temporarily unavailable"),
		)
	}

	return nil
}

// Example 11: Complex multi-step operation with layered errors
// Use this pattern when an operation has multiple steps that can fail
func example11_MultiStepOperation(configFile, inputFile string) error {
	// Step 1: Read configuration (usage error if missing)
	if configFile == "" {
		return NewUsage(
			"missing required flag",
			errors.New("--config is required"),
		)
	}

	// Step 2: Read config file (input error with context)
	configData, err := os.ReadFile(configFile)
	if err != nil {
		return NewInputWithContext(
			"failed to read configuration file",
			configFile,
			err,
		)
	}

	// Step 3: Parse configuration (input error)
	var config struct {
		OutputPath string `json:"output"`
	}
	if err := json.Unmarshal(configData, &config); err != nil {
		return WrapInput(
			"failed to parse configuration JSON",
			err,
		)
	}

	// Step 4: Read input file (input error with context)
	inputData, err := os.ReadFile(inputFile)
	if err != nil {
		return NewInputWithContext(
			"failed to read input file",
			inputFile,
			err,
		)
	}

	_ = inputData
	_ = config

	return nil
}

// Example 12: Complete CLI run() function
// This shows a complete example of how to structure a run() function
func example12_CompleteRunFunction() error {
	// This would be called from main() via clierror.Run(run)
	//
	// func main() {
	//     clierror.Run(run)
	// }

	// Phase 1: Validate flags (usage errors)
	inputPath := "example.txt"
	if inputPath == "" {
		return NewUsage(
			"missing required flag",
			errors.New("--input is required"),
		)
	}

	outputPath := "output.txt"
	if outputPath == "" {
		return NewUsage(
			"missing required flag",
			errors.New("--output is required"),
		)
	}

	// Phase 2: Read and validate input (input errors)
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return NewInputWithContext(
			"failed to read input file",
			inputPath,
			err,
		)
	}

	if len(data) == 0 {
		return NewInput(
			"input file is empty",
			fmt.Errorf("%s contains no data", inputPath),
		)
	}

	// Phase 3: Process data
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return WrapInput(
			"failed to parse input JSON",
			err,
		)
	}

	// Phase 4: Write output (input error for write failures)
	outputData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return WrapInternal(
			"failed to serialize output JSON",
			err,
		)
	}

	if err := os.WriteFile(outputPath, outputData, 0644); err != nil {
		return NewInputWithContext(
			"failed to write output file",
			outputPath,
			err,
		)
	}

	fmt.Printf("Successfully processed %s → %s\n", inputPath, outputPath)
	return nil
}

// Example 13: Error accumulation pattern
// Use this pattern when you want to collect multiple errors before failing
func example13_ErrorAccumulation(files []string) error {
	var errors []error
	successCount := 0

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			errors = append(errors,
				NewInputWithContext(
					"failed to read file",
					file,
					err,
				),
			)
			continue
		}

		_ = data
		successCount++
	}

	// If we had any errors, report them
	if len(errors) > 0 {
		fmt.Printf("Warning: only %d/%d files processed successfully\n",
			successCount, len(files))

		// Return the first error with context about total failures
		return NewInput(
			fmt.Sprintf("failed to process %d/%d files", len(errors), len(files)),
			errors[0],
		)
	}

	return nil
}

// Example 14: Permission error handling
// Use this pattern for file permissions and access control
func example14_PermissionHandling(path string) error {
	// Try to read a file
	_, err := os.ReadFile(path)
	if err != nil {
		// Check if it's a permission error
		if os.IsPermission(err) {
			return NewPermission(
				"insufficient permissions to read file",
				fmt.Errorf("cannot read %s: permission denied", path),
			)
		}

		// Otherwise it's an input error
		return NewInputWithContext(
			"failed to read file",
			path,
			err,
		)
	}

	return nil
}

// Example 15: Panic-safe processing
// This pattern shows how to structure code that might panic
// Note: The clierror.Run() wrapper handles panics automatically
func example15_PanicSafeProcessing(data []byte) error {
	// This function is called from run(), which is called from clierror.Run
	// If this panics, clierror.Run will catch it and exit cleanly

	var result interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return WrapInput("failed to parse JSON", err)
	}

	// Simulate potential panic (normally this would be bad code)
	// result.(string) // This would panic if result is not a string
	// But clierror.Run would catch it and exit with code 1

	_ = result
	return nil
}
