# go-hollow TypeScript Examples

This directory contains TypeScript implementations of the three realistic scenarios described in [USAGE.md](../../USAGE.md).

## Prerequisites

- Node.js 18+ or Bun 1.0+
- Generated Cap'n Proto bindings (run `../../tools/gen-schema.sh typescript`)

## Tech Stack

- **Runtime**: Bun (for fast execution and built-in TypeScript support)
- **Dependencies**: @capnp-ts/generator for Cap'n Proto bindings
- **Storage**: AWS SDK v3 for S3 integration
- **Types**: Full TypeScript with strict mode

## Installation

```bash
# Install bun if not already installed
curl -fsSL https://bun.sh/install | bash

# Install dependencies
bun install
```

## Running Examples

### Scenario A: Movie Catalog

```bash
# Generate schemas first
cd ../../
./tools/gen-schema.sh typescript

# Run movie catalog example
cd examples/typescript
bun run movie/main.ts
```

### Scenario B: Commerce Orders

```bash
bun run commerce/main.ts
```

### Scenario C: IoT Metrics

```bash
bun run iot/main.ts
```

## Example Structure

Each scenario demonstrates:

1. **Schema Loading**: Using @capnp-ts generated bindings
2. **Producer Setup**: TypeScript wrapper for go-hollow producer
3. **Consumer Setup**: Promise-based consumer with async/await
4. **Index Queries**: Type-safe index operations with generics
5. **CLI Integration**: TypeScript wrapper for hollow-cli

## Files

- `movie/main.ts` - Movie catalog scenario implementation
- `commerce/main.ts` - E-commerce orders scenario implementation  
- `iot/main.ts` - IoT metrics scenario implementation
- `src/hollow-ts/` - TypeScript wrapper library for go-hollow
- `package.json` - Bun package configuration
- `tsconfig.json` - TypeScript configuration

## Features

- **Type Safety**: Full TypeScript types for all Cap'n Proto schemas
- **Async/Await**: Promise-based APIs for all operations
- **Error Handling**: Comprehensive error handling with Result types
- **Performance**: Leverages Bun's fast JavaScript engine

## TODO (Phase 6)

- Implement @capnp-ts schema generation
- Create TypeScript wrapper for go-hollow APIs
- Add Promise-based announcer implementation
- Implement blob storage with AWS SDK v3
- Add comprehensive type definitions
