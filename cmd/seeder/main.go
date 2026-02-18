package main

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"time"

	"github.com/SSujoy-Samanta/leaderboard-backend/internal/config"
	"github.com/SSujoy-Samanta/leaderboard-backend/internal/database"
	"github.com/SSujoy-Samanta/leaderboard-backend/internal/models"
	"github.com/SSujoy-Samanta/leaderboard-backend/internal/repository"
)

func main() {
	log.Println("🌱 Starting Complete Database Seeder (PostgreSQL + Redis)...")

	// Load configuration
	cfg := config.LoadConfig()

	// Connect to PostgreSQL
	db, err := database.ConnectPostgres(&cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer database.CloseDB()

	// Run migrations
	if err := database.AutoMigrate(db); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Connect to Redis
	redisClient, err := database.ConnectRedis(&cfg.Redis)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer database.CloseRedis()

	// Initialize repositories
	userRepo := repository.NewUserRepository(db)
	leaderboardRepo := repository.NewLeaderboardRepository(redisClient)

	// Check if data already exists
	count, _ := userRepo.Count()
	if count > 0 {
		log.Printf("⚠️  Database already contains %d users", count)
		log.Println("Do you want to continue and add more users? (y/n)")
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			log.Println("Seeding cancelled")
			return
		}
	}

	// Check Redis
	redisSize, _ := leaderboardRepo.GetLeaderboardSize()
	if redisSize > 0 {
		log.Printf("⚠️  Redis already contains %d users", redisSize)
		log.Println("Do you want to clear and resync? (y/n)")
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			log.Println("Seeding cancelled")
			return
		}
	}

	// Configuration
	numUsers := 10000
	log.Printf("Creating %d users...\n", numUsers)

	// Initialize random seed
	rand.Seed(time.Now().UnixNano())

	// Common name prefixes for realistic usernames
	prefixes := []string{
		"pro", "ninja", "gamer", "killer", "shadow", "master", "legend",
		"dark", "fire", "ice", "thunder", "storm", "dragon", "phoenix",
		"rahul", "amit", "priya", "rohan", "sneha", "vikram", "ananya",
	}

	suffixes := []string{
		"x", "king", "queen", "lord", "god", "pro", "elite", "prime",
		"123", "007", "gamer", "player", "master", "legend", "warrior",
	}

	// STEP 1: Seed PostgreSQL
	log.Println("\n📊 STEP 1: Seeding PostgreSQL...")
	log.Println("─────────────────────────────────")

	batchSize := 500
	totalBatches := numUsers / batchSize
	startTime := time.Now()

	for batch := 0; batch < totalBatches; batch++ {
		users := make([]models.User, 0, batchSize)

		for i := 0; i < batchSize; i++ {
			userNum := batch*batchSize + i + 1

			// Generate UNIQUE username (always include userNum to ensure uniqueness)
			var username string
			randChoice := rand.Float64()

			if randChoice < 0.3 {
				// 30% chance: prefix_suffix_NUM
				username = fmt.Sprintf("%s_%s_%d",
					prefixes[rand.Intn(len(prefixes))],
					suffixes[rand.Intn(len(suffixes))],
					userNum)
			} else if randChoice < 0.6 {
				// 30% chance: prefix_NUM
				username = fmt.Sprintf("%s_%d",
					prefixes[rand.Intn(len(prefixes))],
					userNum)
			} else {
				// 40% chance: user_NUM format
				username = fmt.Sprintf("user_%d", userNum)
			}

			// Generate rating with bell curve distribution
			rating := generateBellCurveRating()

			users = append(users, models.User{
				Username: username,
				Rating:   rating,
			})
		}

		// Insert to PostgreSQL
		if err := db.Create(&users).Error; err != nil {
			log.Fatalf("Failed to insert users batch %d: %v", batch+1, err)
		}

		// Progress
		progress := float64(batch+1) / float64(totalBatches) * 100
		log.Printf("  ✅ Batch %d/%d completed (%d users) - %.1f%%",
			batch+1, totalBatches, (batch+1)*batchSize, progress)
	}

	pgElapsed := time.Since(startTime)
	totalUsers, _ := userRepo.Count()

	log.Printf("\n✅ PostgreSQL seeding completed!")
	log.Printf("   📊 Total users: %d", totalUsers)
	log.Printf("   ⏱️  Time: %v\n", pgElapsed)

	// STEP 2: Sync to Redis
	log.Println("\n🔄 STEP 2: Syncing to Redis...")
	log.Println("─────────────────────────────────")

	syncStart := time.Now()
	offset := 0
	totalSynced := 0
	syncBatchSize := 500

	for {
		// Fetch users from PostgreSQL
		users, err := userRepo.GetAll(syncBatchSize, offset)
		if err != nil {
			log.Fatalf("Failed to fetch users: %v", err)
		}

		if len(users) == 0 {
			break
		}

		// Sync to Redis
		for _, user := range users {
			// Add to leaderboard (1 Redis operation)
			if err := leaderboardRepo.AddUser(user.ID, user.Rating); err != nil {
				log.Printf("  ⚠️  Failed to add user %d to leaderboard: %v", user.ID, err)
				continue
			}

			// Cache user data (1 Redis operation)
			if err := leaderboardRepo.CacheUser(&user); err != nil {
				log.Printf("  ⚠️  Failed to cache user %d: %v", user.ID, err)
			}

			// NO username indexing (uses PostgreSQL for search)
		}

		totalSynced += len(users)
		progress := float64(totalSynced) / float64(totalUsers) * 100
		log.Printf("  📊 Synced %d/%d users (%.1f%%)", totalSynced, totalUsers, progress)

		offset += syncBatchSize

		// Break if we got less than batch size
		if len(users) < syncBatchSize {
			break
		}
	}

	syncElapsed := time.Since(syncStart)
	leaderboardSize, _ := leaderboardRepo.GetLeaderboardSize()

	log.Printf("\n✅ Redis sync completed!")
	log.Printf("   🏆 Leaderboard size: %d", leaderboardSize)
	log.Printf("   ⏱️  Time: %v\n", syncElapsed)

	// Summary
	totalTime := time.Since(startTime)
	log.Println("\n═══════════════════════════════════")
	log.Println("🎉 SEEDING COMPLETE!")
	log.Println("═══════════════════════════════════")
	log.Printf("📊 PostgreSQL users:  %d", totalUsers)
	log.Printf("🏆 Redis leaderboard: %d", leaderboardSize)
	log.Printf("⏱️  Total time:       %v", totalTime)
	log.Printf("   ├─ PostgreSQL:     %v", pgElapsed)
	log.Printf("   └─ Redis sync:     %v", syncElapsed)
	log.Println("\n🔑 Redis Keys Created:")
	log.Println("   ├─ leaderboard:global    : 1 sorted set")
	log.Printf("   └─ user:cache:*          : %d hashes\n", totalUsers)
	log.Printf("   📦 Total Redis keys      : %d\n", totalUsers+1)
	log.Println("\n🚀 Start server with: go run cmd/server/main.go")
}

// generateBellCurveRating generates rating with normal distribution
func generateBellCurveRating() int {
	mean := 2500.0
	stdDev := 800.0

	// Box-Muller transform for normal distribution
	u1 := rand.Float64()
	u2 := rand.Float64()

	z := math.Sqrt(-2.0*math.Log(u1)) * math.Cos(2.0*math.Pi*u2)
	rating := int(mean + stdDev*z)

	// Clamp to valid range
	if rating < 100 {
		rating = 100
	}
	if rating > 5000 {
		rating = 5000
	}

	return rating
}
