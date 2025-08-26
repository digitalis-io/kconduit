package kafka

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/IBM/sarama"
)

type Client struct {
	brokers  []string
	config   *sarama.Config
	admin    sarama.ClusterAdmin
	producer sarama.SyncProducer
}

func NewClient(brokers []string) (*Client, error) {
	config := sarama.NewConfig()
	config.Version = sarama.V2_8_0_0
	config.Producer.Return.Successes = true
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 5

	admin, err := sarama.NewClusterAdmin(brokers, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create cluster admin: %w", err)
	}

	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		admin.Close()
		return nil, fmt.Errorf("failed to create producer: %w", err)
	}

	return &Client{
		brokers:  brokers,
		config:   config,
		admin:    admin,
		producer: producer,
	}, nil
}

func (c *Client) ListTopics() ([]string, error) {
	metadata, err := c.admin.ListTopics()
	if err != nil {
		return nil, fmt.Errorf("failed to list topics: %w", err)
	}

	topics := make([]string, 0, len(metadata))
	for topic := range metadata {
		topics = append(topics, topic)
	}

	sort.Strings(topics)
	return topics, nil
}

func (c *Client) GetTopicDetails() ([]TopicInfo, error) {
	metadata, err := c.admin.ListTopics()
	if err != nil {
		return nil, fmt.Errorf("failed to list topics: %w", err)
	}

	var topicInfos []TopicInfo
	for name, details := range metadata {
		info := TopicInfo{
			Name:       name,
			Partitions: int(details.NumPartitions),
		}

		info.ReplicationFactor = int(details.ReplicationFactor)

		topicInfos = append(topicInfos, info)
	}

	sort.Slice(topicInfos, func(i, j int) bool {
		return topicInfos[i].Name < topicInfos[j].Name
	})

	return topicInfos, nil
}

func (c *Client) CreateTopic(name string, numPartitions int32, replicationFactor int16) error {
	if name == "" {
		return fmt.Errorf("topic name cannot be empty")
	}
	
	if numPartitions < 1 {
		numPartitions = 1
	}
	
	if replicationFactor < 1 {
		replicationFactor = 1
	}

	topicDetail := &sarama.TopicDetail{
		NumPartitions:     numPartitions,
		ReplicationFactor: replicationFactor,
	}

	err := c.admin.CreateTopic(name, topicDetail, false)
	if err != nil {
		return fmt.Errorf("failed to create topic: %w", err)
	}

	return nil
}

func (c *Client) ProduceMessage(topic, key, value string) error {
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.StringEncoder(value),
	}
	
	if key != "" {
		msg.Key = sarama.StringEncoder(key)
	}

	partition, offset, err := c.producer.SendMessage(msg)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	_ = partition
	_ = offset
	return nil
}

func (c *Client) ConsumeMessages(ctx context.Context, topic string, messageChan chan<- Message) error {
	consumer, err := sarama.NewConsumer(c.brokers, c.config)
	if err != nil {
		return fmt.Errorf("failed to create consumer: %w", err)
	}

	partitions, err := consumer.Partitions(topic)
	if err != nil {
		consumer.Close()
		return fmt.Errorf("failed to get partitions: %w", err)
	}

	var partitionConsumers []sarama.PartitionConsumer

	for _, partition := range partitions {
		pc, err := consumer.ConsumePartition(topic, partition, sarama.OffsetOldest)
		if err != nil {
			// Close all previously opened partition consumers
			for _, pcons := range partitionConsumers {
				pcons.Close()
			}
			consumer.Close()
			return fmt.Errorf("failed to consume partition %d: %w", partition, err)
		}
		partitionConsumers = append(partitionConsumers, pc)

		go func(pc sarama.PartitionConsumer, partition int32) {
			for {
				select {
				case <-ctx.Done():
					return
				case msg, ok := <-pc.Messages():
					if !ok || msg == nil {
						return
					}
					
					headers := make(map[string]string)
					for _, h := range msg.Headers {
						headers[string(h.Key)] = string(h.Value)
					}
					
					message := Message{
						Topic:     msg.Topic,
						Partition: msg.Partition,
						Offset:    msg.Offset,
						Key:       string(msg.Key),
						Value:     string(msg.Value),
						Timestamp: msg.Timestamp,
						Headers:   headers,
					}
					
					select {
					case messageChan <- message:
					case <-ctx.Done():
						return
					}
				case err := <-pc.Errors():
					if err != nil {
						// Log error but continue consuming
						select {
						case messageChan <- Message{Topic: topic, Value: fmt.Sprintf("Error: %v", err)}:
						case <-ctx.Done():
							return
						}
					}
				}
			}
		}(pc, partition)
	}

	// Wait for context cancellation
	<-ctx.Done()

	// Clean up all partition consumers
	for _, pc := range partitionConsumers {
		pc.Close()
	}
	consumer.Close()

	return nil
}

func (c *Client) Close() error {
	var errs []error
	
	if c.producer != nil {
		if err := c.producer.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close producer: %w", err))
		}
	}
	
	if c.admin != nil {
		if err := c.admin.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close admin: %w", err))
		}
	}
	
	if len(errs) > 0 {
		return fmt.Errorf("errors closing client: %v", errs)
	}
	return nil
}

type TopicInfo struct {
	Name              string
	Partitions        int
	ReplicationFactor int
}

type Message struct {
	Topic     string
	Partition int32
	Offset    int64
	Key       string
	Value     string
	Timestamp time.Time
	Headers   map[string]string
}