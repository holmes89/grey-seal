package docproc

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	greyseal "github.com/holmes89/grey-seal/lib"
)

var _ greyseal.DocumentProcessingService = (*DocumentProcessorImpl)(nil)

type DocumentProcessorImpl struct {
	vectorDB   greyseal.VectorDB
	embeddings greyseal.EmbeddingService
}

func NewDocumentProcessor(vdb greyseal.VectorDB, es greyseal.EmbeddingService) *DocumentProcessorImpl {
	return &DocumentProcessorImpl{
		vectorDB:   vdb,
		embeddings: es,
	}
}

func (dp *DocumentProcessorImpl) ProcessDirectory(dirPath string) error {
	return filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".txt") {
			log.Printf("Processing file: %s", path)
			return dp.ProcessFile(path)
		}
		return nil
	})
}

func (dp *DocumentProcessorImpl) ProcessFile(filePath string) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}
	chunks := greyseal.ChunkText(string(content), 500)
	for i, chunk := range chunks {
		vector, err := dp.embeddings.GenerateEmbedding(chunk)
		if err != nil {
			return fmt.Errorf("failed to generate embedding: %w", err)
		}
		doc := greyseal.DocumentChunk{
			ID:       fmt.Sprintf("%s_chunk_%d", filepath.Base(filePath), i),
			Content:  chunk,
			FilePath: filePath,
			ChunkID:  i,
			Vector:   vector,
		}
		if err := dp.vectorDB.StoreDocument(doc); err != nil {
			return fmt.Errorf("failed to store document: %w", err)
		}
	}
	log.Printf("Processed %s into %d chunks", filePath, len(chunks))
	return nil
}
