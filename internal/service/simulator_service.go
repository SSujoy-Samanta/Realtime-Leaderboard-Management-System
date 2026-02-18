package service

import (
	"log"
	"math/rand"
	"time"

	"github.com/SSujoy-Samanta/leaderboard-backend/internal/config"
)

type SimulatorService interface {
	Start()
	Stop()
}

type UserRepository interface {
	GetRandomUserID() (uint, error)
}

type simulatorService struct {
	leaderboardSvc LeaderboardService
	userRepo       UserRepository
	ticker         *time.Ticker
	stopCh         chan bool
	running        bool
}

func NewSimulatorService(
	leaderboardSvc LeaderboardService,
	userRepo UserRepository,
) SimulatorService {
	return &simulatorService{
		leaderboardSvc: leaderboardSvc,
		userRepo:       userRepo,
		stopCh:         make(chan bool),
		running:        false,
	}
}

// Start begins the score update simulation
func (s *simulatorService) Start() {
	if s.running {
		log.Println("⚠️  Simulator already running")
		return
	}

	interval := 3 * time.Second // Default 3 seconds
	if config.AppCfg != nil {
		interval = config.AppCfg.App.ScoreUpdateInterval
	}
	s.ticker = time.NewTicker(interval)
	s.running = true

	log.Printf("🎮 Score simulator started (interval: %v)", interval)

	go func() {
		for {
			select {
			case <-s.ticker.C:
				s.simulateScoreUpdate()
			case <-s.stopCh:
				log.Println("⏹️  Score simulator stopped")
				return
			}
		}
	}()
}

// Stop halts the score update simulation
func (s *simulatorService) Stop() {
	if !s.running {
		return
	}

	s.ticker.Stop()
	s.stopCh <- true
	s.running = false
}

// simulateScoreUpdate updates a random user's score
func (s *simulatorService) simulateScoreUpdate() {
	// Get random user
	userID, err := s.userRepo.GetRandomUserID()
	if err != nil {
		log.Printf("❌ Failed to get random user: %v", err)
		return
	}

	// Generate random rating change (-100 to +100)
	change := rand.Intn(201) - 100

	// Get current rating (simulated - would get from DB in real scenario)
	// For simplicity, we'll generate a new rating in valid range
	newRating := 1500 + change

	// Ensure within bounds
	if newRating < 100 {
		newRating = 100
	}
	if newRating > 5000 {
		newRating = 5000
	}

	// Update score
	if _, err := s.leaderboardSvc.UpdateUserScore(userID, newRating); err != nil {
		log.Printf("❌ Failed to update user %d: %v", userID, err)
		return
	}

	// Success is logged in UpdateUserScore
}