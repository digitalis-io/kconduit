package kafka

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/IBM/sarama"
	"github.com/axonops/kconduit/pkg/logger"
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

// SASLConfig holds SASL authentication configuration
type SASLConfig struct {
	Enabled    bool
	Mechanism  string // PLAIN, SCRAM-SHA-256, SCRAM-SHA-512
	Username   string
	Password   string
	Protocol   string // SASL_PLAINTEXT or SASL_SSL
}

func NewClient(brokers []string) (*Client, error) {
	return NewClientWithAuth(brokers, nil)
}

// NewClientWithAuth creates a new Kafka client with optional SASL authentication
func NewClientWithAuth(brokers []string, saslConfig *SASLConfig) (*Client, error) {
	log := logger.Get()
	log.WithField("brokers", brokers).Debug("Creating new Kafka client")
	
	config := sarama.NewConfig()
	config.Version = sarama.V2_8_0_0
	config.Producer.Return.Successes = true
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 5
	config.Admin.Timeout = 10 * time.Second
	config.Metadata.Retry.Max = 3
	config.Metadata.Retry.Backoff = 250 * time.Millisecond
	config.Metadata.Timeout = 10 * time.Second
	
	// Configure SASL if provided
	if saslConfig != nil && saslConfig.Enabled {
		log.WithFields(map[string]interface{}{
			"mechanism": saslConfig.Mechanism,
			"username":  saslConfig.Username,
			"protocol":  saslConfig.Protocol,
		}).Info("Configuring SASL authentication")
		
		config.Net.SASL.Enable = true
		config.Net.SASL.User = saslConfig.Username
		config.Net.SASL.Password = saslConfig.Password
		
		// Set SASL mechanism
		switch strings.ToUpper(saslConfig.Mechanism) {
		case "PLAIN":
			config.Net.SASL.Mechanism = sarama.SASLTypePlaintext
		case "SCRAM-SHA-256":
			config.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA256
		case "SCRAM-SHA-512":
			config.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA512
		default:
			return nil, fmt.Errorf("unsupported SASL mechanism: %s", saslConfig.Mechanism)
		}
		
		// Set security protocol
		if strings.ToUpper(saslConfig.Protocol) == "SASL_SSL" {
			config.Net.TLS.Enable = true
		}
	}

	admin, err := sarama.NewClusterAdmin(brokers, config)
	if err != nil {
		log.WithError(err).WithField("brokers", brokers).Error("Failed to create cluster admin")
		return nil, fmt.Errorf("failed to create cluster admin: %w", err)
	}

	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		if closeErr := admin.Close(); closeErr != nil {
			log.WithError(closeErr).Warn("Failed to close admin client after producer creation failure")
		}
		log.WithError(err).WithField("brokers", brokers).Error("Failed to create producer")
		return nil, fmt.Errorf("failed to create producer: %w", err)
	}

	log.WithField("brokers", brokers).Info("Successfully connected to Kafka cluster")
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
		defer func() {
			if closeErr := controller.Close(); closeErr != nil {
				logger.Get().WithError(closeErr).Warn("Failed to close controller")
			}
		}()

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
	log := logger.Get()
	
	// Get the controller broker
	controller, err := c.admin.Controller()
	if err != nil {
		return nil, fmt.Errorf("failed to get controller: %w", err)
	}
	defer func() {
		if err := controller.Close(); err != nil {
			log.WithError(err).Warn("Failed to close controller connection")
		}
	}()
	
	// Store the controller's ID - this is the actual active controller
	controllerBrokerID := controller.ID()
	log.WithField("controllerBrokerID", controllerBrokerID).Info("Active controller broker ID")

	// Create an empty metadata request to get all metadata
	request := &sarama.MetadataRequest{}
	metadata, err := controller.GetMetadata(request)
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata: %w", err)
	}
	
	log.WithFields(map[string]interface{}{
		"metadata.ControllerID": metadata.ControllerID,
		"controller.ID()": controllerBrokerID,
		"brokerCount": len(metadata.Brokers),
	}).Info("Metadata retrieved from cluster")

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
		// In KRaft mode, use the controller.ID() we got directly
		// In ZooKeeper mode, use metadata.ControllerID
		if broker.ID() == controllerBrokerID || (metadata.ControllerID >= 0 && metadata.ControllerID == broker.ID()) {
			info.IsController = true
			log.WithFields(map[string]interface{}{
				"brokerID": broker.ID(),
				"controllerBrokerID": controllerBrokerID,
				"metadata.ControllerID": metadata.ControllerID,
			}).Info("Found active controller broker")
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

func (c *Client) DeleteTopic(name string) error {
	log := logger.Get()
	
	if name == "" {
		return fmt.Errorf("topic name cannot be empty")
	}
	
	log.WithField("topic", name).Info("Deleting topic")
	
	// Delete the topic
	err := c.admin.DeleteTopic(name)
	if err != nil {
		log.WithField("topic", name).WithError(err).Error("Failed to delete topic")
		return fmt.Errorf("failed to delete topic: %w", err)
	}
	
	log.WithField("topic", name).Info("Successfully deleted topic")
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
		if closeErr := consumer.Close(); closeErr != nil {
			logger.Get().WithError(closeErr).Warn("Failed to close consumer after partition error")
		}
		return fmt.Errorf("failed to get partitions: %w", err)
	}

	var partitionConsumers []sarama.PartitionConsumer

	for _, partition := range partitions {
		pc, err := consumer.ConsumePartition(topic, partition, sarama.OffsetOldest)
		if err != nil {
			// Close all previously opened partition consumers
			for _, pcons := range partitionConsumers {
				if closeErr := pcons.Close(); closeErr != nil {
					logger.Get().WithError(closeErr).Warn("Failed to close partition consumer during cleanup")
				}
			}
			if closeErr := consumer.Close(); closeErr != nil {
				logger.Get().WithError(closeErr).Warn("Failed to close consumer after consume partition error")
			}
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
		if closeErr := pc.Close(); closeErr != nil {
			logger.Get().WithError(closeErr).Warn("Failed to close partition consumer during cleanup")
		}
	}
	if closeErr := consumer.Close(); closeErr != nil {
		logger.Get().WithError(closeErr).Warn("Failed to close consumer during cleanup")
	}

	return nil
}

func (c *Client) UpdateTopicConfig(topicName string, configKey string, configValue string) error {
	log := logger.Get()
	
	if topicName == "" || configKey == "" {
		err := fmt.Errorf("topic name and config key cannot be empty")
		log.WithError(err).Error("Invalid parameters for UpdateTopicConfig")
		return err
	}

	log.WithFields(map[string]interface{}{
		"topic": topicName,
		"key":   configKey,
		"value": configValue,
	}).Debug("Updating topic configuration")

	// Create the configuration entries map
	configEntries := map[string]*string{
		configKey: &configValue,
	}

	// Apply the configuration change
	err := c.admin.AlterConfig(sarama.TopicResource, topicName, configEntries, false)
	if err != nil {
		log.WithFields(map[string]interface{}{
			"topic": topicName,
			"key":   configKey,
			"value": configValue,
			"error": err,
		}).Error("Failed to update topic configuration")
		return fmt.Errorf("failed to update topic config: %w", err)
	}

	log.WithFields(map[string]interface{}{
		"topic": topicName,
		"key":   configKey,
		"value": configValue,
	}).Info("Successfully updated topic configuration")

	return nil
}

func (c *Client) ModifyTopicPartitions(topicName string, numPartitions int32) error {
	log := logger.Get()
	
	if topicName == "" {
		err := fmt.Errorf("topic name cannot be empty")
		log.WithError(err).Error("Invalid parameters for ModifyTopicPartitions")
		return err
	}

	if numPartitions < 1 {
		err := fmt.Errorf("number of partitions must be at least 1")
		log.WithError(err).Error("Invalid partition count")
		return err
	}

	log.WithFields(map[string]interface{}{
		"topic":      topicName,
		"partitions": numPartitions,
	}).Debug("Modifying topic partitions")

	// Get current topic metadata to check current partition count
	metadata, err := c.admin.ListTopics()
	if err != nil {
		return fmt.Errorf("failed to list topics: %w", err)
	}

	topicMeta, exists := metadata[topicName]
	if !exists {
		return fmt.Errorf("topic %s not found", topicName)
	}

	currentPartitions := topicMeta.NumPartitions
	if numPartitions <= currentPartitions {
		return fmt.Errorf("new partition count (%d) must be greater than current count (%d)", 
			numPartitions, currentPartitions)
	}

	// Create partition update
	err = c.admin.CreatePartitions(topicName, numPartitions, nil, false)
	if err != nil {
		log.WithFields(map[string]interface{}{
			"topic":      topicName,
			"partitions": numPartitions,
			"error":      err,
		}).Error("Failed to modify topic partitions")
		return fmt.Errorf("failed to modify partitions: %w", err)
	}

	log.WithFields(map[string]interface{}{
		"topic":         topicName,
		"oldPartitions": currentPartitions,
		"newPartitions": numPartitions,
	}).Info("Successfully modified topic partitions")

	return nil
}

func (c *Client) GetConsumerGroups() ([]ConsumerGroupInfo, error) {
	log := logger.Get()
	
	// List all consumer groups
	groups, err := c.admin.ListConsumerGroups()
	if err != nil {
		log.WithError(err).Error("Failed to list consumer groups")
		return nil, fmt.Errorf("failed to list consumer groups: %w", err)
	}
	
	var groupInfos []ConsumerGroupInfo
	
	for groupID := range groups {
		// Get group description for detailed info
		descriptions, err := c.admin.DescribeConsumerGroups([]string{groupID})
		if err != nil {
			log.WithField("groupID", groupID).WithError(err).Warn("Failed to describe consumer group")
			continue
		}
		
		if len(descriptions) == 0 {
			continue
		}
		
		desc := descriptions[0]
		
		// Build consumer group info
		info := ConsumerGroupInfo{
			GroupID:    groupID,
			State:      desc.State,
			NumMembers: len(desc.Members),
		}
		
		// Get coordinator info - coordinator is typically embedded in the description
		info.Coordinator = "unknown"
		
		// Collect unique topics from member metadata
		topicSet := make(map[string]struct{})
		for _, member := range desc.Members {
			// Parse member metadata to get topics
			// Note: MemberMetadata contains the subscription info
			if len(member.MemberMetadata) > 0 {
				// TODO: Parse member metadata to extract subscription details
				// The metadata contains encoded consumer protocol information
				logger.Get().WithField("member", member.MemberId).Debug("Member has metadata to be parsed")
			}
		}
		
		// For now, get topics another way - through ListConsumerGroupOffsets
		offsets, err := c.admin.ListConsumerGroupOffsets(groupID, nil)
		if err == nil && offsets != nil {
			for topic := range offsets.Blocks {
				topicSet[topic] = struct{}{}
				info.Topics = append(info.Topics, topic)
			}
		}
		info.NumTopics = len(topicSet)
		
		// Calculate consumer lag (simplified - would need offset fetch for accurate lag)
		info.ConsumerLag = c.calculateConsumerLag(groupID, info.Topics)
		
		// Collect member IDs
		for _, member := range desc.Members {
			info.Members = append(info.Members, member.MemberId)
		}
		
		groupInfos = append(groupInfos, info)
	}
	
	// Sort by group ID
	sort.Slice(groupInfos, func(i, j int) bool {
		return groupInfos[i].GroupID < groupInfos[j].GroupID
	})
	
	return groupInfos, nil
}

func (c *Client) calculateConsumerLag(groupID string, topics []string) int64 {
	log := logger.Get()
	var totalLag int64
	
	// Get committed offsets for the consumer group
	offsets, err := c.admin.ListConsumerGroupOffsets(groupID, nil)
	if err != nil {
		log.WithField("groupID", groupID).WithError(err).Debug("Failed to get consumer group offsets")
		return 0
	}
	
	// Create a consumer to get high water marks
	consumer, err := sarama.NewConsumer(c.brokers, c.config)
	if err != nil {
		log.WithError(err).Debug("Failed to create consumer for lag calculation")
		return 0
	}
	defer func() {
		if closeErr := consumer.Close(); closeErr != nil {
			log.WithError(closeErr).Debug("Failed to close consumer during lag calculation")
		}
	}()
	
	// Calculate lag for each topic/partition
	for topic, partitionOffsets := range offsets.Blocks {
		// Get partitions for this topic
		partitions, err := consumer.Partitions(topic)
		if err != nil {
			log.WithField("topic", topic).WithError(err).Debug("Failed to get partitions")
			continue
		}
		
		for partitionID, block := range partitionOffsets {
			// Check if this partition exists
			partitionFound := false
			for _, p := range partitions {
				if p == partitionID {
					partitionFound = true
					break
				}
			}
			
			if !partitionFound {
				continue
			}
			
			// Get the partition consumer to fetch high water mark
			pc, err := consumer.ConsumePartition(topic, partitionID, sarama.OffsetNewest)
			if err != nil {
				log.WithField("topic", topic).WithField("partition", partitionID).WithError(err).Debug("Failed to get partition consumer")
				continue
			}
			
			// Get high water mark
			highWaterMark := pc.HighWaterMarkOffset()
			if closeErr := pc.Close(); closeErr != nil {
				log.WithField("topic", topic).WithField("partition", partitionID).WithError(closeErr).Debug("Failed to close partition consumer")
			}
			
			// Calculate lag for this partition
			if highWaterMark > 0 && block.Offset >= 0 {
				lag := highWaterMark - block.Offset
				if lag > 0 {
					totalLag += lag
				}
			}
		}
	}
	
	return totalLag
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

type ConsumerGroupInfo struct {
	GroupID       string
	NumMembers    int
	NumTopics     int
	ConsumerLag   int64
	Coordinator   string
	State         string
	Topics        []string
	Members       []string
}

// ACL represents a Kafka ACL entry
type ACL struct {
	Principal      string
	Host           string
	Operation      string
	PermissionType string
	ResourceType   string
	ResourceName   string
	PatternType    string
}

// ListACLs retrieves all ACLs from the cluster
func (c *Client) ListACLs() ([]ACL, error) {
	log := logger.Get()
	log.Info("Listing ACLs")

	// Create a filter to get all ACLs (empty filter matches all)
	filter := sarama.AclFilter{
		ResourceType:              sarama.AclResourceAny,
		ResourcePatternTypeFilter: sarama.AclPatternAny,
		Operation:                 sarama.AclOperationAny,
		PermissionType:            sarama.AclPermissionAny,
	}

	log.WithFields(map[string]interface{}{
		"filter_resource_type": filter.ResourceType,
		"filter_pattern_type":  filter.ResourcePatternTypeFilter,
		"filter_operation":     filter.Operation,
		"filter_permission":    filter.PermissionType,
	}).Debug("Listing ACLs with filter")

	result, err := c.admin.ListAcls(filter)
	if err != nil {
		log.WithError(err).Error("Failed to describe ACLs")
		return nil, fmt.Errorf("failed to describe ACLs: %w", err)
	}
	
	log.WithField("count", len(result)).Debug("ACL resources found")

	var acls []ACL
	for _, resourceAcls := range result {
		resource := resourceAcls.Resource
		resourceType := getResourceTypeName(resource.ResourceType)
		resourceName := resource.ResourceName
		patternType := getPatternTypeName(resource.ResourcePatternType)

		for _, acl := range resourceAcls.Acls {
			acls = append(acls, ACL{
				Principal:      acl.Principal,
				Host:           acl.Host,
				Operation:      getOperationName(acl.Operation),
				PermissionType: getPermissionTypeName(acl.PermissionType),
				ResourceType:   resourceType,
				ResourceName:   resourceName,
				PatternType:    patternType,
			})
		}
	}

	log.WithField("count", len(acls)).Info("Successfully listed ACLs")
	return acls, nil
}

// CreateACL creates a new ACL in the cluster
func (c *Client) CreateACL(acl ACL) error {
	log := logger.Get()
	log.WithFields(map[string]interface{}{
		"principal":      acl.Principal,
		"resource":       acl.ResourceName,
		"resourceType":   acl.ResourceType,
		"operation":      acl.Operation,
		"permissionType": acl.PermissionType,
		"patternType":    acl.PatternType,
		"host":           acl.Host,
	}).Info("Creating ACL")

	resource := sarama.Resource{
		ResourceType:        parseResourceType(acl.ResourceType),
		ResourceName:        acl.ResourceName,
		ResourcePatternType: parsePatternType(acl.PatternType),
	}

	aclCreation := sarama.Acl{
		Principal:      acl.Principal,
		Host:           acl.Host,
		Operation:      parseOperation(acl.Operation),
		PermissionType: parsePermissionType(acl.PermissionType),
	}

	log.WithFields(map[string]interface{}{
		"resource_type_parsed": resource.ResourceType,
		"pattern_type_parsed":  resource.ResourcePatternType,
		"operation_parsed":     aclCreation.Operation,
		"permission_parsed":    aclCreation.PermissionType,
	}).Debug("Creating ACL with parsed values")

	err := c.admin.CreateACL(resource, aclCreation)
	if err != nil {
		log.WithError(err).Error("Failed to create ACL")
		return fmt.Errorf("failed to create ACL: %w", err)
	}

	log.WithFields(map[string]interface{}{
		"principal":    acl.Principal,
		"resource":     acl.ResourceName,
		"resourceType": acl.ResourceType,
		"operation":    acl.Operation,
	}).Info("Successfully created ACL")
	return nil
}

// DeleteACL deletes an ACL from the cluster
func (c *Client) DeleteACL(acl ACL) error {
	log := logger.Get()
	log.WithFields(map[string]interface{}{
		"principal":    acl.Principal,
		"resource":     acl.ResourceName,
		"resourceType": acl.ResourceType,
		"operation":    acl.Operation,
	}).Info("Deleting ACL")

	filter := sarama.AclFilter{
		ResourceType:              parseResourceType(acl.ResourceType),
		ResourceName:              &acl.ResourceName,
		ResourcePatternTypeFilter: parsePatternType(acl.PatternType),
		Principal:                 &acl.Principal,
		Host:                      &acl.Host,
		Operation:                 parseOperation(acl.Operation),
		PermissionType:            parsePermissionType(acl.PermissionType),
	}

	matches, err := c.admin.DeleteACL(filter, false)
	if err != nil {
		log.WithError(err).Error("Failed to delete ACL")
		return fmt.Errorf("failed to delete ACL: %w", err)
	}

	if len(matches) == 0 {
		return fmt.Errorf("no matching ACLs found to delete")
	}

	log.WithField("deleted", len(matches)).Info("Successfully deleted ACL(s)")
	return nil
}

// Helper functions to convert between Sarama types and strings
func getResourceTypeName(t sarama.AclResourceType) string {
	switch t {
	case sarama.AclResourceTopic:
		return "Topic"
	case sarama.AclResourceGroup:
		return "Group"
	case sarama.AclResourceCluster:
		return "Cluster"
	case sarama.AclResourceTransactionalID:
		return "TransactionalId"
	case sarama.AclResourceDelegationToken:
		return "DelegationToken"
	default:
		return "Unknown"
	}
}

func parseResourceType(s string) sarama.AclResourceType {
	switch s {
	case "Topic":
		return sarama.AclResourceTopic
	case "Group":
		return sarama.AclResourceGroup
	case "Cluster":
		return sarama.AclResourceCluster
	case "TransactionalId":
		return sarama.AclResourceTransactionalID
	case "DelegationToken":
		return sarama.AclResourceDelegationToken
	default:
		return sarama.AclResourceUnknown
	}
}

func getPatternTypeName(t sarama.AclResourcePatternType) string {
	switch t {
	case sarama.AclPatternLiteral:
		return "Literal"
	case sarama.AclPatternPrefixed:
		return "Prefixed"
	default:
		return "Any"
	}
}

func parsePatternType(s string) sarama.AclResourcePatternType {
	switch s {
	case "Literal":
		return sarama.AclPatternLiteral
	case "Prefixed":
		return sarama.AclPatternPrefixed
	default:
		return sarama.AclPatternAny
	}
}

func getOperationName(o sarama.AclOperation) string {
	switch o {
	case sarama.AclOperationRead:
		return "Read"
	case sarama.AclOperationWrite:
		return "Write"
	case sarama.AclOperationCreate:
		return "Create"
	case sarama.AclOperationDelete:
		return "Delete"
	case sarama.AclOperationAlter:
		return "Alter"
	case sarama.AclOperationDescribe:
		return "Describe"
	case sarama.AclOperationClusterAction:
		return "ClusterAction"
	case sarama.AclOperationDescribeConfigs:
		return "DescribeConfigs"
	case sarama.AclOperationAlterConfigs:
		return "AlterConfigs"
	case sarama.AclOperationIdempotentWrite:
		return "IdempotentWrite"
	case sarama.AclOperationAll:
		return "All"
	default:
		return "Unknown"
	}
}

func parseOperation(s string) sarama.AclOperation {
	switch s {
	case "Read":
		return sarama.AclOperationRead
	case "Write":
		return sarama.AclOperationWrite
	case "Create":
		return sarama.AclOperationCreate
	case "Delete":
		return sarama.AclOperationDelete
	case "Alter":
		return sarama.AclOperationAlter
	case "Describe":
		return sarama.AclOperationDescribe
	case "ClusterAction":
		return sarama.AclOperationClusterAction
	case "DescribeConfigs":
		return sarama.AclOperationDescribeConfigs
	case "AlterConfigs":
		return sarama.AclOperationAlterConfigs
	case "IdempotentWrite":
		return sarama.AclOperationIdempotentWrite
	case "All":
		return sarama.AclOperationAll
	default:
		return sarama.AclOperationUnknown
	}
}

func getPermissionTypeName(p sarama.AclPermissionType) string {
	switch p {
	case sarama.AclPermissionAllow:
		return "Allow"
	case sarama.AclPermissionDeny:
		return "Deny"
	default:
		return "Unknown"
	}
}

func parsePermissionType(s string) sarama.AclPermissionType {
	switch s {
	case "Allow":
		return sarama.AclPermissionAllow
	case "Deny":
		return sarama.AclPermissionDeny
	default:
		return sarama.AclPermissionUnknown
	}
}
