package repository

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/SSujoy-Samanta/leaderboard-backend/internal/database"
	"github.com/SSujoy-Samanta/leaderboard-backend/internal/models"
	"github.com/redis/go-redis/v9"
)

type LeaderboardRepository interface {
	AddUser(userID uint, rating int) error
	UpdateUserScore(userID uint, rating int) error
	GetUserRank(userID uint) (int64, error)
	GetTopUsers(limit int) ([]models.LeaderboardEntry, error)
	GetUsersByRating(rating int) ([]uint, error)
	RemoveUser(userID uint) error
	GetLeaderboardSize() (int64, error)
	CacheUser(user *models.User) error
	GetCachedUser(userID uint) (*models.User, error)
}

type leaderboardRepository struct {
	redis *redis.Client
	ctx   context.Context
}

func NewLeaderboardRepository(redisClient *redis.Client) LeaderboardRepository {
	return &leaderboardRepository{
		redis: redisClient,
		ctx:   database.Ctx,
	}
}

// AddUser adds a user to the leaderboard sorted set
func (r *leaderboardRepository) AddUser(userID uint, rating int) error {
	return r.redis.ZAdd(r.ctx, database.LeaderboardKey, redis.Z{
		Score:  float64(rating),
		Member: fmt.Sprintf("user:%d", userID),
	}).Err()
}

// UpdateUserScore updates user's score in leaderboard
func (r *leaderboardRepository) UpdateUserScore(userID uint, rating int) error {
	return r.AddUser(userID, rating) // ZAdd handles both add and update
}

// GetUserRank returns the global rank of a user (1-indexed, handles ties)
func (r *leaderboardRepository) GetUserRank(userID uint) (int64, error) {
	member := fmt.Sprintf("user:%d", userID)

	// Get user's score
	score, err := r.redis.ZScore(r.ctx, database.LeaderboardKey, member).Result()
	if err != nil {
		if err == redis.Nil {
			return 0, fmt.Errorf("user not found in leaderboard")
		}
		return 0, err
	}

	// Count users with HIGHER scores (exclusive)
	higherCount, err := r.redis.ZCount(r.ctx, database.LeaderboardKey,
		fmt.Sprintf("(%f", score), "+inf").Result()
	if err != nil {
		return 0, err
	}

	// Rank = number of users with higher scores + 1
	return higherCount + 1, nil
}

// GetTopUsers returns top N users from leaderboard with ranks
func (r *leaderboardRepository) GetTopUsers(limit int) ([]models.LeaderboardEntry, error) {
	results, err := r.redis.ZRevRangeWithScores(r.ctx, database.LeaderboardKey, 0, int64(limit-1)).Result()
	if err != nil {
		return nil, err
	}

	entries := make([]models.LeaderboardEntry, 0, len(results))
	currentRank := int64(1)
	var previousScore float64

	for i, z := range results {
		if i > 0 && z.Score != previousScore {
			currentRank = int64(i) + 1
		}

		userIDStr := strings.TrimPrefix(z.Member.(string), "user:")
		userID, _ := strconv.ParseUint(userIDStr, 10, 32)

		entries = append(entries, models.LeaderboardEntry{
			Rank:   currentRank,
			UserID: uint(userID),
			Rating: int(z.Score),
		})

		previousScore = z.Score
	}

	return entries, nil
}

// GetUsersByRating returns all users with a specific rating
func (r *leaderboardRepository) GetUsersByRating(rating int) ([]uint, error) {
	score := float64(rating)

	members, err := r.redis.ZRangeByScore(r.ctx, database.LeaderboardKey, &redis.ZRangeBy{
		Min: fmt.Sprintf("%f", score),
		Max: fmt.Sprintf("%f", score),
	}).Result()

	if err != nil {
		return nil, err
	}

	userIDs := make([]uint, 0, len(members))
	for _, member := range members {
		userIDStr := strings.TrimPrefix(member, "user:")
		userID, _ := strconv.ParseUint(userIDStr, 10, 32)
		userIDs = append(userIDs, uint(userID))
	}

	return userIDs, nil
}

// RemoveUser removes a user from leaderboard
func (r *leaderboardRepository) RemoveUser(userID uint) error {
	member := fmt.Sprintf("user:%d", userID)
	return r.redis.ZRem(r.ctx, database.LeaderboardKey, member).Err()
}

// GetLeaderboardSize returns total number of users in leaderboard
func (r *leaderboardRepository) GetLeaderboardSize() (int64, error) {
	return r.redis.ZCard(r.ctx, database.LeaderboardKey).Result()
}

// CacheUser caches user data in Redis hash
func (r *leaderboardRepository) CacheUser(user *models.User) error {
	key := fmt.Sprintf(database.UserCacheKey, user.ID)

	return r.redis.HSet(r.ctx, key,
		"id", user.ID,
		"username", user.Username,
		"rating", user.Rating,
	).Err()
}

// GetCachedUser retrieves cached user data
func (r *leaderboardRepository) GetCachedUser(userID uint) (*models.User, error) {
	key := fmt.Sprintf(database.UserCacheKey, userID)

	result, err := r.redis.HGetAll(r.ctx, key).Result()
	if err != nil || len(result) == 0 {
		return nil, fmt.Errorf("user not in cache")
	}

	id, _ := strconv.ParseUint(result["id"], 10, 32)
	rating, _ := strconv.Atoi(result["rating"])

	return &models.User{
		ID:       uint(id),
		Username: result["username"],
		Rating:   rating,
	}, nil
}