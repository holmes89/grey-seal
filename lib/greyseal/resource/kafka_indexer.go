package resource

import (
	"context"
	"strings"
	"time"

	"github.com/holmes89/archaea/kafka"
	greysealv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	shrikev1 "github.com/holmes89/shrike/lib/schemas/shrike/v1"
	"go.uber.org/zap"
)

// kafkaIndexer implements Indexer by publishing events to Kafka/Redpanda.
//
// For SOURCE_TEXT resources, it publishes a TextExtractedEvent directly to the
// shrike ingestion topic — no content fetching is required since the text is
// already in Resource.Path.
//
// For SOURCE_WEBSITE and SOURCE_PDF resources, it enqueues the full Resource
// proto to the grey-seal pending topic so the worker can fetch the content
// asynchronously and then forward a TextExtractedEvent to shrike.
type kafkaIndexer struct {
	textProducer     *kafka.Producer[*shrikev1.TextExtractedEvent]
	resourceProducer *kafka.Producer[*greysealv1.Resource]
	logger           *zap.Logger
}

// NewKafkaIndexer creates an Indexer that publishes to Kafka.
// brokers is a comma-separated list of broker addresses (e.g. "redpanda-0:9092").
func NewKafkaIndexer(brokers string, logger *zap.Logger) Indexer {
	conn := kafka.NewConn(strings.Split(brokers, ","))
	return &kafkaIndexer{
		textProducer:     kafka.NewProducer[*shrikev1.TextExtractedEvent](conn),
		resourceProducer: kafka.NewProducer[*greysealv1.Resource](conn),
		logger:           logger,
	}
}

func (k *kafkaIndexer) Index(ctx context.Context, r *greysealv1.Resource) error {
	if r.Source == greysealv1.Source_SOURCE_TEXT {
		return k.indexText(ctx, r)
	}
	return k.enqueueForFetch(ctx, r)
}

// indexText publishes a TextExtractedEvent for SOURCE_TEXT resources.
// The inline text stored in r.Path is used as the full text directly.
func (k *kafkaIndexer) indexText(ctx context.Context, r *greysealv1.Resource) error {
	event := &shrikev1.TextExtractedEvent{
		EntityUuid: r.Uuid,
		App:        "grey-seal",
		EntityType: "Resource",
		FullText:   r.Path,
		Title:      r.Name,
		SourceUrl:  r.Path,
	}
	if err := k.textProducer.Publish(ctx, event, r.Uuid, time.Now()); err != nil {
		k.logger.Error("failed to publish TextExtractedEvent", zap.String("uuid", r.Uuid), zap.Error(err))
		return err
	}
	k.logger.Info("published TextExtractedEvent to shrike", zap.String("uuid", r.Uuid))
	return nil
}

// enqueueForFetch publishes the Resource proto to the grey-seal worker topic so
// the worker can fetch the content and re-publish it as a TextExtractedEvent.
func (k *kafkaIndexer) enqueueForFetch(ctx context.Context, r *greysealv1.Resource) error {
	if err := k.resourceProducer.Publish(ctx, r, r.Uuid, time.Now()); err != nil {
		k.logger.Error("failed to enqueue resource for fetch", zap.String("uuid", r.Uuid), zap.Error(err))
		return err
	}
	k.logger.Info("enqueued resource for async content fetch", zap.String("uuid", r.Uuid), zap.String("source", r.Source.String()))
	return nil
}
