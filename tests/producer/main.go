package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

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
	flag.StringVar(&topic, "topic", "topic01", "Kafka topic to produce to")

	// SASL-related flags
	flag.BoolVar(&saslEnabled, "sasl", false, "Enable SASL authentication")
	flag.StringVar(&saslMechanism, "sasl-mechanism", "PLAIN", "SASL mechanism (e.g., PLAIN, SCRAM-SHA-256, SCRAM-SHA-512)")
	flag.StringVar(&saslUsername, "sasl-username", "", "SASL username")
	flag.StringVar(&saslPassword, "sasl-password", "", "SASL password")
	flag.StringVar(&securityProtocol, "security-protocol", "SASL_PLAINTEXT", "Security protocol (SASL_PLAINTEXT or SASL_SSL)")

	flag.Parse()

	// Base producer configuration
	config := &kafka.ConfigMap{
		"bootstrap.servers": brokers,
	}

	// Apply SASL settings if requested
	if saslEnabled {
		config.SetKey("security.protocol", securityProtocol)
		config.SetKey("sasl.mechanisms", saslMechanism)
		config.SetKey("sasl.username", saslUsername)
		config.SetKey("sasl.password", saslPassword)
	}

	// Create a new producer instance
	p, err := kafka.NewProducer(config)
	if err != nil {
		log.Fatalf("Failed to create producer: %v", err)
	}
	defer p.Close()

	// Handle delivery reports
	go func() {
		for e := range p.Events() {
			switch ev := e.(type) {
			case *kafka.Message:
				if ev.TopicPartition.Error != nil {
					log.Printf("Delivery failed: %v\n", ev.TopicPartition.Error)
				} else {
					fmt.Printf("Delivered message to %v\n", ev.TopicPartition)
				}
			}
		}
	}()

	// Handle OS signals
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)

	run := true
	for run {
		select {
		case sig := <-sigchan:
			fmt.Printf("Caught signal %v: terminating\n", sig)
			run = false
		default:
			// Produce a message
			value := fmt.Sprintf("Test message at %v", time.Now().Format(time.RFC3339))
			err := p.Produce(&kafka.Message{
				TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
				Value:          []byte(value),
			}, nil)

			if err != nil {
				log.Printf("Failed to produce message: %v", err)
			} else {
				fmt.Printf("Queued message for topic %s: %s\n", topic, value)
			}

			time.Sleep(2 * time.Second)
		}
	}

	// Ensure all queued messages are delivered before shutdown
	fmt.Println("Flushing pending messages...")
	p.Flush(15 * 1000)

	fmt.Println("Producer gracefully shut down.")
}
