package handler

import (
	"net/http"
	"strconv"

	"github.com/SSujoy-Samanta/leaderboard-backend/internal/service"
	"github.com/gin-gonic/gin"
)

type LeaderboardHandler struct {
	leaderboardSvc service.LeaderboardService
}

func NewLeaderboardHandler(leaderboardSvc service.LeaderboardService) *LeaderboardHandler {
	return &LeaderboardHandler{
		leaderboardSvc: leaderboardSvc,
	}
}

// GetLeaderboard godoc
// @Summary Get top users leaderboard
// @Description Returns the top N users with their ranks
// @Tags leaderboard
// @Accept json
// @Produce json
// @Param limit query int false "Number of users to return" default(100)
// @Success 200 {array} models.LeaderboardEntry
// @Router /leaderboard [get]
func (h *LeaderboardHandler) GetLeaderboard(c *gin.Context) {
	// Parse limit parameter
	limitStr := c.DefaultQuery("limit", "100")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000 // Max limit
	}

	// Get leaderboard
	entries, err := h.leaderboardSvc.GetLeaderboard(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch leaderboard",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"count":   len(entries),
		"data":    entries,
	})
}

// GetUserRank godoc
// @Summary Get user's global rank
// @Description Returns the global rank of a specific user
// @Tags leaderboard
// @Accept json
// @Produce json
// @Param user_id path int true "User ID"
// @Success 200 {object} map[string]interface{}
// @Router /leaderboard/user/{user_id}/rank [get]
func (h *LeaderboardHandler) GetUserRank(c *gin.Context) {
	// Parse user ID
	userIDStr := c.Param("user_id")
	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid user ID",
		})
		return
	}

	// Get rank
	rank, err := h.leaderboardSvc.GetUserRank(uint(userID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "User not found in leaderboard",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"user_id": userID,
		"rank":    rank,
	})
}

// UpdateUserScore godoc
// @Summary Update user's score
// @Description Updates a user's rating and recalculates their rank
// @Tags leaderboard
// @Accept json
// @Produce json
// @Param user_id path int true "User ID"
// @Param body body map[string]int true "New Rating"
// @Success 200 {object} map[string]interface{}
// @Router /leaderboard/user/{user_id}/score [put]
func (h *LeaderboardHandler) UpdateUserScore(c *gin.Context) {
	// Parse user ID
	userIDStr := c.Param("user_id")
	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid user ID",
		})
		return
	}

	// Parse request body
	var req struct {
		NewRating int `json:"new_rating" binding:"required,min=100,max=5000"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body. new_rating must be between 100 and 5000",
		})
		return
	}

	// Update score (Redis-first, returns payload with rank delta)
	payload, err := h.leaderboardSvc.UpdateUserScore(uint(userID), req.NewRating)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to update score",
		})
		return
	}

	// Return full payload with rank delta
	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"user_id":      payload.UserID,
		"username":     payload.Username,
		"old_rating":   payload.OldRating,
		"new_rating":   payload.NewRating,
		"rating_delta": payload.RatingDelta, // +50, -30, etc.
		"old_rank":     payload.OldRank,
		"new_rank":     payload.NewRank,
		"rank_delta":   payload.RankDelta, // +2, -10, etc.
		"timestamp":    payload.Timestamp,
	})
}

// GetStats godoc
// @Summary Get leaderboard statistics
// @Description Returns statistics about the leaderboard
// @Tags leaderboard
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /leaderboard/stats [get]
func (h *LeaderboardHandler) GetStats(c *gin.Context) {
	stats, err := h.leaderboardSvc.GetLeaderboardStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch stats",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}
