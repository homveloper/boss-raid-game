package main

import (
	"fmt"
	"log"
	v2 "nodestorage/v2"
	"os"
	"path/filepath"
)

func main() {
	// Get benchmark results directory
	resultsDir := "benchmark_results"
	if len(os.Args) > 1 {
		resultsDir = os.Args[1]
	}

	// Get output directory
	outputDir := filepath.Join(resultsDir, "reports")
	if len(os.Args) > 2 {
		outputDir = os.Args[2]
	}

	// Parse cache benchmark results
	cacheBenchmarkPath := filepath.Join(resultsDir, "cache_benchmark.json")
	cacheResults, err := v2.ParseBenchmarkResults(cacheBenchmarkPath)
	if err != nil {
		log.Printf("Warning: Failed to parse cache benchmark results: %v", err)
	} else {
		// Generate cache benchmark report
		cacheOutputDir := filepath.Join(outputDir, "cache")
		err = v2.GenerateComparisonReport(cacheResults, cacheOutputDir)
		if err != nil {
			log.Printf("Warning: Failed to generate cache benchmark report: %v", err)
		} else {
			fmt.Printf("Cache benchmark report generated in %s\n", cacheOutputDir)
		}
	}

	// Parse storage benchmark results
	storageBenchmarkPath := filepath.Join(resultsDir, "storage_benchmark.json")
	storageResults, err := v2.ParseBenchmarkResults(storageBenchmarkPath)
	if err != nil {
		log.Printf("Warning: Failed to parse storage benchmark results: %v", err)
	} else {
		// Generate storage benchmark report
		storageOutputDir := filepath.Join(outputDir, "storage")
		err = v2.GenerateComparisonReport(storageResults, storageOutputDir)
		if err != nil {
			log.Printf("Warning: Failed to generate storage benchmark report: %v", err)
		} else {
			fmt.Printf("Storage benchmark report generated in %s\n", storageOutputDir)
		}
	}

	// Generate summary report
	summaryPath := filepath.Join(outputDir, "summary.md")
	summaryFile, err := os.Create(summaryPath)
	if err != nil {
		log.Printf("Warning: Failed to create summary report: %v", err)
		return
	}
	defer summaryFile.Close()

	// Write summary header
	fmt.Fprintf(summaryFile, "# nodestorage/v2 Benchmark Summary\n\n")
	fmt.Fprintf(summaryFile, "This document summarizes the benchmark results for the nodestorage/v2 package.\n\n")

	// Write cache benchmark summary
	fmt.Fprintf(summaryFile, "## Cache Benchmark Summary\n\n")
	if len(cacheResults) > 0 {
		fmt.Fprintf(summaryFile, "The following cache implementations were benchmarked:\n\n")
		fmt.Fprintf(summaryFile, "- Memory Cache\n")
		fmt.Fprintf(summaryFile, "- BadgerDB Cache\n")
		fmt.Fprintf(summaryFile, "- Redis Cache (if available)\n\n")

		fmt.Fprintf(summaryFile, "For detailed results, see the reports in the `cache` directory.\n\n")

		// Add links to cache reports
		fmt.Fprintf(summaryFile, "### Cache Operation Reports\n\n")
		operations := make(map[string]bool)
		for _, result := range cacheResults {
			operations[result.Operation] = true
		}
		for operation := range operations {
			fmt.Fprintf(summaryFile, "- [%s](cache/%s_report.md)\n", operation, operation)
		}
		fmt.Fprintf(summaryFile, "\n")
	} else {
		fmt.Fprintf(summaryFile, "No cache benchmark results available.\n\n")
	}

	// Write storage benchmark summary
	fmt.Fprintf(summaryFile, "## Storage Benchmark Summary\n\n")
	if len(storageResults) > 0 {
		fmt.Fprintf(summaryFile, "The following storage configurations were benchmarked:\n\n")
		fmt.Fprintf(summaryFile, "- Storage with No Cache\n")
		fmt.Fprintf(summaryFile, "- Storage with Memory Cache\n")
		fmt.Fprintf(summaryFile, "- Storage with BadgerDB Cache\n")
		fmt.Fprintf(summaryFile, "- Storage with Redis Cache (if available)\n\n")

		fmt.Fprintf(summaryFile, "For detailed results, see the reports in the `storage` directory.\n\n")

		// Add links to storage reports
		fmt.Fprintf(summaryFile, "### Storage Operation Reports\n\n")
		operations := make(map[string]bool)
		for _, result := range storageResults {
			operations[result.Operation] = true
		}
		for operation := range operations {
			fmt.Fprintf(summaryFile, "- [%s](storage/%s_report.md)\n", operation, operation)
		}
		fmt.Fprintf(summaryFile, "\n")
	} else {
		fmt.Fprintf(summaryFile, "No storage benchmark results available.\n\n")
	}

	// Write general recommendations
	fmt.Fprintf(summaryFile, "## General Recommendations\n\n")
	fmt.Fprintf(summaryFile, "Based on the benchmark results, the following general recommendations can be made:\n\n")
	fmt.Fprintf(summaryFile, "1. **Memory Cache** provides the fastest access times but is limited by available memory.\n")
	fmt.Fprintf(summaryFile, "   - Best for small to medium-sized datasets with frequent reads.\n")
	fmt.Fprintf(summaryFile, "   - Not suitable for distributed environments or when persistence is required.\n\n")
	fmt.Fprintf(summaryFile, "2. **BadgerDB Cache** offers a good balance between performance and persistence.\n")
	fmt.Fprintf(summaryFile, "   - Best for medium to large datasets that need persistence.\n")
	fmt.Fprintf(summaryFile, "   - Good for single-node applications with moderate read/write ratios.\n\n")
	fmt.Fprintf(summaryFile, "3. **Redis Cache** is suitable for distributed environments where cache sharing is required.\n")
	fmt.Fprintf(summaryFile, "   - Best for distributed applications that need a shared cache.\n")
	fmt.Fprintf(summaryFile, "   - Good for applications with moderate read/write ratios.\n\n")
	fmt.Fprintf(summaryFile, "4. **No Cache** might be appropriate for write-heavy workloads with infrequent reads.\n")
	fmt.Fprintf(summaryFile, "   - Best for write-heavy applications where cache invalidation would be frequent.\n")
	fmt.Fprintf(summaryFile, "   - Good for applications where data consistency is more important than read performance.\n\n")

	fmt.Printf("Summary report generated in %s\n", summaryPath)
}
