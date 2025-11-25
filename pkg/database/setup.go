package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/cclli4/realtime-message-app/app/models"
	"github.com/cclli4/realtime-message-app/pkg/env"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var (
	MongoClient *mongo.Client
)

// ======================================
// MySQL setup
// ======================================
func SetupDatabase() {
	var err error

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		env.GetEnv("DB_USER", ""),
		env.GetEnv("DB_PASSWORD", ""),
		env.GetEnv("DB_HOST", ""),
		env.GetEnv("DB_PORT", ""),
		env.GetEnv("DB_NAME", ""),
	)

	maxAttempts := 5
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
		if err == nil {
			break
		}
		log.Printf("[WARN] failed to connect MySQL (attempt %d/%d): %v", attempt, maxAttempts, err)
		time.Sleep(time.Duration(attempt) * time.Second)
	}
	if err != nil {
		log.Fatalf("[ERROR] failed to connect MySQL after retries: %v", err)
	}

	if err := DB.AutoMigrate(&models.User{}, &models.UserSession{}); err != nil {
		log.Fatalf("[ERROR] failed to migrate database: %v", err)
	}

	log.Println("[OK] Connected to MySQL & migrated")
}

// ======================================
// MongoDB setup (optional, no panic)
// ======================================
func SetupMongoDB() {
	uri := env.GetEnv("MONGO_URI", "")
	if uri == "" {
		log.Println("[WARN] MONGO_URI is empty, skipping MongoDB initialization")
		return
	}

	var (
		client *mongo.Client
		err    error
	)

	for attempt := 1; attempt <= 5; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		client, err = mongo.Connect(ctx, options.Client().ApplyURI(uri))
		if err == nil {
			if pingErr := client.Ping(ctx, nil); pingErr != nil {
				err = pingErr
			}
		}
		cancel()

		if err == nil {
			break
		}

		log.Printf("[WARN] error connecting to MongoDB (attempt %d/5): %v", attempt, err)
		time.Sleep(time.Duration(attempt) * time.Second)
	}

	if err != nil {
		log.Fatalf("[ERROR] failed to connect to MongoDB after retries: %v", err)
	}

	log.Println("[OK] Connected to MongoDB")
	MongoClient = client

	// Initialize MongoDB collection for messages
	dbName := env.GetEnv("MONGO_DB_NAME", "message_app")
	MongoDB = client.Database(dbName).Collection("messages")
	log.Printf("[OK] MongoDB collection initialized: %s.messages", dbName)
}
