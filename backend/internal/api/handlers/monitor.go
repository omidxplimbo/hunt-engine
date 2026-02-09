package handlers

import (
	"fmt"
	"runtime"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"github.com/omidxplimbo/hunt-engine/backend/internal/worker"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

type SystemStats struct {
	CPUUsage     float64 `json:"cpu_usage"`
	MemoryTotal  uint64  `json:"memory_total"`
	MemoryUsed   uint64  `json:"memory_used"`
	MemoryUsage  float64 `json:"memory_usage"`
	NumGoroutine int     `json:"num_goroutine"`
}

type ProcessInfo struct {
	TargetID   uint   `json:"target_id"`
	TargetName string `json:"target_name"`
	Command    string `json:"command"`
	Duration   string `json:"duration"`
	PID        int    `json:"pid"`
}

type MonitorResponse struct {
	Stats     SystemStats   `json:"stats"`
	Processes []ProcessInfo `json:"processes"`
}

// GetMonitorData handles the real-time monitoring request
func GetMonitorData(c *fiber.Ctx) error {
	// Only Admin check
	if !isAdmin(c) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Access denied"})
	}

	// 1. System Stats
	v, _ := mem.VirtualMemory()
	cStats, _ := cpu.Percent(0, false)
	cpuPercent := 0.0
	if len(cStats) > 0 {
		cpuPercent = cStats[0]
	}

	stats := SystemStats{
		CPUUsage:     cpuPercent,
		MemoryTotal:  v.Total,
		MemoryUsed:   v.Used,
		MemoryUsage:  v.UsedPercent,
		NumGoroutine: runtime.NumGoroutine(),
	}

	// 2. Running Processes
	rawTasks := worker.GetRunningTasks()
	processes := make([]ProcessInfo, 0, len(rawTasks))

	// Collect TargetIDs to fetch names
	targetIDs := make(map[uint]string)
	for _, t := range rawTasks {
		targetIDs[t.TargetID] = ""
	}

	// Batch fetch target names
	if len(targetIDs) > 0 {
		var targets []models.Target
		ids := make([]uint, 0, len(targetIDs))
		for id := range targetIDs {
			ids = append(ids, id)
		}
		database.DB.Select("id, root_domain").Where("id IN ?", ids).Find(&targets)
		for _, t := range targets {
			targetIDs[t.ID] = t.RootDomain
		}
	}

	for _, t := range rawTasks {
		name := targetIDs[t.TargetID]
		if name == "" {
			name = fmt.Sprintf("Target #%d", t.TargetID)
		}
		// Truncate command if too long
		cmd := t.Command
		if len(cmd) > 60 {
			cmd = cmd[:57] + "..."
		}

		processes = append(processes, ProcessInfo{
			TargetID:   t.TargetID,
			TargetName: name,
			Command:    cmd,
			Duration:   time.Since(t.StartTime).Round(time.Second).String(),
			PID:        t.PID,
		})
	}

	return c.JSON(fiber.Map{
		"status": "success",
		"data": MonitorResponse{
			Stats:     stats,
			Processes: processes,
		},
	})
}
