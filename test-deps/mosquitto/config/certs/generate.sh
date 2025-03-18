#!/bin/bash

IP="127.0.0.1"
SUBJECT_CA="/C=SE/PT=Porto/L=Porto/O=oddbit/OU=Ca/CN=$IP"
SUBJECT_SERVER="/C=PT/ST=Porto/L=Porto/O=oddbit/OU=Server/CN=$IP"
SUBJECT_CLIENT="/C=PT/ST=Porto/L=Porto/O=oddbit/OU=Client/CN=$IP"

function generate_CA () {
   echo "$SUBJECT_CA"
   openssl req -x509 -nodes -sha256 -newkey rsa:2048 -subj "$SUBJECT_CA"  -days 3650 -keyout ca.key -out ca.crt
}

function generate_server () {
   echo "$SUBJECT_SERVER"
   openssl req -nodes -sha256 -new -subj "$SUBJECT_SERVER" -keyout server.key -out server.csr
   openssl x509 -req -sha256 -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out server.crt -days 3650
}

function generate_client () {
   echo "$SUBJECT_CLIENT"
   openssl req -new -nodes -sha256 -subj "$SUBJECT_CLIENT" -out client.csr -keyout client.key 
   openssl x509 -req -sha256 -in client.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out client.crt -days 3650
}

generate_CA
generate_server
generate_client
