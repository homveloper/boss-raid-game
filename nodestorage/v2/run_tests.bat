@echo off
echo Running nodestorage/v2 tests...

echo.
echo Running cache tests...
go test -v ./cache

echo.
echo Running storage tests...
go test -v .

echo.
echo All tests completed.
