#!/bin/bash
#
# Test ACL operations against the cluster in docker-compose-acls.yaml.
#
# The bootstrap address matters here. Each broker advertises its BROKER_HOST
# listener as localhost:<published port> — 29092, 39092, 49092 — which is the
# address a client on *your machine* uses. Run a client inside a container and
# it bootstraps fine, then reads that same metadata back and tries to reach the
# brokers on ports nothing in the container is listening on:
#
#   WARN Connection to node 4 (localhost/127.0.0.1:29092) could not be
#   established. Node may not be available.
#
# BROKER_HOST_SSL is advertised as broker-N:9094 and mapped to SASL_PLAINTEXT,
# so it resolves from inside the Docker network and still authenticates. It is
# the listener kafka-ui is pointed at for the same reason.

set -euo pipefail

BOOTSTRAP="${BOOTSTRAP:-broker-1:9094}"
COMMAND_CONFIG=/opt/kafka/config/client.properties
TOPIC="${TOPIC:-test-topic}"
PRINCIPAL="${PRINCIPAL:-User:alice}"

acls() {
  docker exec broker-1 /opt/kafka/bin/kafka-acls.sh \
    --bootstrap-server "$BOOTSTRAP" \
    --command-config "$COMMAND_CONFIG" \
    "$@"
}

echo "Creating test ACL..."
acls --add \
  --allow-principal "$PRINCIPAL" \
  --operation Read \
  --topic "$TOPIC"

echo -e "\nListing ACLs..."
acls --list

echo -e "\nDone!"
