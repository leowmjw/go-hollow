#!/bin/bash

# Generate Cap'n Proto bindings for all supported languages
# This script ensures consistent schema compilation across languages

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
SCHEMAS_DIR="$PROJECT_ROOT/schemas"
GENERATED_DIR="$PROJECT_ROOT/generated"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if capnp is installed
check_capnp() {
    if ! command -v capnp &> /dev/null; then
        log_error "Cap'n Proto compiler (capnp) not found"
        log_error "Install with: brew install capnp (macOS) or apt-get install capnproto (Ubuntu)"
        exit 1
    fi
    
    log_info "Found Cap'n Proto compiler: $(capnp --version)"
}

# Generate Go bindings
generate_go() {
    log_info "Generating Go bindings..."
    
    local go_dir="$GENERATED_DIR/go"
    mkdir -p "$go_dir"
    
    # Generate bindings for each schema file individually
    local schemas=(
        "common:common"
        "movie_dataset:movie"
        "commerce_dataset:commerce"
        "iot_dataset:iot"
    )
    
    for schema_info in "${schemas[@]}"; do
        local schema_file="${schema_info%:*}"
        local package_name="${schema_info#*:}"
        local package_dir="$go_dir/$package_name"
        
        log_info "  Processing $schema_file.capnp -> package $package_name..."
        
        mkdir -p "$package_dir"
        capnp compile -I"$SCHEMAS_DIR" -ogo:"$package_dir" "$SCHEMAS_DIR/$schema_file.capnp"
        
        # Create individual go.mod for each package
        cat > "$package_dir/go.mod" << EOF
module github.com/leowmjw/go-hollow/generated/go/$package_name

go 1.24.5

require (
    capnproto.org/go/capnp/v3 v3.1.0-alpha.1
)
EOF
    done
    
    # Create go.mod for generated code
    cat > "$go_dir/go.mod" << EOF
module github.com/leowmjw/go-hollow/generated/go

go 1.24.5

require (
    capnproto.org/go/capnp/v3 v3.1.0-alpha.1
)
EOF
    
    log_info "Go bindings generated in $go_dir"
}

# Generate Python bindings (placeholder for future implementation)
generate_python() {
    log_warn "Python bindings generation not yet implemented"
    log_warn "Will be implemented in Phase 6 with pycapnp"
    
    local py_dir="$GENERATED_DIR/python"
    mkdir -p "$py_dir"
    
    # Create placeholder
    cat > "$py_dir/README.md" << EOF
# Python Bindings

Python bindings will be generated here using pycapnp.

## Installation
\`\`\`bash
pip install pycapnp
\`\`\`

## Generation (TODO)
\`\`\`bash
capnp compile -I../../schemas -opy:. ../../schemas/*.capnp
\`\`\`
EOF
}

# Generate TypeScript bindings (placeholder for future implementation)
generate_typescript() {
    log_warn "TypeScript bindings generation not yet implemented"
    log_warn "Will be implemented in Phase 6 with @capnp-ts"
    
    local ts_dir="$GENERATED_DIR/typescript"
    mkdir -p "$ts_dir"
    
    # Create placeholder
    cat > "$ts_dir/README.md" << EOF
# TypeScript Bindings

TypeScript bindings will be generated here using @capnp-ts.

## Installation
\`\`\`bash
bun add @capnp-ts/generator
\`\`\`

## Generation (TODO)
\`\`\`bash
capnp compile -I../../schemas -ots:. ../../schemas/*.capnp
\`\`\`
EOF
}

# Generate Java bindings (placeholder for future implementation)
generate_java() {
    log_warn "Java bindings generation not yet implemented"
    log_warn "Will be implemented in Phase 6 with capnproto-java"
    
    local java_dir="$GENERATED_DIR/java"
    mkdir -p "$java_dir"
    
    # Create placeholder
    cat > "$java_dir/README.md" << EOF
# Java Bindings

Java bindings will be generated here using capnproto-java.

## Installation
Add to pom.xml:
\`\`\`xml
<dependency>
    <groupId>org.capnproto</groupId>
    <artifactId>runtime</artifactId>
    <version>0.1.14</version>
</dependency>
\`\`\`

## Generation (TODO)
\`\`\`bash
capnp compile -I../../schemas -ojava:. ../../schemas/*.capnp
\`\`\`
EOF
}

# Generate Rust bindings (placeholder for future implementation)
generate_rust() {
    log_warn "Rust bindings generation not yet implemented"
    log_warn "Will be implemented in Phase 6 with capnpc-rust"
    
    local rust_dir="$GENERATED_DIR/rust"
    mkdir -p "$rust_dir"
    
    # Create placeholder
    cat > "$rust_dir/README.md" << EOF
# Rust Bindings

Rust bindings will be generated here using capnpc-rust.

## Installation
Add to Cargo.toml:
\`\`\`toml
[dependencies]
capnp = "0.18"

[build-dependencies]
capnpc = "0.18"
\`\`\`

## Generation (TODO)
\`\`\`bash
capnp compile -I../../schemas -orust:. ../../schemas/*.capnp
\`\`\`
EOF
}

# Clean generated files
clean() {
    log_info "Cleaning generated files..."
    rm -rf "$GENERATED_DIR"
    log_info "Generated files cleaned"
}

# Main function
main() {
    cd "$PROJECT_ROOT"
    
    case "${1:-all}" in
        "clean")
            clean
            ;;
        "go")
            check_capnp
            generate_go
            ;;
        "python")
            generate_python
            ;;
        "typescript"|"ts")
            generate_typescript
            ;;
        "java")
            generate_java
            ;;
        "rust")
            generate_rust
            ;;
        "all")
            check_capnp
            clean
            generate_go
            generate_python
            generate_typescript
            generate_java
            generate_rust
            log_info "Schema generation complete!"
            ;;
        "help"|"-h"|"--help")
            echo "Usage: $0 [LANGUAGE|all|clean|help]"
            echo ""
            echo "Languages:"
            echo "  go          Generate Go bindings only"
            echo "  python      Generate Python bindings only"
            echo "  typescript  Generate TypeScript bindings only"
            echo "  java        Generate Java bindings only"
            echo "  rust        Generate Rust bindings only"
            echo "  all         Generate all language bindings (default)"
            echo ""
            echo "Commands:"
            echo "  clean       Remove all generated files"
            echo "  help        Show this help message"
            ;;
        *)
            log_error "Unknown language or command: $1"
            log_error "Use '$0 help' to see available options"
            exit 1
            ;;
    esac
}

main "$@"
