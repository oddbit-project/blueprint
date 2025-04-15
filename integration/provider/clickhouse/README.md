# ClickHouse Integration Tests

This directory contains integration tests for the ClickHouse provider. These tests validate the functionality of the ClickHouse client and repository against a real ClickHouse server running in Docker.

## Prerequisites

- Docker and Docker Compose must be installed
- The tests use the ClickHouse server defined in the project's `docker-compose.yml`

## Running the Tests

There are several ways to run the integration tests:

### 1. Full Test (Up, Test, Down)

This will start the Docker containers, run the tests, and then stop the containers:

```bash
make full-test
```

### 2. Manual Steps

Start the Docker containers:

```bash
make up
```

Run the tests:

```bash
make test
```

Stop the Docker containers:

```bash
make down
```

### 3. Run with Race Detection

To run tests with Go's race detector:

```bash
make test-race
```

## Test Details

The integration tests cover:

- **Client Functionality**: Connection, ping, version information
- **Basic Queries**: Simple SELECT operations
- **Repository Operations**: Insert, fetch, count, and delete operations
- **Complex Data Types**: Arrays, maps, and various scalar types
- **Repository SQL Builders**: Verify SQL generation for various operations

## Notes

- Tests are tagged with `integration` to prevent them from running during regular unit tests
- The tests create temporary tables for testing and clean them up afterward
- If tests fail unexpectedly, you may need to manually drop the test tables