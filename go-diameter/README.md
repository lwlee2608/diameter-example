# Go Diameter Example

## Generate private key and cert
```bash
./gen_key.sh
```

## Start Diameter Server with TLS
```bash
 go run server/server.go -cert_file="server.crt" -key_file="server.key"  
 ```

 ## Start Diameter Server without TLS
 ```bash
  go run server/server.go 
  ```