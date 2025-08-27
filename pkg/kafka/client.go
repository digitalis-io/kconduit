package kafka

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/IBM/sarama"
)

const topicCacheDuration = 1 * time.Minute

type Client struct {
	brokers           []string
	config            *sarama.Config
	admin             sarama.ClusterAdmin
	producer          sarama.SyncProducer
	topics            []TopicInfo
	topicsLastFetched time.Time
}

func NewClient(brokers []string) (*Client, error) {
	config := sarama.NewConfig()
	config.Version = sarama.V2_8_0_0
	config.Producer.Return.Successes = true
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 5
	config.Admin.Timeout = 10 * time.Second
	config.Metadata.Retry.Max = 3
	config.Metadata.Retry.Backoff = 250 * time.Millisecond
	config.Metadata.Timeout = 10 * time.Second

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
	if c.topicsLastFetched.Add(topicCacheDuration).After(time.Now()) && len(c.topics) > 0 {
		return c.topics, nil
	}

	metadata, err := c.admin.ListTopics()
	if err != nil {
		return nil, fmt.Errorf("failed to list topics: %w", err)
	}

	var topicInfos []TopicInfo
	for name, details := range metadata {
		info := TopicInfo{
			Name:              name,
			Partitions:        int(details.NumPartitions),
			ReplicationFactor: int(details.ReplicationFactor),
		}

		topicInfos = append(topicInfos, info)
	}

	sort.Slice(topicInfos, func(i, j int) bool {
		return topicInfos[i].Name < topicInfos[j].Name
	})

	// Cache topics
	c.topics = topicInfos

	return c.topics, nil
}

func (c *Client) GetTopicConfig(topicName string) (*TopicConfig, error) {
	// Get topic metadata
	metadata, err := c.admin.ListTopics()
	if err != nil {
		return nil, fmt.Errorf("failed to list topics: %w", err)
	}

	topicMeta, exists := metadata[topicName]
	if !exists {
		return nil, fmt.Errorf("topic %s not found", topicName)
	}

	config := &TopicConfig{
		Name:              topicName,
		Partitions:        int(topicMeta.NumPartitions),
		ReplicationFactor: int(topicMeta.ReplicationFactor),
		Configs:           make(map[string]string),
		PartitionDetails:  make([]PartitionInfo, 0),
	}

	// Get topic configuration
	resource := sarama.ConfigResource{
		Type: sarama.TopicResource,
		Name: topicName,
	}
	
	configs, err := c.admin.DescribeConfig(resource)
	if err == nil && configs != nil {
		for _, entry := range configs {
			config.Configs[entry.Name] = entry.Value
		}
	}

	// Get partition details
	controller, err := c.admin.Controller()
	if err == nil {
		defer controller.Close()
		
		request := &sarama.MetadataRequest{
			Topics: []string{topicName},
		}
		
		metadata, err := controller.GetMetadata(request)
		if err == nil && metadata != nil {
			for _, topic := range metadata.Topics {
				if topic.Name == topicName {
					for _, partition := range topic.Partitions {
						partInfo := PartitionInfo{
							ID:       partition.ID,
							Leader:   partition.Leader,
							Replicas: partition.Replicas,
							ISR:      partition.Isr,
						}
						config.PartitionDetails = append(config.PartitionDetails, partInfo)
					}
					break
				}
			}
		}
	}

	return config, nil
}

func (c *Client) GetBrokers() ([]BrokerInfo, error) {
	controller, err := c.admin.Controller()
	if err != nil {
		return nil, fmt.Errorf("failed to get controller: %w", err)
	}
	defer controller.Close()

	// Create an empty metadata request to get all metadata
	request := &sarama.MetadataRequest{}
	metadata, err := controller.GetMetadata(request)
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata: %w", err)
	}

	var brokers []BrokerInfo
	for _, broker := range metadata.Brokers {
		// Parse host and port from address
		host := broker.Addr()
		port := int32(9092) // default Kafka port
		if idx := strings.LastIndex(host, ":"); idx != -1 {
			if p, err := strconv.Atoi(host[idx+1:]); err == nil {
				port = int32(p)
				host = host[:idx]
			}
		}
		
		info := BrokerInfo{
			ID:     broker.ID(),
			Host:   host,
			Port:   port,
			Rack:   broker.Rack(),
			Status: "Online", // Brokers in metadata are online
		}
		
		// Check if this broker is the controller
		if metadata.ControllerID == broker.ID() {
			info.IsController = true
		}

		// Try to get API versions from the broker
		// First ensure broker is connected
		if err := broker.Open(c.config); err == nil {
			connected, _ := broker.Connected()
			if connected {
				apiVersions, err := broker.ApiVersions(&sarama.ApiVersionsRequest{})
				if err == nil && apiVersions != nil && len(apiVersions.ApiKeys) > 0 {
					// Get Kafka version from API versions
					info.ApiVersions = c.getKafkaVersion(apiVersions.ApiKeys)
					info.ListenerCount = len(apiVersions.ApiKeys)
				}
			}
		}
		
		// If we still don't have version, use a default
		if info.ApiVersions == "" {
			info.ApiVersions = "2.8+" // Based on our config version
		}

		// Get log dir count (requires broker connection)
		if descLogDirs, err := c.admin.DescribeLogDirs([]int32{broker.ID()}); err == nil {
			if logDirs, ok := descLogDirs[broker.ID()]; ok {
				info.LogDirCount = len(logDirs)
			}
		}

		brokers = append(brokers, info)
	}

	sort.Slice(brokers, func(i, j int) bool {
		return brokers[i].ID < brokers[j].ID
	})

	return brokers, nil
}

func (c *Client) getKafkaVersion(apiKeys []sarama.ApiVersionsResponseKey) string {
	// Determine Kafka version based on API versions
	if len(apiKeys) == 0 {
		return "Unknown"
	}
	
	maxApiKey := int16(0)
	apiVersionMap := make(map[int16]int16)
	
	for _, api := range apiKeys {
		if api.ApiKey > maxApiKey {
			maxApiKey = api.ApiKey
		}
		apiVersionMap[api.ApiKey] = api.MaxVersion
	}
	
	// More accurate version detection based on specific API keys
	// Check for API keys introduced in different Kafka versions
	
	// Kafka 3.0+ introduced API key 67 (DescribeCluster)
	if _, hasKey67 := apiVersionMap[67]; hasKey67 {
		if maxApiKey >= 68 {
			return "3.5+"
		}
		return "3.0+"
	}
	
	// Kafka 2.8 introduced API key 60 (DescribeQuorum)
	if _, hasKey60 := apiVersionMap[60]; hasKey60 {
		return "2.8+"
	}
	
	// Kafka 2.6 introduced API key 48 (DescribeClientQuotas)  
	if _, hasKey48 := apiVersionMap[48]; hasKey48 {
		return "2.6+"
	}
	
	// Kafka 2.4 introduced API key 43 (ElectLeaders)
	if _, hasKey43 := apiVersionMap[43]; hasKey43 {
		return "2.4+"
	}
	
	// Kafka 2.0 introduced API key 36 (SaslAuthenticate)
	if _, hasKey36 := apiVersionMap[36]; hasKey36 {
		return "2.0+"
	}
	
	// Check by max API key for older versions
	if maxApiKey >= 35 {
		return "2.0+"
	}
	
	if maxApiKey >= 20 {
		return "1.0+"
	}
	
	return "0.11+"
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

type TopicConfig struct {
	Name              string
	Partitions        int
	ReplicationFactor int
	Configs           map[string]string
	PartitionDetails  []PartitionInfo
}

type PartitionInfo struct {
	ID       int32
	Leader   int32
	Replicas []int32
	ISR      []int32
}

type BrokerInfo struct {
	ID            int32
	Host          string
	Port          int32
	Rack          string
	IsController  bool
	ApiVersions   string
	ListenerCount int
	LogDirCount   int
	Status        string // "Online", "Offline", "Unknown"
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
