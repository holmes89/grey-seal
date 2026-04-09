//go:build integration

package resource_test

import (
	"context"
	"strings"
	"testing"
	"time"

	greysealv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	"github.com/holmes89/grey-seal/lib/greyseal/resource"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kfake"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.uber.org/zap"
	shrikev1 "github.com/holmes89/shrike/lib/schemas/shrike/v1"
	"google.golang.org/protobuf/proto"
)

// consumeOne polls the kgo client until one record arrives on any of the
// given topics, or the context deadline is exceeded.
func consumeOne(t *testing.T, ctx context.Context, cl *kgo.Client) *kgo.Record {
	t.Helper()
	for {
		require.NoError(t, ctx.Err(), "timed out waiting for Kafka message")
		fetches := cl.PollFetches(ctx)
		require.NoError(t, fetches.Err())
		iter := fetches.RecordIter()
		for !iter.Done() {
			return iter.Next()
		}
	}
}

func TestKafkaIndexer_SourceText_Integration(t *testing.T) {
	cluster, err := kfake.NewCluster(kfake.NumBrokers(1))
	require.NoError(t, err)
	defer cluster.Close()

	brokers := strings.Join(cluster.ListenAddrs(), ",")

	indexer := resource.NewKafkaIndexer(brokers, zap.NewNop())

	r := &greysealv1.Resource{
		Uuid:   "uuid-text-001",
		Name:   "My Text Doc",
		Source: greysealv1.Source_SOURCE_TEXT,
		Path:   "Hello, world!",
	}

	ctx := context.Background()
	require.NoError(t, indexer.Index(ctx, r))

	// The topic is derived from the type name: shrikev1.TextExtractedEvent
	textTopic := "shrikev1.TextExtractedEvent"

	consumer, err := kgo.NewClient(
		kgo.SeedBrokers(cluster.ListenAddrs()...),
		kgo.ConsumeTopics(textTopic),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	require.NoError(t, err)
	defer consumer.Close()

	ctx2, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	rec := consumeOne(t, ctx2, consumer)
	require.Equal(t, textTopic, rec.Topic)

	var event shrikev1.TextExtractedEvent
	require.NoError(t, proto.Unmarshal(rec.Value, &event))
	require.Equal(t, "uuid-text-001", event.GetEntityUuid())
	require.Equal(t, "My Text Doc", event.GetTitle())
	require.Equal(t, "Hello, world!", event.GetFullText())
	require.Equal(t, "grey-seal", event.GetApp())
}

func TestKafkaIndexer_SourceWebsite_Integration(t *testing.T) {
	cluster, err := kfake.NewCluster(kfake.NumBrokers(1))
	require.NoError(t, err)
	defer cluster.Close()

	brokers := strings.Join(cluster.ListenAddrs(), ",")

	indexer := resource.NewKafkaIndexer(brokers, zap.NewNop())

	r := &greysealv1.Resource{
		Uuid:   "uuid-web-001",
		Name:   "My Website",
		Source: greysealv1.Source_SOURCE_WEBSITE,
		Path:   "https://example.com",
	}

	ctx := context.Background()
	require.NoError(t, indexer.Index(ctx, r))

	// For SOURCE_WEBSITE the resource is enqueued on: greysealv1.Resource
	resourceTopic := "greysealv1.Resource"

	consumer, err := kgo.NewClient(
		kgo.SeedBrokers(cluster.ListenAddrs()...),
		kgo.ConsumeTopics(resourceTopic),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	require.NoError(t, err)
	defer consumer.Close()

	ctx2, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	rec := consumeOne(t, ctx2, consumer)
	require.Equal(t, resourceTopic, rec.Topic)

	var got greysealv1.Resource
	require.NoError(t, proto.Unmarshal(rec.Value, &got))
	require.Equal(t, "uuid-web-001", got.GetUuid())
	require.Equal(t, "My Website", got.GetName())
	require.Equal(t, greysealv1.Source_SOURCE_WEBSITE, got.GetSource())
}
