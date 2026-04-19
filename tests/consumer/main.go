package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/confluentinc/confluent-kafka-go/kafka"
)

func main() {
	var brokers string
	var topic string
	var saslEnabled bool
	var saslMechanism string
	var saslUsername string
	var saslPassword string
	var securityProtocol string

	flag.StringVar(&brokers, "brokers", "localhost:9092", "Comma-separated list of Kafka broker addresses")
	flag.StringVar(&brokers, "b", "localhost:9092", "Comma-separated list of Kafka broker addresses (short)")
	flag.StringVar(&topic, "topic", "my-topic", "Kafka topic to consume from")

	// New SASL-related flags
	flag.BoolVar(&saslEnabled, "sasl", false, "Enable SASL authentication")
	flag.StringVar(&saslMechanism, "sasl-mechanism", "PLAIN", "SASL mechanism (e.g., PLAIN, SCRAM-SHA-256, SCRAM-SHA-512)")
	flag.StringVar(&saslUsername, "sasl-username", "", "SASL username")
	flag.StringVar(&saslPassword, "sasl-password", "", "SASL password")
	flag.StringVar(&securityProtocol, "security-protocol", "SASL_PLAINTEXT", "Security protocol (SASL_PLAINTEXT or SASL_SSL)")

	flag.Parse()

	// Base consumer configuration
	config := &kafka.ConfigMap{
		"bootstrap.servers": brokers,
		"group.id":          "tests-group", // Consumer group ID
		"auto.offset.reset": "earliest",    // Start reading from the beginning if no offset is saved
	}

	// If SASL is enabled, extend config
	if saslEnabled {
		config.SetKey("security.protocol", securityProtocol)
		config.SetKey("sasl.mechanisms", saslMechanism)
		config.SetKey("sasl.username", saslUsername)
		config.SetKey("sasl.password", saslPassword)
	}

	// Create a new consumer instance
	c, err := kafka.NewConsumer(config)
	if err != nil {
		log.Fatalf("Failed to create consumer: %v", err)
	}
	defer func() {
		if err := c.Close(); err != nil {
			log.Printf("Error closing consumer: %v", err)
		}
	}()

	// Subscribe to topic
	err = c.SubscribeTopics([]string{topic}, nil)
	if err != nil {
		log.Fatalf("Failed to subscribe to topic: %v", err)
	}

	// Handle graceful shutdown
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)

	fmt.Printf("Consumer started on topic %s. Press Ctrl+C to exit.\n", topic)

	run := true
	for run {
		select {
		case sig := <-sigchan:
			fmt.Printf("Caught signal %v: terminating\n", sig)
			run = false
		default:
			ev := c.Poll(100)
			if ev == nil {
				continue
			}

			switch e := ev.(type) {
			case *kafka.Message:
				fmt.Printf("Received message from topic %s [%d] at offset %v: %s\n",
					*e.TopicPartition.Topic, e.TopicPartition.Partition, e.TopicPartition.Offset, string(e.Value))
			case kafka.Error:
				log.Printf("Consumer error: %v (%v)\n", e.Code(), e.String())
			}
		}
	}

	fmt.Println("Consumer gracefully shut down.")
}
