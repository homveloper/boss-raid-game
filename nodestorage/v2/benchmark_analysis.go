package v2

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// BenchmarkResult represents a single benchmark result
type BenchmarkResult struct {
	Name         string        `json:"name"`
	Operations   int           `json:"operations"`
	NsPerOp      float64       `json:"ns_per_op"`
	BytesPerOp   int64         `json:"bytes_per_op"`
	AllocsPerOp  int64         `json:"allocs_per_op"`
	MBPerSecond  float64       `json:"mb_per_second"`
	Elapsed      time.Duration `json:"elapsed"`
	Parallelism  int           `json:"parallelism"`
	CacheType    string        `json:"cache_type"`
	Operation    string        `json:"operation"`
	DocumentSize int           `json:"document_size"`
}

// ParseBenchmarkResults parses benchmark results from a JSON file
func ParseBenchmarkResults(filePath string) ([]BenchmarkResult, error) {
	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Parse JSON
	var results []BenchmarkResult
	err = json.Unmarshal(data, &results)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Extract additional information from benchmark names
	for i := range results {
		// Parse benchmark name
		// Format: BenchmarkXxx/Cache=yyy/Op=zzz-N or BenchmarkXxx/Size=nnn/Op=zzz-N
		parts := strings.Split(results[i].Name, "/")
		if len(parts) >= 3 {
			for _, part := range parts[1:] {
				if strings.HasPrefix(part, "Cache=") {
					results[i].CacheType = strings.TrimPrefix(part, "Cache=")
				} else if strings.HasPrefix(part, "Op=") {
					opPart := strings.TrimPrefix(part, "Op=")
					results[i].Operation = strings.Split(opPart, "-")[0]
				} else if strings.HasPrefix(part, "Size=") {
					sizePart := strings.TrimPrefix(part, "Size=")
					fmt.Sscanf(sizePart, "%d", &results[i].DocumentSize)
				}
			}
		}
	}

	return results, nil
}

// GenerateComparisonReport generates a comparison report from benchmark results
func GenerateComparisonReport(results []BenchmarkResult, outputDir string) error {
	// Create output directory if it doesn't exist
	err := os.MkdirAll(outputDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Group results by operation
	operationResults := make(map[string][]BenchmarkResult)
	for _, result := range results {
		operationResults[result.Operation] = append(operationResults[result.Operation], result)
	}

	// Generate report for each operation
	for operation, opResults := range operationResults {
		// Sort results by cache type
		sort.Slice(opResults, func(i, j int) bool {
			if opResults[i].CacheType != opResults[j].CacheType {
				return opResults[i].CacheType < opResults[j].CacheType
			}
			return opResults[i].DocumentSize < opResults[j].DocumentSize
		})

		// Create report file
		reportPath := filepath.Join(outputDir, fmt.Sprintf("%s_report.md", operation))
		reportFile, err := os.Create(reportPath)
		if err != nil {
			return fmt.Errorf("failed to create report file: %w", err)
		}
		defer reportFile.Close()

		// Write report header
		fmt.Fprintf(reportFile, "# Benchmark Report: %s\n\n", operation)
		fmt.Fprintf(reportFile, "## Performance Comparison\n\n")
		fmt.Fprintf(reportFile, "| Cache Type | Document Size | Operations | ns/op | MB/s | Allocs/op |\n")
		fmt.Fprintf(reportFile, "|------------|--------------|------------|-------|------|----------|\n")

		// Write report data
		for _, result := range opResults {
			fmt.Fprintf(reportFile, "| %s | %d | %d | %.2f | %.2f | %d |\n",
				result.CacheType,
				result.DocumentSize,
				result.Operations,
				result.NsPerOp,
				result.MBPerSecond,
				result.AllocsPerOp,
			)
		}

		// Write performance analysis
		fmt.Fprintf(reportFile, "\n## Performance Analysis\n\n")

		// Group results by document size
		sizeResults := make(map[int][]BenchmarkResult)
		for _, result := range opResults {
			sizeResults[result.DocumentSize] = append(sizeResults[result.DocumentSize], result)
		}

		// Analyze each document size
		for size, sizeResult := range sizeResults {
			fmt.Fprintf(reportFile, "### Document Size: %d bytes\n\n", size)

			// Sort by performance (ns/op)
			sort.Slice(sizeResult, func(i, j int) bool {
				return sizeResult[i].NsPerOp < sizeResult[j].NsPerOp
			})

			// Find fastest and slowest
			fastest := sizeResult[0]
			slowest := sizeResult[len(sizeResult)-1]

			fmt.Fprintf(reportFile, "- Fastest: **%s** (%.2f ns/op)\n", fastest.CacheType, fastest.NsPerOp)
			fmt.Fprintf(reportFile, "- Slowest: **%s** (%.2f ns/op)\n", slowest.CacheType, slowest.NsPerOp)
			fmt.Fprintf(reportFile, "- Performance difference: **%.2fx**\n\n", slowest.NsPerOp/fastest.NsPerOp)

			// Write performance comparison chart (ASCII art)
			fmt.Fprintf(reportFile, "```\n")
			fmt.Fprintf(reportFile, "Performance comparison (lower is better):\n\n")

			// Find max ns/op for scaling
			maxNsPerOp := 0.0
			for _, result := range sizeResult {
				if result.NsPerOp > maxNsPerOp {
					maxNsPerOp = result.NsPerOp
				}
			}

			// Generate chart
			chartWidth := 50
			for _, result := range sizeResult {
				barLength := int((result.NsPerOp / maxNsPerOp) * float64(chartWidth))
				bar := strings.Repeat("█", barLength)
				fmt.Fprintf(reportFile, "%-10s %s %.2f ns/op\n", result.CacheType, bar, result.NsPerOp)
			}
			fmt.Fprintf(reportFile, "```\n\n")
		}

		// Write recommendations
		fmt.Fprintf(reportFile, "## Recommendations\n\n")

		// Find best overall cache type
		cachePerformance := make(map[string]float64)
		cacheCount := make(map[string]int)
		for _, result := range opResults {
			cachePerformance[result.CacheType] += result.NsPerOp
			cacheCount[result.CacheType]++
		}

		// Calculate average performance
		for cacheType := range cachePerformance {
			cachePerformance[cacheType] /= float64(cacheCount[cacheType])
		}

		// Sort cache types by average performance
		var cacheTypes []string
		for cacheType := range cachePerformance {
			cacheTypes = append(cacheTypes, cacheType)
		}
		sort.Slice(cacheTypes, func(i, j int) bool {
			return cachePerformance[cacheTypes[i]] < cachePerformance[cacheTypes[j]]
		})

		// Write recommendations
		fmt.Fprintf(reportFile, "Based on the benchmark results for the **%s** operation:\n\n", operation)

		if len(cacheTypes) > 0 {
			fmt.Fprintf(reportFile, "1. **%s** provides the best overall performance.\n", cacheTypes[0])
		}

		// Document size specific recommendations
		fmt.Fprintf(reportFile, "2. For different document sizes:\n")
		for size, sizeResult := range sizeResults {
			// Sort by performance (ns/op)
			sort.Slice(sizeResult, func(i, j int) bool {
				return sizeResult[i].NsPerOp < sizeResult[j].NsPerOp
			})

			fmt.Fprintf(reportFile, "   - For %d byte documents: **%s** is recommended.\n",
				size, sizeResult[0].CacheType)
		}

		// General advice
		fmt.Fprintf(reportFile, "\n### General Advice\n\n")
		fmt.Fprintf(reportFile, "- **Memory cache** provides the fastest access times but is limited by available memory.\n")
		fmt.Fprintf(reportFile, "- **BadgerDB cache** offers a good balance between performance and persistence.\n")
		fmt.Fprintf(reportFile, "- **Redis cache** is suitable for distributed environments where cache sharing is required.\n")
		fmt.Fprintf(reportFile, "- **No cache** might be appropriate for write-heavy workloads with infrequent reads.\n")
	}

	return nil
}

// RunBenchmarkAnalysis runs benchmarks and generates analysis reports
func RunBenchmarkAnalysis(benchCommand, outputDir string) error {
	// Create temporary file for benchmark results
	tempFile, err := os.CreateTemp("", "benchmark-*.json")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer os.Remove(tempFile.Name())
	tempFile.Close()

	// Run benchmarks and save results to JSON
	cmd := fmt.Sprintf("%s -json > %s", benchCommand, tempFile.Name())
	err = exec.Command("sh", "-c", cmd).Run()
	if err != nil {
		return fmt.Errorf("failed to run benchmarks: %w", err)
	}

	// Parse benchmark results
	results, err := ParseBenchmarkResults(tempFile.Name())
	if err != nil {
		return fmt.Errorf("failed to parse benchmark results: %w", err)
	}

	// Generate comparison report
	err = GenerateComparisonReport(results, outputDir)
	if err != nil {
		return fmt.Errorf("failed to generate comparison report: %w", err)
	}

	return nil
}
