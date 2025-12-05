package agent

import (
	"fmt"
	"strings"
	"time"
)

// Scheduler 定时调度器
type Scheduler struct {
	agent      *MealAgent
	lunchTime  string // "11:00"
	dinnerTime string // "17:00"
	stopCh     chan struct{}
	notifyCh   chan string // 推送通知的 channel
}

// NewScheduler 创建调度器
func NewScheduler(agent *MealAgent, lunch, dinner string) *Scheduler {
	return &Scheduler{
		agent:      agent,
		lunchTime:  lunch,
		dinnerTime: dinner,
		stopCh:     make(chan struct{}),
		notifyCh:   make(chan string, 10),
	}
}

// Start 启动定时任务
func (s *Scheduler) Start() {
	go s.run()
}

// Stop 停止定时任务
func (s *Scheduler) Stop() {
	close(s.stopCh)
}

// Notifications 获取通知 channel
func (s *Scheduler) Notifications() <-chan string {
	return s.notifyCh
}

func (s *Scheduler) run() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	// 每天清空临时排除
	lastDate := time.Now().Format("2006-01-02")

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			now := time.Now()
			currentTime := now.Format("15:04")
			currentDate := now.Format("2006-01-02")

			// 新的一天，清空临时排除
			if currentDate != lastDate {
				s.agent.cfg.ClearTempExclude()
				s.agent.Reset()
				lastDate = currentDate
			}

			// 检查是否到了提醒时间
			if currentTime == s.lunchTime {
				s.triggerRecommendation("lunch")
			} else if currentTime == s.dinnerTime {
				s.triggerRecommendation("dinner")
			}
		}
	}
}

func (s *Scheduler) triggerRecommendation(mealType string) {
	s.agent.Reset() // 重置对话上下文

	recommendation, err := s.agent.GetRecommendation(mealType)
	if err != nil {
		s.notifyCh <- fmt.Sprintf("获取推荐失败: %v", err)
		return
	}

	mealName := map[string]string{"lunch": "午餐", "dinner": "晚餐"}[mealType]
	notification := fmt.Sprintf("\n🍽️  %s时间到！\n\n%s", mealName, recommendation)
	s.notifyCh <- notification
}

// ManualTrigger 手动触发推荐
func (s *Scheduler) ManualTrigger() {
	hour := time.Now().Hour()
	mealType := "lunch"
	if hour >= 15 {
		mealType = "dinner"
	}
	s.triggerRecommendation(mealType)
}

// ParseScheduleTime 解析时间字符串
func ParseScheduleTime(timeStr string) (hour, minute int, err error) {
	parts := strings.Split(timeStr, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid time format: %s", timeStr)
	}

	_, err = fmt.Sscanf(timeStr, "%d:%d", &hour, &minute)
	return
}