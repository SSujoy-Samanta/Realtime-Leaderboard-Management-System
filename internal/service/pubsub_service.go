package service

import (
	"context"
	"encoding/json"
	"log"

	"github.com/SSujoy-Samanta/leaderboard-backend/internal/database"
	"github.com/SSujoy-Samanta/leaderboard-backend/internal/models"
	"github.com/redis/go-redis/v9"
)

const (
	ScoreUpdateChannel = "leaderboard:score_updates"
)

type PubSubService interface {
	Start(messageHandler func(*models.ScoreUpdatePayload))
	Stop()
	Publish(payload *models.ScoreUpdatePayload) error
}

type pubSubService struct {
	redis     *redis.Client
	ctx       context.Context
	cancelCtx context.CancelFunc
	pubsub    *redis.PubSub
	running   bool
}

func NewPubSubService(redisClient *redis.Client) PubSubService {
	ctx, cancel := context.WithCancel(database.Ctx)

	return &pubSubService{
		redis:     redisClient,
		ctx:       ctx,
		cancelCtx: cancel,
		running:   false,
	}
}

// Start subscribes to Redis channel and handles incoming messages
func (s *pubSubService) Start(messageHandler func(*models.ScoreUpdatePayload)) {
	if s.running {
		log.Println("⚠️  PubSub service already running")
		return
	}

	// Subscribe to channel
	s.pubsub = s.redis.Subscribe(s.ctx, ScoreUpdateChannel)
	s.running = true

	log.Printf("📡 PubSub service started (subscribed to: %s)", ScoreUpdateChannel)

	// Start listening in goroutine
	go func() {
		defer func() {
			s.pubsub.Close()
			s.running = false
		}()

		// Receive messages
		ch := s.pubsub.Channel()

		for {
			select {
			case msg := <-ch:
				// Parse message
				var payload models.ScoreUpdatePayload
				if err := json.Unmarshal([]byte(msg.Payload), &payload); err != nil {
					log.Printf("⚠️  Failed to unmarshal PubSub message: %v", err)
					continue
				}

				// Call handler (broadcasts to local WebSocket clients)
				if messageHandler != nil {
					messageHandler(&payload)
				}

			case <-s.ctx.Done():
				log.Println("⏹️  PubSub subscription stopped")
				return
			}
		}
	}()
}

// Stop unsubscribes and closes the subscription
func (s *pubSubService) Stop() {
	if !s.running {
		return
	}

	log.Println("⏹️  Stopping PubSub service...")
	s.cancelCtx()
}

// Publish sends a score update to Redis channel (broadcasts to ALL servers)
func (s *pubSubService) Publish(payload *models.ScoreUpdatePayload) error {
	// Serialize payload
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// Publish to Redis channel
	// All subscribed servers (including this one) will receive it
	return s.redis.Publish(s.ctx, ScoreUpdateChannel, data).Err()
}
