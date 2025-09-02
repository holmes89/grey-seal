package vectordb

import (
	"context"
	"database/sql"
	"fmt"

	greyseal "github.com/holmes89/grey-seal/lib"

	_ "github.com/marcboeker/go-duckdb/v2"
)

var _ greyseal.VectorDB = (*VectorDBImpl)(nil)

// VectorDBImpl wraps DuckDB operations for vector search
// Implements vectordb.VectorDB
type VectorDBImpl struct {
	db   *sql.DB
	conn *sql.Conn
}

func NewVectorDB(dbPath string) (*VectorDBImpl, error) {
	db, err := sql.Open("duckdb", fmt.Sprintf("%s?access_mode=READ_WRITE", dbPath))
	if err != nil {
		return nil, fmt.Errorf("failed to open DuckDB: %w", err)
	}

	conn, err := db.Conn(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to DuckDB: %w", err)
	}

	// Load extensions IMMEDIATELY after getting connection
	// This ensures they're available during WAL replay
	if _, err := conn.ExecContext(context.Background(), "INSTALL vss;"); err != nil {
		return nil, fmt.Errorf("failed to install vss extension: %w", err)
	}
	if _, err := conn.ExecContext(context.Background(), "LOAD vss;"); err != nil {
		return nil, fmt.Errorf("failed to load vss extension: %w", err)
	}
	if _, err := conn.ExecContext(context.Background(), "SET hnsw_enable_experimental_persistence=TRUE;"); err != nil {
		return nil, fmt.Errorf("failed to set HNSW persistence: %w", err)
	}

	vdb := &VectorDBImpl{db: db, conn: conn}

	if err := vdb.setupTables(); err != nil {
		return nil, fmt.Errorf("failed to setup tables: %w", err)
	}

	return vdb, nil
}

// setupTables creates the necessary tables (extensions already loaded)
func (vdb *VectorDBImpl) setupTables() error {
	// Extensions are already loaded in NewVectorDB
	queries := []string{
		`CREATE TABLE IF NOT EXISTS documents (
			id VARCHAR PRIMARY KEY,
			content TEXT NOT NULL,
			file_path VARCHAR NOT NULL,
			chunk_id INTEGER NOT NULL,
			embedding FLOAT[768]
		);`,
		`CREATE INDEX IF NOT EXISTS idx_documents_embedding ON documents USING HNSW (embedding) WITH (metric = 'cosine');`,
	}

	for _, query := range queries {
		if _, err := vdb.conn.ExecContext(context.Background(), query); err != nil {
			return fmt.Errorf("failed to execute query '%s': %w", query, err)
		}
	}
	return nil
}

// StoreDocument stores a document chunk with its embedding
func (vdb *VectorDBImpl) StoreDocument(doc greyseal.DocumentChunk) error {
	vectorStr := greyseal.VectorToString(doc.Vector)
	query := `INSERT OR REPLACE INTO documents (id, content, file_path, chunk_id, embedding) VALUES (?, ?, ?, ?, ?::FLOAT[])`
	_, err := vdb.conn.ExecContext(context.Background(), query, doc.ID, doc.Content, doc.FilePath, doc.ChunkID, vectorStr)
	return err
}

// SearchSimilar finds documents similar to the query vector
func (vdb *VectorDBImpl) SearchSimilar(queryVector []float32, limit int) ([]greyseal.SearchResult, error) {
	if limit <= 0 {
		limit = 5
	}
	vectorStr := greyseal.VectorToString(queryVector)
	query := `SELECT id, content, file_path, chunk_id, array_cosine_distance(embedding, ?::FLOAT[768]) as similarity FROM documents ORDER BY similarity DESC LIMIT ?`
	rows, err := vdb.conn.QueryContext(context.Background(), query, vectorStr, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}
	defer rows.Close()
	var results []greyseal.SearchResult
	for rows.Next() {
		var result greyseal.SearchResult
		err := rows.Scan(&result.ID, &result.Content, &result.FilePath, &result.ChunkID, &result.Similarity)
		if err != nil {
			return nil, fmt.Errorf("failed to scan result: %w", err)
		}
		results = append(results, result)
	}
	return results, nil
}

func (vdb *VectorDBImpl) Close() error {
	fmt.Println("closing")
	if _, err := vdb.conn.ExecContext(context.Background(), "CHECKPOINT;"); err != nil {
		// Log error but continue closing
		fmt.Printf("Warning: failed to checkpoint: %v\n", err)
	}

	if err := vdb.conn.Close(); err != nil {
		fmt.Println(err)
		return err
	}
	if err := vdb.db.Close(); err != nil {
		fmt.Println(err)
		return err
	}
	return nil
}
