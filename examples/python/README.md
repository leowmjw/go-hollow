# go-hollow Python Examples

This directory contains Python implementations of the three realistic scenarios described in [USAGE.md](../../USAGE.md).

## Prerequisites

- Python 3.9 or later
- `uv` or `uvx` for dependency management
- Generated Cap'n Proto bindings (run `../../tools/gen-schema.sh python`)

## Tech Stack

- **Python**: Standalone using uv/uvx (no pip/conda)
- **Dependencies**: pycapnp, asyncio for async operations
- **Storage**: aiobotocore for S3 integration
- **Logging**: Standard library logging

## Installation

```bash
# Install uv if not already installed
curl -LsSf https://astral.sh/uv/install.sh | sh

# Install dependencies
uv sync
```

## Running Examples

### Scenario A: Movie Catalog

```bash
# Generate schemas first
cd ../../
./tools/gen-schema.sh python

# Run movie catalog example
cd examples/python
uvx run movie/main.py
```

### Scenario B: Commerce Orders

```bash
uvx run commerce/main.py
```

### Scenario C: IoT Metrics

```bash
uvx run iot/main.py
```

## Example Structure

Each scenario demonstrates:

1. **Schema Loading**: Using pycapnp with generated bindings
2. **Producer Setup**: Creating versioned snapshots
3. **Consumer Setup**: Reading data with indexes and filtering
4. **Async Operations**: Using asyncio for announcer and blob storage
5. **CLI Integration**: Python wrapper for hollow-cli commands

## Files

- `movie/main.py` - Movie catalog scenario implementation
- `commerce/main.py` - E-commerce orders scenario implementation  
- `iot/main.py` - IoT metrics scenario implementation
- `hollow_py/` - Python wrapper library for go-hollow
- `pyproject.toml` - uv project configuration
- `requirements.txt` - Python dependencies

## TODO (Phase 6)

- Implement pycapnp schema generation
- Create Python wrapper for go-hollow APIs
- Add async announcer implementation
- Implement blob storage with aiobotocore
