package models

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Username  string         `gorm:"uniqueIndex:idx_username;size:50;not null" json:"username"`
	Rating    int            `gorm:"index:idx_rating_desc,sort:desc;not null;default:1500" json:"rating"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName specifies the table name
func (User) TableName() string {
	return "users"
}

// BeforeCreate hook to validate rating
func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.Rating < 100 {
		u.Rating = 100
	}
	if u.Rating > 5000 {
		u.Rating = 5000
	}
	return nil
}

// BeforeUpdate hook to validate rating
func (u *User) BeforeUpdate(tx *gorm.DB) error {
	if u.Rating < 100 {
		u.Rating = 100
	}
	if u.Rating > 5000 {
		u.Rating = 5000
	}
	return nil
}

// ScoreUpdate represents a score change history
type ScoreUpdate struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index:idx_user_updates;not null" json:"user_id"`
	User      User      `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"` // Foreign key relation
	OldRating int       `json:"old_rating"`
	NewRating int       `json:"new_rating"`
	Change    int       `json:"change"`
	UpdatedAt time.Time `gorm:"index:idx_update_time" json:"updated_at"`
}

func (ScoreUpdate) TableName() string {
	return "score_updates"
}

// LeaderboardEntry represents a leaderboard entry with rank
type LeaderboardEntry struct {
	Rank     int64  `json:"rank"`
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Rating   int    `json:"rating"`
}

// SearchResult represents search result with global rank
type SearchResult struct {
	GlobalRank int64  `json:"global_rank"`
	UserID     uint   `json:"user_id"`
	Username   string `json:"username"`
	Rating     int    `json:"rating"`
}

// ScoreUpdateRequest represents a score update request
type ScoreUpdateRequest struct {
	UserID    uint `json:"user_id" binding:"required"`
	NewRating int  `json:"new_rating" binding:"required,min=100,max=5000"`
}

// WebSocketMessage represents real-time update message
type WebSocketMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

// ScoreUpdatePayload represents score update WebSocket payload
type ScoreUpdatePayload struct {
	UserID    uint   `json:"user_id"`
	Username  string `json:"username"`
	OldRating int    `json:"old_rating"`
	NewRating int    `json:"new_rating"`
	OldRank    int64  `json:"old_rank"`
	NewRank   int64  `json:"new_rank"`
	RankDelta  int64  `json:"rank_delta"` // +2, -10, etc. (positive = improved)
	RatingDelta int   `json:"rating_delta"` // +50, -30, etc.
	Timestamp  int64  `json:"timestamp"`
}

// DBSyncQueueItem represents an item in the async DB sync queue
type DBSyncQueueItem struct {
	UserID    uint
	OldRating int
	NewRating int
	Timestamp time.Time
}