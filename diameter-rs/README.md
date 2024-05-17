# Rust Diameter Example

Make sure you are in the correct directory
```bash
cd diameter-rs
```

Generate private key and cert
```bash
./gen_key.sh
```

## Start TLS Diameter Server
```bash
cargo run --bin server
 ```

## Start TLS Diameter Client
```bash
cargo run --bin client
 ```