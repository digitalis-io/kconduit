#!/bin/bash

# Test ACL operations with Kafka

echo "Creating test ACL..."
docker exec broker-1 /opt/kafka/bin/kafka-acls.sh \
  --bootstrap-server localhost:9092 \
  --command-config /opt/kafka/config/client.properties \
  --add \
  --allow-principal User:alice \
  --operation Read \
  --topic test-topic

echo -e "\nListing ACLs..."
docker exec broker-1 /opt/kafka/bin/kafka-acls.sh \
  --bootstrap-server localhost:9092 \
  --command-config /opt/kafka/config/client.properties \
  --list

echo -e "\nDone!"