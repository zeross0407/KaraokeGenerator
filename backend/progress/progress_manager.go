package progress

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// ProgressMessage là struct để gửi cập nhật tiến trình
type ProgressMessage struct {
	Type            string  `json:"type"`
	Status          string  `json:"status"`
	Message         string  `json:"message"`
	Percentage      float64 `json:"percentage"`
	CurrentStep     string  `json:"current_step"`
	EstimatedTimeLeft string  `json:"estimated_time_left"`
	SessionID       string  `json:"sessionId"`
	Timestamp       int64   `json:"timestamp"`
}

// ProgressManager quản lý tiến trình cho mỗi phiên (session)
type ProgressManager struct {
	// Dùng map để lưu trữ tiến trình theo sessionID
	sessions     map[string]*ProgressMessage
	// Đảm bảo thread-safe khi nhiều goroutine cùng truy cập
	mutex       sync.RWMutex
}

// Global instance của ProgressManager
var progressManager *ProgressManager
var once sync.Once

// GetProgressManager trả về singleton instance của ProgressManager
func GetProgressManager() *ProgressManager {
	once.Do(func() {
		progressManager = &ProgressManager{
			sessions: make(map[string]*ProgressMessage),
		}
	})
	return progressManager
}

// UpdateProgress cập nhật tiến trình xử lý với phần trăm và message
// Đây là hàm chính bạn sẽ gọi từ bất kỳ đâu để cập nhật tiến trình
func UpdateProgress(sessionID string, percentage float64, message string, currentStep string) {
	manager := GetProgressManager()
	
	// Tính toán thời gian còn lại (đơn giản hóa)
	var estimatedTimeLeft string
	if percentage < 100 {
		// Giả sử còn 1 phút cho mỗi 10% còn lại
		remainingMinutes := (100 - percentage) / 10
		if remainingMinutes < 1 {
			estimatedTimeLeft = "Less than a minute"
		} else {
			estimatedTimeLeft = fmt.Sprintf("About %.0f minutes", remainingMinutes)
		}
	} else {
		estimatedTimeLeft = "Completed"
	}
	
	// Xác định trạng thái dựa trên phần trăm
	status := "processing"
	if percentage <= 0 {
		status = "start"
	} else if percentage >= 100 {
		status = "complete"
	}
	
	// Tạo thông điệp tiến trình
	progressMsg := &ProgressMessage{
		Type:            "process_update",
		Status:          status,
		Message:         message,
		Percentage:      percentage,
		CurrentStep:     currentStep,
		EstimatedTimeLeft: estimatedTimeLeft,
		SessionID:       sessionID,
		Timestamp:       time.Now().Unix(),
	}
	
	// Lưu vào bộ nhớ
	manager.mutex.Lock()
	manager.sessions[sessionID] = progressMsg
	manager.mutex.Unlock()
	
	// In ra console để theo dõi
	progressJSON, _ := json.Marshal(progressMsg)
	fmt.Printf("[PROGRESS] %s\n", string(progressJSON))
}

// Hàm lấy tiến trình hiện tại của một session
func GetProgress(sessionID string) *ProgressMessage {
	manager := GetProgressManager()
	
	manager.mutex.RLock()
	defer manager.mutex.RUnlock()
	
	return manager.sessions[sessionID]
}

// Hàm xóa tiến trình khi hoàn thành
func ClearProgress(sessionID string) {
	manager := GetProgressManager()
	
	manager.mutex.Lock()
	delete(manager.sessions, sessionID)
	manager.mutex.Unlock()
} 