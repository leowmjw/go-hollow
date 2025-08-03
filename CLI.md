# Go-Hollow CLI Reference

The Go-Hollow CLI provides a set of commands for interacting with the Go-Hollow system, including schema validation, data production, consumption, inspection, and comparison.

## Command Structure

```
hollow-cli [command] [options]
```

## Available Commands

### `help`

Display help information about available commands and options.

```bash
hollow-cli help
```

### `schema`

Validate schema files in Cap'n Proto or legacy DSL format.

```bash
hollow-cli schema -data=<schema-file-path> [-verbose]
```

Options:
- `-data`: Path to the schema file (required)
- `-verbose`: Display detailed schema information

Example:
```bash
hollow-cli schema -data=fixtures/schema/test_schema.capnp -verbose
```

### `produce`

Run a producer cycle with test data.

```bash
hollow-cli produce -data=<data-file-path> [-store=<store-type>] [-verbose]
```

Options:
- `-data`: Path to the data file (required)
- `-store`: Blob store type (memory, s3) (default: memory)
- `-verbose`: Display detailed output

Example:
```bash
hollow-cli produce -data=data.json -store=memory
```

### `consume`

Run a consumer to read data from a specific version.

```bash
hollow-cli consume -version=<version> [-store=<store-type>] [-verbose]
```

Options:
- `-version`: Version number to consume (required)
- `-store`: Blob store type (memory, s3) (default: memory)
- `-verbose`: Display detailed output

Example:
```bash
hollow-cli consume -version=1 -verbose
```

### `inspect`

Inspect a specific version of the data.

```bash
hollow-cli inspect -version=<version> [-store=<store-type>] [-verbose]
```

Options:
- `-version`: Version number to inspect (required)
- `-store`: Blob store type (memory, s3) (default: memory)
- `-verbose`: Display detailed output

Example:
```bash
hollow-cli inspect -store=memory -version=1
```

### `diff`

Compare two versions of the data.

```bash
hollow-cli diff -version=<from-version> -target=<to-version> [-store=<store-type>] [-verbose]
```

Options:
- `-version`: Source version number (required)
- `-target`: Target version number (required)
- `-store`: Blob store type (memory, s3) (default: memory)
- `-verbose`: Display detailed output

Example:
```bash
hollow-cli diff -store=memory -version=1 -target=2
```

## Schema Validation Examples

### Valid Cap'n Proto Schema

```bash
hollow-cli schema -data=fixtures/schema/test_schema.capnp -verbose
```

Example output:
```
Validating schema file: fixtures/schema/test_schema.capnp
Schema Person validated successfully
Schema Address validated successfully
Schema Role validated successfully
Schema validation passed for 3 schemas

Schema: Person (Type: 0)
  Fields: 7
    id (reference to @0)
    name (reference to @1)
    email (reference to @2)
    ...
```

### Invalid Schema

```bash
hollow-cli schema -data=fixtures/schema/invalid_schema.txt -verbose
```

Example output:
```
Validating schema file: fixtures/schema/invalid_schema.txt
Error parsing schema file: invalid Cap'n Proto schema: missing @0x version identifier
```

## Advanced Usage

### Using S3 Storage

```bash
hollow-cli produce -data=data.json -store=s3 -endpoint=localhost:9000 -bucket=hollow-test
```

### Schema Evolution

When evolving schemas, use the diff command to see changes between versions:

```bash
# Produce data with new schema
hollow-cli produce -data=new_data.json -store=memory

# Compare with previous version
hollow-cli diff -store=memory -version=2 -target=1
```

## Troubleshooting

### Common Errors

1. **Schema validation errors**:
   - Check that your schema file follows the correct format (Cap'n Proto or legacy DSL)
   - Ensure all referenced types are defined
   - Verify field IDs are unique within each struct

2. **Storage errors**:
   - For S3 storage, check that the endpoint and bucket are correct
   - Verify credentials are properly configured

3. **Version not found**:
   - Ensure the requested version exists in the blob store
   - Use `inspect` command with `-store` option to list available versions
