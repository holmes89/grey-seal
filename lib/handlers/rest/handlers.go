package rest

import (
	"net/http"

	"github.com/gin-gonic/gin"
	greyseal "github.com/holmes89/grey-seal/lib"
)

// RestHandler defines the interface for setting up REST routes.
type RestHandler interface {
	SetupRoutes() *gin.Engine
}

type RestHandlerImpl struct {
	RAGService   greyseal.RAGService
	DocProcessor greyseal.DocumentProcessingService
}

func NewRestHandler(ragService greyseal.RAGService, docProcessor greyseal.DocumentProcessingService) *RestHandlerImpl {
	return &RestHandlerImpl{
		RAGService:   ragService,
		DocProcessor: docProcessor,
	}
}

func (h *RestHandlerImpl) SetupRoutes() *gin.Engine {
	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	r.POST("/ingest", func(c *gin.Context) {
		var req struct {
			DirectoryPath string `json:"directory_path"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := h.DocProcessor.ProcessDirectory(req.DirectoryPath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Documents processed successfully"})
	})

	r.POST("/query", func(c *gin.Context) {
		var req greyseal.RAGRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if req.Limit == 0 {
			req.Limit = 5
		}
		response, err := h.RAGService.Query(c.Request.Context(), req.Query, req.Limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, response)
	})

	r.POST("/search", func(c *gin.Context) {
		var req greyseal.RAGRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		// For search, just use the RAGService's vectorDB and embedding
		response, err := h.RAGService.Query(c.Request.Context(), req.Query, req.Limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"results": response.Context})
	})

	return r
}
