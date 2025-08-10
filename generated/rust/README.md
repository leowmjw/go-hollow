# Rust Bindings

Rust bindings will be generated here using capnpc-rust.

## Installation
Add to Cargo.toml:
```toml
[dependencies]
capnp = "0.18"

[build-dependencies]
capnpc = "0.18"
```

## Generation (TODO)
```bash
capnp compile -I../../schemas -orust:. ../../schemas/*.capnp
```
