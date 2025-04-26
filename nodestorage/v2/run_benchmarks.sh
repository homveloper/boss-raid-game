#!/bin/bash
echo "Running nodestorage/v2 benchmarks..."

# Create benchmark results directory
mkdir -p benchmark_results

# Run cache benchmarks
echo ""
echo "Running cache benchmarks..."
go test -bench=BenchmarkCacheComparison -benchmem -benchtime=1s ./cache -json > benchmark_results/cache_benchmark.json

# Run storage benchmarks
echo ""
echo "Running storage benchmarks..."
go test -bench=BenchmarkStorageCacheComparison -benchmem -benchtime=1s . -json > benchmark_results/storage_benchmark.json

# Generate reports
echo ""
echo "Generating benchmark reports..."
go run cmd/benchmark_analysis/main.go

echo ""
echo "All benchmarks completed. Results are in the benchmark_results directory."
