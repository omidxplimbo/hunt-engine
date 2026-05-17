package persistence

import (
	"encoding/json"
	"log"
	"reflect"
	"sort"
	"strconv"
	"time"

	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/cache"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/telegram"
	workerhelpers "github.com/omidxplimbo/hunt-engine/backend/internal/worker/helpers"
	workertypes "github.com/omidxplimbo/hunt-engine/backend/internal/worker/types"
	"gorm.io/gorm"
)

type Context struct {
	LogChange func(tx *gorm.DB, assetID uint, field, oldVal, newVal string) error
}

func (ctx Context) logChange(tx *gorm.DB, assetID uint, field, oldVal, newVal string) error {
	if ctx.LogChange == nil {
		return nil
	}
	return ctx.LogChange(tx, assetID, field, oldVal, newVal)
}

func UpdateAssetsWithDiff(ctx Context, targetID uint, results map[string]workertypes.HTTPXResult) {
	var targetName string
	var target models.Target
	isFirstRun := false
	if err := database.DB.First(&target, targetID).Error; err == nil {
		targetName = target.RootDomain
		isFirstRun = target.ScanCount == 0
	}

	countUpdated := 0
	countChanges := 0

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		for hostInput, result := range results {
			var asset models.Asset
			if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("value = ? AND target_id = ?", hostInput, targetID).First(&asset).Error; err != nil {
				if err == gorm.ErrRecordNotFound {
					// این حالت نادر است چون Phase 2 فقط assets موجود در دیتابیس را پردازش می‌کند
					// اما اگر asset جدیدی پیدا شد و لایو است، فقط در صورتی نوتیف می‌فرستیم که اولین اجرا نباشد
					asset = models.Asset{
						TargetID: targetID, Value: hostInput, Type: "subdomain", IsNew: true, IsLive: true,
						Technologies: "[]", HostIP: "[]", RawHttpx: "{}", DnsxIP: "[]",
					}
					if err := tx.Create(&asset).Error; err != nil {
						return err
					}
					// فقط برای assets جدید که لایو هستند و اولین اجرا نیست، نوتیف می‌فرستیم
					if !isFirstRun && result.StatusCode > 0 {
						telegram.SendNewAssetAlert(targetName, hostInput)
					}
				} else {
					continue
				}
			}

			isFirstProbing := asset.RawHttpx == "" || asset.RawHttpx == "{}"
			hasChanged := false
			now := time.Now()

			checkAndNotify := func(field, oldVal, newVal string) error {
				if oldVal != newVal {
					if err := ctx.logChange(tx, asset.ID, field, oldVal, newVal); err != nil {
						return err
					}
					if !isFirstProbing {
						telegram.SendChangeAlert(targetName, hostInput, field, oldVal, newVal)
					}
					hasChanged = true
				}
				return nil
			}

			if err := checkAndNotify("status_code", strconv.Itoa(asset.StatusCode), strconv.Itoa(result.StatusCode)); err != nil {
				return err
			}
			oldTitle := workerhelpers.ShortenString(asset.Title, 50)
			newTitle := workerhelpers.ShortenString(result.Title, 50)
			if err := checkAndNotify("title", oldTitle, newTitle); err != nil {
				return err
			}
			if err := checkAndNotify("web_server", asset.WebServer, result.WebServer); err != nil {
				return err
			}

			newTechJSON := marshalJSONOrDefault(result.Technologies, "[]")
			newIPsJSON := marshalJSONOrDefault(result.A, "[]")

			if !areJSONArraysEqual(asset.Technologies, newTechJSON) {
				if err := checkAndNotify("technologies", asset.Technologies, newTechJSON); err != nil {
					return err
				}
			}
			if !areJSONArraysEqual(asset.HostIP, newIPsJSON) {
				if err := checkAndNotify("host_ip", asset.HostIP, newIPsJSON); err != nil {
					return err
				}
			}

			if hasChanged {
				asset.LastChangeAt = &now
				countChanges++
			}

			asset.FinalURL = result.URL
			asset.StatusCode = result.StatusCode
			asset.Title = result.Title
			asset.ContentLength = result.ContentLength
			asset.HostIP = newIPsJSON
			asset.WebServer = result.WebServer
			asset.CDNName = result.CDNName
			asset.Technologies = newTechJSON
			asset.BodyHash = result.BodyHash
			asset.HeaderHash = result.HeaderHash
			asset.RawHttpx = result.RawJSON
			asset.ResponseTimeMs = workerhelpers.ParseResponseTime(result.ResponseTime)

			if err := tx.Save(&asset).Error; err != nil {
				return err
			}
			countUpdated++
		}
		return nil
	})

	if err != nil {
		log.Printf("❌ Transaction failed: %v\n", err)
	} else {
		// کش را می‌سوزانیم
		if countUpdated > 0 {
			cache.IncrementTargetVersion(targetID)
		}
		log.Printf("✅ Smart Update Complete. Updated: %d, Changes Detected: %d.\n", countUpdated, countChanges)
	}
}

func areJSONArraysEqual(json1, json2 string) bool {
	var arr1, arr2 []string
	_ = json.Unmarshal([]byte(json1), &arr1)
	_ = json.Unmarshal([]byte(json2), &arr2)

	if len(arr1) != len(arr2) {
		return false
	}

	sort.Strings(arr1)
	sort.Strings(arr2)

	return reflect.DeepEqual(arr1, arr2)
}

func marshalJSONOrDefault(v interface{}, defaultVal string) string {
	if reflect.ValueOf(v).Kind() == reflect.Slice && reflect.ValueOf(v).Len() == 0 {
		return defaultVal
	}

	jsonBytes, err := json.Marshal(v)
	if err != nil || string(jsonBytes) == "null" || string(jsonBytes) == "" {
		return defaultVal
	}

	return string(jsonBytes)
}
