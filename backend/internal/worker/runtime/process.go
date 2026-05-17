package runtime

import (
	"sort"
	"sync"
	"time"
)

// TaskInfo represents a currently running external tool process.
type TaskInfo struct {
	TargetID  uint      `json:"target_id"`
	Command   string    `json:"command"`
	StartTime time.Time `json:"start_time"`
	PID       int       `json:"pid"`
}

var runningProcesses sync.Map

// GetRunningTasks returns active external tool processes, newest first.
func GetRunningTasks() []TaskInfo {
	var tasks []TaskInfo
	runningProcesses.Range(func(key, value interface{}) bool {
		if task, ok := value.(TaskInfo); ok {
			tasks = append(tasks, task)
		}
		return true
	})

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].StartTime.After(tasks[j].StartTime)
	})

	return tasks
}

func storeRunningProcess(key string, task TaskInfo) {
	runningProcesses.Store(key, task)
}

func deleteRunningProcess(key string) {
	runningProcesses.Delete(key)
}
