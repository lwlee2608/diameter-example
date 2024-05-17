# Go Diameter Example

Make sure you are in the correct directory
```bash
cd go-diameter
```

Generate private key and cert
```bash
./gen_key.sh
```

## Start Diameter Server with TLS
```bash
 go run server/server.go -network="sctp" -cert_file="server.crt" -key_file="server.key"  
 ```

 ## Start Diameter Server without TLS
 ```bash
  go run server/server.go -network="sctp"
  ```

## Start Diameter Client with TLS
```bash
 go run client/client.go -ssl
 ```