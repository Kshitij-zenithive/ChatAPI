package auth

import (
	"fmt"
	"time"

	`"github.com/golang-jwt/jwt/v5"`
	"crm-api/models"
	"github.com/golang-jwt/jwt/v4"
)

func GenerateJWT(user *models.User, authProvider string, expiryHours int, key []byte) (string, error) {
	expirationTime := time.Now().Add(time.Duration(expiryHours) * time.Hour).Unix()

	fmt.Println("username:", user.Name)
	fmt.Println("role:", user.Role)

	claims := jwt.MapClaims{
		"user_id":       user.ID.String(),
		"name":          user.Name,
		"role":          user.Role,
		"auth_provider": authProvider,
		"exp":           expirationTime,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString(key)
}
func GenerateJWTDemo(user *models.UserDemo, authProvider string, expiryHours int, key []byte) (string, error) {
	expirationTime := time.Now().Add(time.Duration(expiryHours) * time.Hour).Unix()

	fmt.Println("username:", user.Name)
	fmt.Println("role:", user.Role)

	claims := jwt.MapClaims{
		"user_id":       user.ID.String(),
		"name":          user.Name,
		"role":          user.Role,
		"auth_provider": authProvider,
		"exp":           expirationTime,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(key)
}
