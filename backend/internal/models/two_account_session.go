package models

import (
	"time"
	"gorm.io/gorm"
)

// TwoAccountStatus نشان‌دهنده وضعیت هوشمند سشن تست است
type TwoAccountStatus string

const (
        TwoAccountStatusRunning    = "running"
        TwoAccountStatusCompleted  = "completed"
	StatusTwoAccountInitializing TwoAccountStatus = "initializing" // در حال جمع‌آوری اطلاعات اولیه
	StatusTwoAccountExploring    TwoAccountStatus = "exploring"    // در حال کاوش نقاط مشکوک (Recon)
	StatusTwoAccountAttacking    TwoAccountStatus = "attacking"    // در حال اجرای پیلودهای هدفمند
	StatusTwoAccountVerifying    TwoAccountStatus = "verifying"    // در حال راستی‌آزمایی باگ کشف شده
	StatusTwoAccountCompleted    TwoAccountStatus = "completed"    // تست تمام شد (با موفقیت یا شکست)
	StatusTwoAccountFailed       TwoAccountStatus = "failed"       // خطا در اجرا
)

// TwoAccountSession نماینده یک جلسه تست IDOR/BOLA با دو حساب همزمان است.
// این مدل "حافظه کوتاه‌مدت" ایجنت برای این فیچر خاص است.
type TwoAccountSession struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	TargetID  uint           `gorm:"not null;index" json:"target_id"`
	OwnerID   uint           `gorm:"not null;index" json:"owner_id"` // کاربری که تست را شروع کرده

	// لینک به کانتکست‌های احراز هویت (از فیچر قبلی)
	// ایجنت هوشمند باید بداند کدام کوکی/توکن متعلق به کدام حساب است
	AuthContextID_A uint   `gorm:"not null" json:"auth_context_id_a"` // حساب قربانی (Victim)
	AuthContextID_B uint   `gorm:"not null" json:"auth_context_id_b"` // حساب مهاجم (Attacker)
	
	// فیلدهای کمکی برای دسترسی سریع (Denormalization برای پرفورمنس)
	ContextA_Name string `gorm:"size:100" json:"context_a_name"`
	ContextB_Name string `gorm:"size:100" json:"context_b_name"`

	// وضعیت هوشمند سشن
	Status TwoAccountStatus `gorm:"type:varchar(20);default:'initializing'" json:"status"`

	// استراتژی فعلی ایجنت
	// مثال: "parameter_fuzzing", "sequence_manipulation", "mass_assignment"
	CurrentStrategy string `gorm:"size:100" json:"current_strategy"`

	// متریک‌های زنده برای تصمیم‌گیری ایجنت
	RequestsSent    int `gorm:"default:0" json:"requests_sent"`
	DifferencesFound int `gorm:"default:0" json:"differences_found"`
	PotentialBugs   int `gorm:"default:0" json:"potential_bugs"`

	// آخرین یافته مهم (برای نمایش در چت‌بات)
	LastSignificantFinding string `gorm:"type:text" json:"last_significant_findings"`

	// زمان‌بندی
	StartedAt time.Time      `json:"started_at"`
	EndedAt   *time.Time     `json:"ended_at"`
	
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (TwoAccountSession) TableName() string {
	return "two_account_sessions"
}
