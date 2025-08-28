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

	flag.StringVar(&brokers, "brokers", "localhost:9092", "Comma-separated list of Kafka broker addresses")
	flag.StringVar(&brokers, "b", "localhost:9092", "Comma-separated list of Kafka broker addresses (short)")
	flag.StringVar(&topic, "topic", "my-topic", "Kafka topic to consume from")
	flag.Parse()

	// Consumer configuration
	config := &kafka.ConfigMap{
		"bootstrap.servers": brokers,
		"group.id":          "tests-group", // Consumer group ID
		"auto.offset.reset": "earliest",    // Start reading from the beginning if no offset is saved
	}

	// Create a new consumer instance
	c, err := kafka.NewConsumer(config)
	if err != nil {
		log.Fatalf("Failed to create consumer: %v", err)
	}
	defer c.Close()

	// Subscribe to the topic
	err = c.SubscribeTopics([]string{topic}, nil)
	if err != nil {
		log.Fatalf("Failed to subscribe to topic: %v", err)
	}

	// Set up a channel to handle OS signals for graceful shutdown
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("Consumer started. Press Ctrl+C to exit.")

	// Main loop to poll for messages
	run := true
	for run {
		select {
		case sig := <-sigchan:
			fmt.Printf("Caught signal %v: terminating\n", sig)
			run = false
		default:
			ev := c.Poll(100) // Poll for events with a timeout of 100ms
			if ev == nil {
				continue
			}

			switch e := ev.(type) {
			case *kafka.Message:
				fmt.Printf("Received message from topic %s [%d] at offset %v: %s\n",
					*e.TopicPartition.Topic, e.TopicPartition.Partition, e.TopicPartition.Offset, string(e.Value))
			case kafka.Error:
				// Handle consumer errors
				log.Printf("Consumer error: %v (%v)\n", e.Code(), e.String())
			}
		}
	}

	fmt.Println("Consumer gracefully shut down.")
}
