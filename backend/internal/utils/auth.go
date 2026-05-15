package utils

import (
	"errors"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// کلید مخفی برای امضای توکن‌ها (در پروداکشن باید از ENV خوانده شود)
var jwtSecret = getJwtSecret()

func getJwtSecret() []byte {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		// Fallback for development, but should panic in production
		return []byte("super_secret_hunt_key_change_me")
	}
	return []byte(secret)
}

// HashPassword پسورد را هش می‌کند
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

// CheckPasswordHash پسورد وارد شده را با هش مقایسه می‌کند
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// GenerateJWT یک توکن برای کاربر می‌سازد
func GenerateJWT(userID uint, username, role string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":  userID,
		"username": username,
		"role":     role,
		"exp":      time.Now().Add(time.Hour * 72).Unix(), // اعتبار: ۳ روز
	})

	tokenString, err := token.SignedString(jwtSecret)
	return tokenString, err
}

// ValidateToken توکن را بررسی و اطلاعات کاربر را برمی‌گرداند
func ValidateToken(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}
