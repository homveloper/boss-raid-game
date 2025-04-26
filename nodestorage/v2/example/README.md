# nodestorage/v2 Examples

This directory contains example applications that demonstrate how to use the `nodestorage/v2` package in real-world scenarios.

## Available Examples

### Guild Territory Construction System

The [guild_territory](./guild_territory) example demonstrates a collaborative guild territory construction system for an online game. It shows how to use `nodestorage/v2` to implement:

- Optimistic concurrency control for territory management
- Section-based concurrency control for building updates
- Real-time change notifications using MongoDB change streams
- Caching for improved read performance

This example is particularly useful for understanding how to handle concurrent modifications in a distributed environment, which is common in online games where multiple players interact with shared resources.

## Running the Examples

Each example directory contains its own README with specific instructions on how to run the example. In general, you'll need:

1. MongoDB running locally or accessible via a connection string
2. Go 1.18 or later
3. The required dependencies installed

## Creating New Examples

If you want to create a new example:

1. Create a new directory under `example/`
2. Implement your example using the `nodestorage/v2` package
3. Include a README.md file explaining the example
4. Add tests to demonstrate the functionality

## Contributing

Contributions of new examples or improvements to existing ones are welcome! Please ensure that any new examples:

1. Demonstrate real-world use cases
2. Include comprehensive documentation
3. Have proper tests
4. Follow Go best practices
