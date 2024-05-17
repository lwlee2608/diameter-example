openssl genpkey -out server.key -algorithm RSA -pkeyopt rsa_keygen_bits:2048

openssl req -nodes -key server.key -x509 -sha256 -days 3650 -out server.crt \
 -subj "/C=US/ST=CA/L=SJ/O=Matrixx/OU=Eng/CN=example.com/emailAddress=user@host" \
 -addext "subjectAltName=DNS:example.com,DNS:www.example.net,IP:10.0.0.1"
