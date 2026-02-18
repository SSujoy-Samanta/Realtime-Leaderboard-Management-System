package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/SSujoy-Samanta/leaderboard-backend/internal/service"
)

type SearchHandler struct {
	searchSvc service.SearchService
}

func NewSearchHandler(searchSvc service.SearchService) *SearchHandler {
	return &SearchHandler{
		searchSvc: searchSvc,
	}
}

// SearchUsers godoc
// @Summary Search users by username
// @Description Searches for users by username and returns results with global ranks
// @Tags search
// @Accept json
// @Produce json
// @Param q query string true "Search query"
// @Param limit query int false "Maximum results" default(100)
// @Success 200 {array} models.SearchResult
// @Router /search [get]
func (h *SearchHandler) SearchUsers(c *gin.Context) {
	// Get search query
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Search query 'q' is required",
		})
		return
	}

	// Parse limit
	limitStr := c.DefaultQuery("limit", "100")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 100
	}
	if limit > 200 {
		limit = 200 // Max limit for search
	}

	// Search users
	results, err := h.searchSvc.SearchUsers(query, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Search failed",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"query":   query,
		"count":   len(results),
		"data":    results,
	})
}