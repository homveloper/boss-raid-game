@echo off
echo Running nodestorage/v2 benchmarks...

REM Create benchmark results directory
mkdir benchmark_results 2>nul

REM Run cache benchmarks
echo.
echo Running cache benchmarks...
go test -bench=BenchmarkCacheComparison -benchmem -benchtime=1s ./cache -json > benchmark_results\cache_benchmark.json

REM Run storage benchmarks
echo.
echo Running storage benchmarks...
go test -bench=BenchmarkStorageCacheComparison -benchmem -benchtime=1s . -json > benchmark_results\storage_benchmark.json

REM Generate reports
echo.
echo Generating benchmark reports...
go run cmd/benchmark_analysis/main.go

echo.
echo All benchmarks completed. Results are in the benchmark_results directory.
