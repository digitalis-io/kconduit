package kafka

import (
	"fmt"
	"sort"
	"sync"

	"github.com/IBM/sarama"
	"github.com/digitalis-io/kconduit/pkg/logger"
)

// metricsWorkers caps how many topics are measured at once. Every topic costs
// two offset requests per partition, so a cluster with thousands of topics
// would otherwise open thousands of concurrent requests against the brokers.
const metricsWorkers = 16

// clientConfig returns a copy of the client's configuration for a short-lived
// sarama client to use.
//
// sarama.NewClient validates and, for some broker addresses, rewrites the
// config it is given. Handing it c.config directly would mean writing to a
// struct the long-lived admin and producer clients are reading from other
// goroutines — a data race, and one that could change protocol behaviour for
// the primary clients as a side effect of a background metrics fetch.
func (c *Client) clientConfig() *sarama.Config {
	config := *c.config
	return &config
}

// TopicMetrics is what a topic costs and holds: the number of records currently
// retained, and the disk it occupies.
//
// Messages is the sum over partitions of (log end offset - log start offset).
// It counts records still inside the retention window, not everything ever
// produced, and it over-counts where a partition holds transaction markers.
//
// DiskBytes is the total across every replica in the cluster, not the size of a
// single copy: a 1 GiB topic with replication factor 3 reports 3 GiB, which is
// what it actually costs the cluster.
type TopicMetrics struct {
	Messages  int64
	DiskBytes int64
}

// GetTopicMetrics measures every topic in the cluster.
//
// It is deliberately not part of GetTopicDetails: metadata for a topic list is
// one cheap request, while this is O(partitions) offset lookups, and the UI
// paints the list before these arrive.
func (c *Client) GetTopicMetrics() (map[string]TopicMetrics, error) {
	log := logger.Get()

	client, err := sarama.NewClient(c.brokers, c.clientConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to create client for topic metrics: %w", err)
	}
	defer func() {
		if closeErr := client.Close(); closeErr != nil {
			log.WithError(closeErr).Debug("Failed to close metrics client")
		}
	}()

	topics, err := client.Topics()
	if err != nil {
		return nil, fmt.Errorf("failed to list topics for metrics: %w", err)
	}

	metrics := make(map[string]TopicMetrics, len(topics))
	var mu sync.Mutex

	var wg sync.WaitGroup
	work := make(chan string)

	for range metricsWorkers {
		wg.Go(func() {
			for topic := range work {
				count := topicMessageCount(client, topic)
				mu.Lock()
				entry := metrics[topic]
				entry.Messages = count
				metrics[topic] = entry
				mu.Unlock()
			}
		})
	}

	for _, topic := range topics {
		work <- topic
	}
	close(work)
	wg.Wait()

	// Disk usage comes from the brokers in one request each, so it is collected
	// separately from the per-partition offsets above. A cluster that refuses
	// DescribeLogDirs (older brokers, or missing DESCRIBE on the cluster) still
	// gets message counts; sizes are simply reported as zero.
	for topic, size := range c.topicDiskUsage() {
		entry := metrics[topic]
		entry.DiskBytes = size
		metrics[topic] = entry
	}

	return metrics, nil
}

// topicMessageCount sums (newest - oldest) across a topic's partitions.
func topicMessageCount(client sarama.Client, topic string) int64 {
	log := logger.Get()

	partitions, err := client.Partitions(topic)
	if err != nil {
		log.WithField("topic", topic).WithError(err).Debug("Failed to list partitions for metrics")
		return 0
	}

	// A partition whose offsets cannot be read is skipped rather than failing
	// the whole topic, which makes the count a lower bound. Log it, so a
	// suspiciously low number can be traced back to the partition behind it.
	var total int64
	for _, partition := range partitions {
		newest, err := client.GetOffset(topic, partition, sarama.OffsetNewest)
		if err != nil {
			log.WithField("topic", topic).WithField("partition", partition).
				WithError(err).Debug("Failed to read newest offset for metrics")
			continue
		}
		oldest, err := client.GetOffset(topic, partition, sarama.OffsetOldest)
		if err != nil {
			log.WithField("topic", topic).WithField("partition", partition).
				WithError(err).Debug("Failed to read oldest offset for metrics")
			continue
		}
		if newest > oldest {
			total += newest - oldest
		}
	}
	return total
}

// topicDiskUsage totals the on-disk size of every replica of every topic.
func (c *Client) topicDiskUsage() map[string]int64 {
	log := logger.Get()

	brokers, err := c.GetBrokers()
	if err != nil {
		log.WithError(err).Debug("Failed to list brokers for disk usage")
		return nil
	}

	ids := make([]int32, 0, len(brokers))
	for _, broker := range brokers {
		ids = append(ids, broker.ID)
	}

	logDirs, err := c.admin.DescribeLogDirs(ids)
	if err != nil {
		log.WithError(err).Debug("Failed to describe log dirs")
		return nil
	}

	sizes := make(map[string]int64)
	for _, dirs := range logDirs {
		for _, dir := range dirs {
			for _, topic := range dir.Topics {
				for _, partition := range topic.Partitions {
					sizes[topic.Topic] += partition.Size
				}
			}
		}
	}
	return sizes
}

// PartitionLag is one row of a consumer group's lag breakdown.
type PartitionLag struct {
	Topic     string
	Partition int32
	Current   int64 // committed offset, -1 when the group has never committed
	LogEnd    int64 // high water mark
	Lag       int64
	Member    string // client id of the member owning the partition, if known
}

// GetConsumerGroupLag returns a group's committed offset, log end offset, and
// lag for every partition it has committed against.
func (c *Client) GetConsumerGroupLag(groupID string) ([]PartitionLag, error) {
	log := logger.Get()

	offsets, err := c.admin.ListConsumerGroupOffsets(groupID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get offsets for group %s: %w", groupID, err)
	}

	client, err := sarama.NewClient(c.brokers, c.clientConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to create client for lag breakdown: %w", err)
	}
	defer func() {
		if closeErr := client.Close(); closeErr != nil {
			log.WithError(closeErr).Debug("Failed to close lag client")
		}
	}()

	owners := c.partitionOwners(groupID)

	var rows []PartitionLag
	for topic, partitionOffsets := range offsets.Blocks {
		for partition, block := range partitionOffsets {
			if block == nil {
				continue
			}

			logEnd, err := client.GetOffset(topic, partition, sarama.OffsetNewest)
			if err != nil {
				log.WithField("topic", topic).WithField("partition", partition).
					WithError(err).Debug("Failed to get log end offset")
				continue
			}

			lag := int64(0)
			if block.Offset >= 0 && logEnd > block.Offset {
				lag = logEnd - block.Offset
			}

			rows = append(rows, PartitionLag{
				Topic:     topic,
				Partition: partition,
				Current:   block.Offset,
				LogEnd:    logEnd,
				Lag:       lag,
				Member:    owners[topicPartition{topic, partition}],
			})
		}
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Topic != rows[j].Topic {
			return rows[i].Topic < rows[j].Topic
		}
		return rows[i].Partition < rows[j].Partition
	})

	return rows, nil
}

// topicPartition identifies one partition of one topic.
type topicPartition struct {
	topic     string
	partition int32
}

// partitionOwners maps each assigned partition to the client id holding it.
// A group with no live members simply yields an empty map, and the breakdown
// then shows no owner rather than failing.
func (c *Client) partitionOwners(groupID string) map[topicPartition]string {
	log := logger.Get()

	described, err := c.admin.DescribeConsumerGroups([]string{groupID})
	if err != nil || len(described) == 0 {
		log.WithField("groupID", groupID).WithError(err).Debug("Failed to describe consumer group")
		return nil
	}

	owners := make(map[topicPartition]string)
	for _, member := range described[0].Members {
		assignment, err := member.GetMemberAssignment()
		if err != nil || assignment == nil {
			continue
		}
		for topic, partitions := range assignment.Topics {
			for _, partition := range partitions {
				owners[topicPartition{topic, partition}] = member.ClientId
			}
		}
	}
	return owners
}
