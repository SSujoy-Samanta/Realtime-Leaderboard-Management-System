package service

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/SSujoy-Samanta/leaderboard-backend/internal/database"
	"github.com/SSujoy-Samanta/leaderboard-backend/internal/models"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

const (
	ScoreUpdateStream = "stream:score_updates"
	ConsumerGroup     = "db-sync-group"
	ConsumerName      = "worker-1"

	BatchSize    = 100
	BlockTimeout = 5 * time.Second

	StreamMaxLen      = 100     // keep last ~100 events
	TrimEveryNBatches = 10      // trim once every 10 batches
)

type DBSyncService interface {
	Start()
	Stop()
	EnqueueUpdate(item models.DBSyncQueueItem) error
}

type dbSyncService struct {
	redis        *redis.Client
	db           *gorm.DB
	ctx          context.Context
	stopCh       chan struct{}
	running      bool
	mu           sync.Mutex
	batchCounter int
}

func NewDBSyncService(redisClient *redis.Client, db *gorm.DB) DBSyncService {
	svc := &dbSyncService{
		redis:  redisClient,
		db:     db,
		ctx:    database.Ctx,
		stopCh: make(chan struct{}),
	}

	svc.initStream()
	return svc
}

func (s *dbSyncService) initStream() {
	// Create consumer group (idempotent)
	err := s.redis.XGroupCreateMkStream(
		s.ctx,
		ScoreUpdateStream,
		ConsumerGroup,
		"0",
	).Err()

	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		log.Fatalf("❌ Failed to create Redis consumer group: %v", err)
	}
}

// Start worker
func (s *dbSyncService) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	log.Println("🔄 DB Sync Worker started (Redis Streams)")
	go s.worker()
}

func (s *dbSyncService) Stop() {
	close(s.stopCh)
	log.Println("⏹️ DB Sync Worker stopping...")
}

// Producer: add event to stream
func (s *dbSyncService) EnqueueUpdate(item models.DBSyncQueueItem) error {
	data, err := json.Marshal(item)
	if err != nil {
		return err
	}

	return s.redis.XAdd(s.ctx, &redis.XAddArgs{
		Stream: ScoreUpdateStream,
		Values: map[string]interface{}{
			"data": data,
		},
	}).Err()
}

// Worker loop
func (s *dbSyncService) worker() {
	for {
		select {
		case <-s.stopCh:
			return
		default:
			s.processBatch()
		}
	}
}

// Read + process messages
func (s *dbSyncService) processBatch() {
	streams, err := s.redis.XReadGroup(
		s.ctx,
		&redis.XReadGroupArgs{
			Group:    ConsumerGroup,
			Consumer: ConsumerName,
			Streams:  []string{ScoreUpdateStream, ">"},
			Count:    BatchSize,
			Block:    BlockTimeout,
		},
	).Result()

	if err != nil && err != redis.Nil {
		log.Printf("⚠️ Redis XREADGROUP error: %v", err)
		return
	}

	if len(streams) == 0 {
		return
	}

	var (
		items      []models.DBSyncQueueItem
		messageIDs []string
	)

	for _, stream := range streams {
		for _, msg := range stream.Messages {
			raw := msg.Values["data"].(string)

			var item models.DBSyncQueueItem
			if err := json.Unmarshal([]byte(raw), &item); err != nil {
				continue
			}

			items = append(items, item)
			messageIDs = append(messageIDs, msg.ID)
		}
	}

	if len(items) == 0 {
		return
	}

	// DB transaction
	err = s.db.Transaction(func(tx *gorm.DB) error {
		for _, item := range items {
			if err := tx.Model(&models.User{}).
				Where("id = ?", item.UserID).
				Update("rating", item.NewRating).Error; err != nil {
				return err
			}

			history := models.ScoreUpdate{
				UserID:    item.UserID,
				OldRating: item.OldRating,
				NewRating: item.NewRating,
				Change:    item.NewRating - item.OldRating,
				UpdatedAt: item.Timestamp,
			}

			if err := tx.Create(&history).Error; err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		log.Printf("❌ DB sync failed, retrying later: %v", err)
		return
	}

	// ACK messages ONLY after DB commit
	s.redis.XAck(
		s.ctx,
		ScoreUpdateStream,
		ConsumerGroup,
		messageIDs...,
	)

	// Increment batch counter
	s.batchCounter++

	// Periodic stream trim (NON-BLOCKING maintenance)
	if s.batchCounter%TrimEveryNBatches == 0 {
		go s.trimStream()
	}

	log.Printf("💾 DB Sync success: %d items", len(items))
}

func (s *dbSyncService) trimStream() {
	err := s.redis.XTrimMaxLen(
		s.ctx,
		ScoreUpdateStream,
		StreamMaxLen,
	).Err()

	if err != nil {
		log.Printf("⚠️ Failed to trim Redis stream: %v", err)
		return
	}

	log.Printf("🧹 Trimmed Redis stream to ~%d entries", StreamMaxLen)
}
