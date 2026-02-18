package service

import (
	"fmt"
	"log"
	"time"

	"github.com/SSujoy-Samanta/leaderboard-backend/internal/models"
	"github.com/SSujoy-Samanta/leaderboard-backend/internal/repository"
)

type LeaderboardService interface {
	GetLeaderboard(limit int) ([]models.LeaderboardEntry, error)
	GetUserRank(userID uint) (int64, error)
	UpdateUserScore(userID uint, newRating int) (*models.ScoreUpdatePayload, error)
	SyncUserToLeaderboard(user *models.User) error
	GetLeaderboardStats() (map[string]interface{}, error)
}

type leaderboardService struct {
	userRepo        repository.UserRepository
	leaderboardRepo repository.LeaderboardRepository
	scoreUpdateRepo repository.ScoreUpdateRepository
	dbSyncService   DBSyncService
	pubSubService   PubSubService
}

func NewLeaderboardService(
	userRepo repository.UserRepository,
	leaderboardRepo repository.LeaderboardRepository,
	scoreUpdateRepo repository.ScoreUpdateRepository,
	dbSyncService DBSyncService,
	pubSubService PubSubService,
) LeaderboardService {
	return &leaderboardService{
		userRepo:        userRepo,
		leaderboardRepo: leaderboardRepo,
		scoreUpdateRepo: scoreUpdateRepo,
		dbSyncService:   dbSyncService,
		pubSubService:   pubSubService,
	}
}

// GetLeaderboard returns top N users with their ranks
func (s *leaderboardService) GetLeaderboard(limit int) ([]models.LeaderboardEntry, error) {
	// Get top users from Redis sorted set
	entries, err := s.leaderboardRepo.GetTopUsers(limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get leaderboard: %w", err)
	}

	// Enrich with usernames
	for i := range entries {
		// Try cache first
		user, err := s.leaderboardRepo.GetCachedUser(entries[i].UserID)
		if err != nil {
			// Fallback to database
			user, err = s.userRepo.GetByID(entries[i].UserID)
			if err != nil {
				log.Printf("Failed to get user %d: %v", entries[i].UserID, err)
				continue
			}
			// Cache for next time
			s.leaderboardRepo.CacheUser(user)
		}

		entries[i].Username = user.Username
	}

	return entries, nil
}

// GetUserRank returns the global rank of a user
func (s *leaderboardService) GetUserRank(userID uint) (int64, error) {
	rank, err := s.leaderboardRepo.GetUserRank(userID)
	if err != nil {
		return 0, fmt.Errorf("failed to get user rank: %w", err)
	}
	return rank, nil
}

// UpdateUserScore updates a user's rating and recalculates rank
func (s *leaderboardService) UpdateUserScore(userID uint, newRating int) (*models.ScoreUpdatePayload, error) {
	// Validate rating bounds
	if newRating < 100 {
		newRating = 100
	}
	if newRating > 5000 {
		newRating = 5000
	}

	// STEP 1: Get current state from Redis (fast!)
	user, err := s.leaderboardRepo.GetCachedUser(userID)
	if err != nil {
		// Fallback to PostgreSQL if not in cache
		user, err = s.userRepo.GetByID(userID)
		if err != nil {
			return nil, fmt.Errorf("user not found: %w", err)
		}
	}

	oldRating := user.Rating
	oldRank, err := s.leaderboardRepo.GetUserRank(userID)
	if err != nil {
		oldRank = 0 // First time in leaderboard
	}

	// STEP 2: Update Redis IMMEDIATELY (hot path - 5ms)
	if err := s.leaderboardRepo.UpdateUserScore(userID, newRating); err != nil {
		return nil, fmt.Errorf("failed to update Redis: %w", err)
	}

	// Update cache
	user.Rating = newRating
	s.leaderboardRepo.CacheUser(user)

	// STEP 3: Get new rank and calculate delta
	newRank, err := s.leaderboardRepo.GetUserRank(userID)
	if err != nil {
		newRank = 0
	}

	// Calculate deltas
	// Rank delta: positive = improved (went UP in ranking, lower number)
	rankDelta := oldRank - newRank // If went from #100 to #50, delta = +50
	ratingDelta := newRating - oldRating

	// STEP 4: Create update payload with deltas
	payload := &models.ScoreUpdatePayload{
		UserID:      userID,
		Username:    user.Username,
		OldRating:   oldRating,
		NewRating:   newRating,
		OldRank:     oldRank,
		NewRank:     newRank,
		RankDelta:   rankDelta,   // +50 = improved 50 positions
		RatingDelta: ratingDelta, // +100 = gained 100 rating points
		Timestamp:   time.Now().Unix(),
	}

	// STEP 5: Publish to Redis Pub/Sub (broadcasts to ALL servers)
	if err := s.pubSubService.Publish(payload); err != nil {
		log.Printf("⚠️  Failed to publish score update: %v", err)
		// Don't fail the request if broadcast fails
	}

	// STEP 6: Enqueue async DB sync (Redis Stream)
	err = s.dbSyncService.EnqueueUpdate(models.DBSyncQueueItem{
		UserID:    userID,
		OldRating: oldRating,
		NewRating: newRating,
		Timestamp: time.Now(),
	})

	if err != nil {
		// IMPORTANT: do NOT fail user flow
		log.Printf("⚠️ Failed to enqueue DB sync for user %d: %v", userID, err)
	}

	log.Printf("Updated user %d (%s): %d -> %d (rank: %d)",
		userID, user.Username, oldRating, newRating, newRank)

	return payload, nil
}

// SyncUserToLeaderboard adds/updates user in Redis leaderboard
func (s *leaderboardService) SyncUserToLeaderboard(user *models.User) error {
	// Add to leaderboard
	if err := s.leaderboardRepo.AddUser(user.ID, user.Rating); err != nil {
		return err
	}

	// Cache user data
	if err := s.leaderboardRepo.CacheUser(user); err != nil {
		return err
	}

	// Username search uses PostgreSQL (no Redis indexing needed)

	return nil
}

// GetLeaderboardStats returns leaderboard statistics
func (s *leaderboardService) GetLeaderboardStats() (map[string]interface{}, error) {
	totalUsers, err := s.userRepo.Count()
	if err != nil {
		return nil, err
	}

	leaderboardSize, err := s.leaderboardRepo.GetLeaderboardSize()
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"total_users":      totalUsers,
		"leaderboard_size": leaderboardSize,
	}, nil
}
