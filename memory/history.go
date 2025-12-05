package memory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// MealRecord 用餐记录
type MealRecord struct {
	Date       string `json:"date"`       // 日期 2024-01-15
	MealType   string `json:"meal_type"`  // lunch / dinner
	Restaurant string `json:"restaurant"` // 餐厅名称
	Category   string `json:"category"`   // 菜系类型
	Rating     int    `json:"rating"`     // 用户评分 1-5（可选）
	Note       string `json:"note"`       // 备注
}

// History 历史记录管理
type History struct {
	Records  []MealRecord `json:"records"`
	filePath string
}

// NewHistory 创建或加载历史记录
func NewHistory(dataDir string) (*History, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}

	filePath := filepath.Join(dataDir, "history.json")
	h := &History{
		Records:  []MealRecord{},
		filePath: filePath,
	}

	// 尝试加载已有记录
	data, err := os.ReadFile(filePath)
	if err == nil {
		json.Unmarshal(data, &h.Records)
	}

	return h, nil
}

// Add 添加用餐记录
func (h *History) Add(record MealRecord) error {
	if record.Date == "" {
		record.Date = time.Now().Format("2006-01-02")
	}
	h.Records = append(h.Records, record)
	return h.save()
}

// GetRecent 获取最近 N 天的记录
func (h *History) GetRecent(days int) []MealRecord {
	cutoff := time.Now().AddDate(0, 0, -days).Format("2006-01-02")
	recent := []MealRecord{}

	for _, r := range h.Records {
		if r.Date >= cutoff {
			recent = append(recent, r)
		}
	}
	return recent
}

// GetToday 获取今天的记录
func (h *History) GetToday() []MealRecord {
	today := time.Now().Format("2006-01-02")
	todayRecords := []MealRecord{}

	for _, r := range h.Records {
		if r.Date == today {
			todayRecords = append(todayRecords, r)
		}
	}
	return todayRecords
}

// GetRecentRestaurants 获取最近吃过的餐厅名称（用于避免重复推荐）
func (h *History) GetRecentRestaurants(days int) []string {
	recent := h.GetRecent(days)
	restaurants := make([]string, 0, len(recent))

	seen := make(map[string]bool)
	for _, r := range recent {
		if !seen[r.Restaurant] {
			seen[r.Restaurant] = true
			restaurants = append(restaurants, r.Restaurant)
		}
	}
	return restaurants
}

// GetRecentPenalty 获取餐厅的历史惩罚权重
// 返回应该减去的权重值：
//   - 今天吃过：-80
//   - 昨天吃过：-50
//   - 2天前吃过：-30
//   - 3天前吃过：-15
//   - 更早或没吃过：0
func (h *History) GetRecentPenalty(restaurantName string) int {
	today := time.Now()

	for _, r := range h.Records {
		if r.Restaurant != restaurantName {
			continue
		}

		recordDate, err := time.Parse("2006-01-02", r.Date)
		if err != nil {
			continue
		}

		daysDiff := int(today.Sub(recordDate).Hours() / 24)

		switch {
		case daysDiff == 0:
			return -80 // 今天吃过
		case daysDiff == 1:
			return -50 // 昨天吃过
		case daysDiff == 2:
			return -30 // 2天前
		case daysDiff == 3:
			return -15 // 3天前
		}
	}

	return 0 // 没有近期记录
}

// GetAllPenalties 获取所有餐厅的惩罚权重（批量查询更高效）
func (h *History) GetAllPenalties() map[string]int {
	penalties := make(map[string]int)
	today := time.Now()

	for _, r := range h.Records {
		recordDate, err := time.Parse("2006-01-02", r.Date)
		if err != nil {
			continue
		}

		daysDiff := int(today.Sub(recordDate).Hours() / 24)
		if daysDiff > 3 {
			continue // 超过3天不计算惩罚
		}

		var penalty int
		switch {
		case daysDiff == 0:
			penalty = -80
		case daysDiff == 1:
			penalty = -50
		case daysDiff == 2:
			penalty = -30
		case daysDiff == 3:
			penalty = -15
		}

		// 取最大惩罚（最近一次）
		if existing, ok := penalties[r.Restaurant]; !ok || penalty < existing {
			penalties[r.Restaurant] = penalty
		}
	}

	return penalties
}

// GetFrequent 获取吃得最频繁的餐厅
func (h *History) GetFrequent(topN int) []string {
	count := make(map[string]int)
	for _, r := range h.Records {
		count[r.Restaurant]++
	}

	// 简单排序
	type kv struct {
		Name  string
		Count int
	}
	var sorted []kv
	for k, v := range count {
		sorted = append(sorted, kv{k, v})
	}
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].Count > sorted[i].Count {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	result := make([]string, 0, topN)
	for i := 0; i < topN && i < len(sorted); i++ {
		result = append(result, sorted[i].Name)
	}
	return result
}

// save 保存到文件
func (h *History) save() error {
	data, err := json.MarshalIndent(h.Records, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(h.filePath, data, 0644)
}

// Summary 生成历史摘要（给 LLM 用）
func (h *History) Summary() string {
	recent := h.GetRecent(7)
	if len(recent) == 0 {
		return "暂无用餐历史记录"
	}

	summary := "最近7天用餐记录：\n"
	for _, r := range recent {
		summary += "- " + r.Date + " " + r.MealType + ": " + r.Restaurant
		if r.Category != "" {
			summary += "（" + r.Category + "）"
		}
		summary += "\n"
	}
	return summary
}