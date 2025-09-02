#!/bin/bash

set -e

KEYSTORE_PASS="changeit"
TRUSTSTORE_PASS="changeit"
DNAME="CN=localhost,OU=Test,O=Test,L=Test,S=Test,C=US"
ALIAS="kafka-server"
VALIDITY=365

# Generate keystore
keytool -genkeypair \
  -alias $ALIAS \
  -keyalg RSA \
  -keystore kafka.server.keystore.jks \
  -storepass $KEYSTORE_PASS \
  -keypass $KEYSTORE_PASS \
  -dname "$DNAME" \
  -validity $VALIDITY

# Export certificate
keytool -export \
  -alias $ALIAS \
  -keystore kafka.server.keystore.jks \
  -storepass $KEYSTORE_PASS \
  -file kafka.server.cer

# Create truststore and import certificate
keytool -import \
  -alias $ALIAS \
  -file kafka.server.cer \
  -keystore kafka.server.truststore.jks \
  -storepass $TRUSTSTORE_PASS \
  -noprompt

echo "Keystore and truststore generated:"
echo "  kafka.server.keystore.jks"
echo "  kafka.server.truststore.jks"
