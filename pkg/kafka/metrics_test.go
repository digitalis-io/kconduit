package kafka

import (
	"errors"
	"testing"

	"github.com/IBM/sarama"
)

// fakeSaramaClient answers only the two calls topicMessageCount makes. The
// embedded interface supplies the rest of sarama.Client's very large method
// set; any unexpected call panics with a nil dereference, which is the desired
// signal that the test is exercising more than it claims to.
type fakeSaramaClient struct {
	sarama.Client

	partitions    []int32
	partitionsErr error
	offsets       map[int32]map[int64]int64 // partition -> requested time -> offset
	offsetErrs    map[int32]bool
}

func (f *fakeSaramaClient) Partitions(string) ([]int32, error) {
	return f.partitions, f.partitionsErr
}

func (f *fakeSaramaClient) GetOffset(_ string, partition int32, time int64) (int64, error) {
	if f.offsetErrs[partition] {
		return 0, errors.New("offset unavailable")
	}
	return f.offsets[partition][time], nil
}

func TestTopicMessageCountSumsPartitions(t *testing.T) {
	client := &fakeSaramaClient{
		partitions: []int32{0, 1},
		offsets: map[int32]map[int64]int64{
			0: {sarama.OffsetOldest: 100, sarama.OffsetNewest: 150},
			1: {sarama.OffsetOldest: 0, sarama.OffsetNewest: 7},
		},
	}

	// 50 retained on partition 0, 7 on partition 1. The count is what is still
	// inside the retention window, not everything ever produced, which is why
	// partition 0's 100 expired records do not appear.
	if got := topicMessageCount(client, "orders"); got != 57 {
		t.Errorf("topicMessageCount = %d, want 57", got)
	}
}

func TestTopicMessageCountSkipsUnreadablePartitions(t *testing.T) {
	client := &fakeSaramaClient{
		partitions: []int32{0, 1},
		offsets: map[int32]map[int64]int64{
			1: {sarama.OffsetOldest: 0, sarama.OffsetNewest: 7},
		},
		offsetErrs: map[int32]bool{0: true},
	}

	// A partition that cannot be read makes the total a lower bound rather than
	// failing the topic outright.
	if got := topicMessageCount(client, "orders"); got != 7 {
		t.Errorf("topicMessageCount with an unreadable partition = %d, want 7", got)
	}
}

func TestTopicMessageCountIgnoresEmptyAndInvertedOffsets(t *testing.T) {
	client := &fakeSaramaClient{
		partitions: []int32{0, 1},
		offsets: map[int32]map[int64]int64{
			0: {sarama.OffsetOldest: 42, sarama.OffsetNewest: 42}, // empty
			1: {sarama.OffsetOldest: 90, sarama.OffsetNewest: 10}, // nonsensical
		},
	}

	if got := topicMessageCount(client, "orders"); got != 0 {
		t.Errorf("topicMessageCount = %d, want 0", got)
	}
}

func TestTopicMessageCountReturnsZeroWhenPartitionsFail(t *testing.T) {
	client := &fakeSaramaClient{partitionsErr: errors.New("no metadata")}

	if got := topicMessageCount(client, "orders"); got != 0 {
		t.Errorf("topicMessageCount = %d, want 0", got)
	}
}
