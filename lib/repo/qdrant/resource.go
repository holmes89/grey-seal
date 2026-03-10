package qdrant

import (
	"context"
	"fmt"
	"os"

	"github.com/google/uuid"
	pb "github.com/qdrant/go-client/qdrant"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const defaultCollection = "greyseal"

// ResourceVectorRepo implements VectorIndexer against a Qdrant collection.
// Each resource is broken into chunks; each chunk is stored as a separate point
// with payload fields "resource_uuid" and "content".
type ResourceVectorRepo struct {
	client     pb.PointsClient
	collection pb.CollectionsClient
	colName    string
}

func NewResourceVectorRepo() (*ResourceVectorRepo, error) {
	host := os.Getenv("QDRANT_HOST")
	if host == "" {
		host = "qdrant:6334"
	}
	colName := os.Getenv("QDRANT_COLLECTION")
	if colName == "" {
		colName = defaultCollection
	}

	conn, err := grpc.NewClient(host, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to qdrant: %w", err)
	}

	repo := &ResourceVectorRepo{
		client:     pb.NewPointsClient(conn),
		collection: pb.NewCollectionsClient(conn),
		colName:    colName,
	}

	if err := repo.ensureCollection(context.Background()); err != nil {
		return nil, err
	}

	return repo, nil
}

// ensureCollection creates the Qdrant collection if it does not already exist.
func (r *ResourceVectorRepo) ensureCollection(ctx context.Context) error {
	_, err := r.collection.Get(ctx, &pb.GetCollectionInfoRequest{CollectionName: r.colName})
	if err == nil {
		return nil // already exists
	}

	_, err = r.collection.Create(ctx, &pb.CreateCollection{
		CollectionName: r.colName,
		VectorsConfig: &pb.VectorsConfig{
			Config: &pb.VectorsConfig_Params{
				Params: &pb.VectorParams{
					Size:     384, // all-minilm embedding size
					Distance: pb.Distance_Cosine,
				},
			},
		},
	})
	return err
}

// Index chunks and indexes a resource into Qdrant.
// embeddings[i] corresponds to chunks[i].
func (r *ResourceVectorRepo) IndexChunks(ctx context.Context, resourceUUID string, chunks []string, embeddings [][]float32) error {
	var points []*pb.PointStruct
	for i, embedding := range embeddings {
		pointID := uuid.NewSHA1(uuid.NameSpaceURL, []byte(fmt.Sprintf("%s-%d", resourceUUID, i))).String()
		points = append(points, &pb.PointStruct{
			Id: &pb.PointId{
				PointIdOptions: &pb.PointId_Uuid{Uuid: pointID},
			},
			Vectors: &pb.Vectors{
				VectorsOptions: &pb.Vectors_Vector{
					Vector: &pb.Vector{Data: embedding},
				},
			},
			Payload: map[string]*pb.Value{
				"resource_uuid": {Kind: &pb.Value_StringValue{StringValue: resourceUUID}},
				"content":       {Kind: &pb.Value_StringValue{StringValue: chunks[i]}},
			},
		})
	}

	_, err := r.client.Upsert(ctx, &pb.UpsertPoints{
		CollectionName: r.colName,
		Points:         points,
	})
	return err
}

// QueryResult holds a single vector search result.
type QueryResult struct {
	ResourceUUID string
	Content      string
	Score        float32
}

// Query finds the top-k most similar chunks to the given embedding vector.
func (r *ResourceVectorRepo) Query(ctx context.Context, queryVector []float32, limit uint64, resourceUUIDs []string) ([]QueryResult, error) {
	req := &pb.SearchPoints{
		CollectionName: r.colName,
		Vector:         queryVector,
		Limit:          limit,
		WithPayload:    &pb.WithPayloadSelector{SelectorOptions: &pb.WithPayloadSelector_Enable{Enable: true}},
	}

	// Optionally scope the search to specific resource UUIDs.
	if len(resourceUUIDs) > 0 {
		var conditions []*pb.Condition
		for _, id := range resourceUUIDs {
			conditions = append(conditions, &pb.Condition{
				ConditionOneOf: &pb.Condition_Field{
					Field: &pb.FieldCondition{
						Key: "resource_uuid",
						Match: &pb.Match{
							MatchValue: &pb.Match_Keyword{Keyword: id},
						},
					},
				},
			})
		}
		req.Filter = &pb.Filter{Should: conditions}
	}

	resp, err := r.client.Search(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("qdrant search failed: %w", err)
	}

	var results []QueryResult
	for _, hit := range resp.GetResult() {
		res := QueryResult{Score: hit.GetScore()}
		if v, ok := hit.GetPayload()["resource_uuid"]; ok {
			res.ResourceUUID = v.GetStringValue()
		}
		if v, ok := hit.GetPayload()["content"]; ok {
			res.Content = v.GetStringValue()
		}
		results = append(results, res)
	}
	return results, nil
}

// DeleteByResource removes all points associated with a resource UUID.
func (r *ResourceVectorRepo) DeleteByResource(ctx context.Context, resourceUUID string) error {
	_, err := r.client.Delete(ctx, &pb.DeletePoints{
		CollectionName: r.colName,
		Points: &pb.PointsSelector{
			PointsSelectorOneOf: &pb.PointsSelector_Filter{
				Filter: &pb.Filter{
					Must: []*pb.Condition{
						{
							ConditionOneOf: &pb.Condition_Field{
								Field: &pb.FieldCondition{
									Key: "resource_uuid",
									Match: &pb.Match{
										MatchValue: &pb.Match_Keyword{Keyword: resourceUUID},
									},
								},
							},
						},
					},
				},
			},
		},
	})
	return err
}
