package service

import (
	"context"
	"crypto/sha256"
	"errors"
	"time"
	
	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/crypto"
	"gorm.io/gorm"
)

type AuthContextService struct {
	db *gorm.DB
	cryptoKey []byte
}

func NewAuthContextService(db *gorm.DB, jwtSecret []byte) *AuthContextService {
	// اطمینان از اینکه کلید رمزنگاری دقیقاً ۳۲ بایت است
	key := sha256.Sum256(jwtSecret)
	return &AuthContextService{
		db: db,
		cryptoKey: key[:],
	}
}

// CreateAuth creates a new auth context with encrypted value
func (s *AuthContextService) CreateAuth(ctx context.Context, req *CreateAuthContextRequest) (*models.AuthContext, error) {
	if len(s.cryptoKey) != 32 {
		return nil, errors.New("invalid encryption key length")
	}

	encryptedValue, err := crypto.Encrypt(req.Value, s.cryptoKey)
	if err != nil {
		return nil, err
	}

	context := models.AuthContext{
		TargetID:    req.TargetID,
		UserID:      req.UserID,
		Name:        req.Name,
		ContextType: models.AuthContextType(req.ContextType),
		KeyName:     req.KeyName,
		Value:       encryptedValue,
		Domain:      req.Domain,
		Path:        req.Path,
		ExpiresAt:   req.ExpiresAt,
		Description: req.Description,
		IsActive:    true,
	}

	if err := s.db.Create(&context).Error; err != nil {
		return nil, err
	}

	context.Value = req.Value // Return original value in response
	return &context, nil
}

// GetByTarget retrieves all active auth contexts for a target
func (s *AuthContextService) GetByTarget(ctx context.Context, targetID, userID uint) ([]models.AuthContext, error) {
	var contexts []models.AuthContext
	err := s.db.Where("target_id = ? AND user_id = ? AND is_active = ?", targetID, userID, true).Find(&contexts).Error
	if err != nil {
		return nil, err
	}

	for i := range contexts {
		decrypted, err := crypto.Decrypt(contexts[i].Value, s.cryptoKey)
		if err == nil {
			contexts[i].Value = decrypted
		} else {
			contexts[i].Value = "***ENCRYPTED***"
		}
	}

	return contexts, nil
}

// Delete deletes an auth context
func (s *AuthContextService) Delete(ctx context.Context, id, userID uint) error {
	return s.db.Where("id = ? AND user_id = ?", id, userID).Delete(&models.AuthContext{}).Error
}

type CreateAuthContextRequest struct {
	TargetID    uint       `json:"target_id" binding:"required"`
	UserID      uint       `json:"user_id"`
	Name        string     `json:"name" binding:"required"`
	ContextType string     `json:"context_type" binding:"required"`
	KeyName     string     `json:"key_name"`
	Value       string     `json:"value" binding:"required"`
	Domain      string     `json:"domain"`
	Path        string     `json:"path"`
	ExpiresAt   *time.Time `json:"expires_at"`
	Description string     `json:"description"`
}
