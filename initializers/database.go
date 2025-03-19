package initializers

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"crm-api/models"
)

var DB *gorm.DB

func ConnectToDatabase() {
	if os.Getenv("RENDER") == "" {
		err := godotenv.Load()
		if err != nil {
			log.Println("No .env file found, using system environment variables")
		}
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("DB_URL")
		if dsn == "" {
			log.Println("No DATABASE_URL or DB_URL found, using default")
			dsn = "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"
		}
	}

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("failed to connect to database")
	}
	if DB == nil {
		fmt.Println("DB is nil")
	}

	err = DB.AutoMigrate(
		&models.User{},
		&models.Client{},
		&models.Chat{},
		&models.Message{},
		&models.Email{},
		&models.TimelineEvent{},
	)
	if err != nil {
		log.Fatalf("Failed to migrate database schema: %v", err)
	}
}
