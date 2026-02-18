package service

import (
	"fmt"
	"sort"

	"github.com/SSujoy-Samanta/leaderboard-backend/internal/models"
	"github.com/SSujoy-Samanta/leaderboard-backend/internal/repository"
)

type SearchService interface {
	SearchUsers(query string, limit int) ([]models.SearchResult, error)
}

type searchService struct {
	userRepo        repository.UserRepository
	leaderboardRepo repository.LeaderboardRepository
	leaderboardSvc  LeaderboardService
}

func NewSearchService(
	userRepo repository.UserRepository,
	leaderboardRepo repository.LeaderboardRepository,
	leaderboardSvc LeaderboardService,
) SearchService {
	return &searchService{
		userRepo:        userRepo,
		leaderboardRepo: leaderboardRepo,
		leaderboardSvc:  leaderboardSvc,
	}
}

// SearchUsers searches for users by username and returns results with global ranks
// OPTIMIZED: Uses PostgreSQL only (no Redis prefix search)
func (s *searchService) SearchUsers(query string, limit int) ([]models.SearchResult, error) {
	if len(query) < 1 {
		return []models.SearchResult{}, nil
	}

	// Use PostgreSQL fuzzy search with trigram index (fast enough!)
	users, err := s.userRepo.SearchByUsername(query, limit)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// Sort by rating (descending)
	sort.Slice(users, func(i, j int) bool {
		return users[i].Rating > users[j].Rating
	})

	// Limit results
	if len(users) > limit {
		users = users[:limit]
	}

	// Build search results with global ranks
	results := make([]models.SearchResult, 0, len(users))

	for _, user := range users {
		// Get global rank for each user from Redis
		rank, err := s.leaderboardSvc.GetUserRank(user.ID)
		if err != nil {
			// If rank not found, skip this user
			continue
		}

		results = append(results, models.SearchResult{
			GlobalRank: rank,
			UserID:     user.ID,
			Username:   user.Username,
			Rating:     user.Rating,
		})
	}

	// Sort by rating descending (maintain rank order)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Rating > results[j].Rating
	})

	return results, nil
}
