package dto

type DashboardStats struct {
	TotalTargets int64 `json:"total_targets"`
	TotalAssets  int64 `json:"total_assets"`
	LiveAssets   int64 `json:"live_assets"`
	FreshAssets  int64 `json:"fresh_assets"` // کشف شده در ۲۴ ساعت گذشته

	// برای نمودار دایره‌ای (Pie Chart)
	AssetsByStatus []CountItem `json:"assets_by_status"`

	// برای نمودار میله‌ای (Bar Chart)
	TopTechnologies []CountItem `json:"top_technologies"`
}

type CountItem struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}
