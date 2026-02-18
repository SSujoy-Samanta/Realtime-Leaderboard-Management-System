package repository

import (
	"github.com/SSujoy-Samanta/leaderboard-backend/internal/models"
	"gorm.io/gorm"
)

type UserRepository interface {
	Create(user *models.User) error
	GetByID(id uint) (*models.User, error)
	GetByUsername(username string) (*models.User, error)
	Update(user *models.User) error
	UpdateRating(userID uint, newRating int) error
	GetAll(limit, offset int) ([]models.User, error)
	Count() (int64, error)
	SearchByUsername(query string, limit int) ([]models.User, error)
	GetTopUsers(limit int) ([]models.User, error)
	GetRandomUserID() (uint, error)
}

type userRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(user *models.User) error {
	return r.db.Create(user).Error
}

func (r *userRepository) GetByID(id uint) (*models.User, error) {
	var user models.User
	err := r.db.First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) GetByUsername(username string) (*models.User, error) {
	var user models.User
	err := r.db.Where("username = ?", username).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) Update(user *models.User) error {
	return r.db.Save(user).Error
}

func (r *userRepository) UpdateRating(userID uint, newRating int) error {
	return r.db.Model(&models.User{}).
		Where("id = ?", userID).
		Update("rating", newRating).Error
}

func (r *userRepository) GetAll(limit, offset int) ([]models.User, error) {
	var users []models.User
	err := r.db.Order("rating DESC, username ASC").
		Limit(limit).
		Offset(offset).
		Find(&users).Error
	return users, err
}

func (r *userRepository) Count() (int64, error) {
	var count int64
	err := r.db.Model(&models.User{}).Count(&count).Error
	return count, err
}

// SearchByUsername uses PostgreSQL trigram similarity for fuzzy search
func (r *userRepository) SearchByUsername(query string, limit int) ([]models.User, error) {
	var users []models.User

	// Use ILIKE for case-insensitive search with trigram index
	err := r.db.Where("username ILIKE ?", "%"+query+"%").
		Order("rating DESC").
		Limit(limit).
		Find(&users).Error

	return users, err
}

func (r *userRepository) GetTopUsers(limit int) ([]models.User, error) {
	var users []models.User
	err := r.db.Order("rating DESC, username ASC").
		Limit(limit).
		Find(&users).Error
	return users, err
}

// GetRandomUserID gets a random user ID for simulator
func (r *userRepository) GetRandomUserID() (uint, error) {
	var user models.User
	err := r.db.Order("RANDOM()").
		Select("id").
		First(&user).Error
	if err != nil {
		return 0, err
	}
	return user.ID, nil
}

// ScoreUpdateRepository handles score update history
type ScoreUpdateRepository interface {
	Create(update *models.ScoreUpdate) error
	GetByUserID(userID uint, limit int) ([]models.ScoreUpdate, error)
}

type scoreUpdateRepository struct {
	db *gorm.DB
}

func NewScoreUpdateRepository(db *gorm.DB) ScoreUpdateRepository {
	return &scoreUpdateRepository{db: db}
}

func (r *scoreUpdateRepository) Create(update *models.ScoreUpdate) error {
	return r.db.Create(update).Error
}

func (r *scoreUpdateRepository) GetByUserID(userID uint, limit int) ([]models.ScoreUpdate, error) {
	var updates []models.ScoreUpdate
	err := r.db.Where("user_id = ?", userID).
		Order("updated_at DESC").
		Limit(limit).
		Find(&updates).Error
	return updates, err
}
