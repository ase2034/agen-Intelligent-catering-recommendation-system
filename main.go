package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"meal-agent/agent"
	"meal-agent/config"
	"meal-agent/memory"
	"meal-agent/preference"
)

func main() {
	// 命令行参数
	configPath := flag.String("config", "config.yaml", "配置文件路径")
	prefPath := flag.String("pref", "restaurants.yaml", "餐厅偏好配置路径")
	dataDir := flag.String("data", "./data", "数据目录路径")
	mode := flag.String("mode", "chat", "运行模式: chat(交互) / daemon(后台定时)")
	flag.Parse()

	// 加载配置
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Printf("加载配置失败: %v\n", err)
		fmt.Println("请复制 config.example.yaml 为 config.yaml 并填写配置")
		os.Exit(1)
	}

	// 初始化历史记录
	history, err := memory.NewHistory(*dataDir)
	if err != nil {
		fmt.Printf("初始化历史记录失败: %v\n", err)
		os.Exit(1)
	}

	// 加载餐厅偏好配置（可选）
	pref, err := preference.Load(*prefPath)
	if err != nil {
		fmt.Printf("加载偏好配置失败: %v（将使用默认权重）\n", err)
		pref = nil
	}

	// 创建 Agent
	mealAgent := agent.NewMealAgent(cfg, history, pref)

	switch *mode {
	case "chat":
		runChatMode(mealAgent)
	case "daemon":
		runDaemonMode(mealAgent, cfg)
	default:
		fmt.Printf("未知模式: %s\n", *mode)
		os.Exit(1)
	}
}

// runChatMode 交互模式
func runChatMode(mealAgent *agent.MealAgent) {
	printWelcome()

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("\n你: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			break
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		// 处理特殊命令
		switch strings.ToLower(input) {
		case "quit", "exit", "q", "退出":
			fmt.Println("\n再见，祝用餐愉快！🍽️")
			return
		case "help", "帮助", "h":
			printHelp()
			continue
		case "推荐", "recommend", "r":
			handleRecommend(mealAgent)
			continue
		case "reset", "重置":
			mealAgent.Reset()
			fmt.Println("\n助手: 已重置对话，有什么可以帮你的？")
			continue
		case "history", "历史":
			handleHistory(mealAgent)
			continue
		}

		// 检查是否是记录命令
		if strings.HasPrefix(input, "记录 ") || strings.HasPrefix(input, "record ") {
			handleRecord(mealAgent, input)
			continue
		}

		// 普通对话
		response, err := mealAgent.Chat(input)
		if err != nil {
			fmt.Printf("\n助手: 抱歉，出错了: %v\n", err)
			continue
		}

		fmt.Printf("\n助手: %s\n", response)
	}
}

// runDaemonMode 后台定时模式
func runDaemonMode(mealAgent *agent.MealAgent, cfg *config.Config) {
	fmt.Println("🍽️  饮食推荐 Agent 已启动（后台模式）")
	fmt.Printf("午餐提醒时间: %s\n", cfg.Schedule.Lunch)
	fmt.Printf("晚餐提醒时间: %s\n", cfg.Schedule.Dinner)
	fmt.Println("按 Ctrl+C 退出")

	scheduler := agent.NewScheduler(mealAgent, cfg.Schedule.Lunch, cfg.Schedule.Dinner)
	scheduler.Start()

	// 监听通知
	go func() {
		for notification := range scheduler.Notifications() {
			fmt.Println(notification)
			fmt.Println("\n---")
		}
	}()

	// 等待退出信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	scheduler.Stop()
	fmt.Println("\n已退出")
}

// printWelcome 打印欢迎信息
func printWelcome() {
	fmt.Println("═══════════════════════════════════════════")
	fmt.Println("       🍽️  饮食推荐助手 Meal Agent")
	fmt.Println("═══════════════════════════════════════════")
	fmt.Println()
	fmt.Println("我可以根据天气和你的位置推荐附近餐厅。")
	fmt.Println("输入 'help' 查看所有命令，输入 'quit' 退出。")
	fmt.Println()

	// 显示当前时间和餐次
	hour := time.Now().Hour()
	mealType := "午餐"
	if hour >= 15 {
		mealType = "晚餐"
	} else if hour < 10 {
		mealType = "早餐/早午餐"
	}
	fmt.Printf("现在是 %s 时间，需要我推荐%s吗？\n", time.Now().Format("15:04"), mealType)
}

// printHelp 打印帮助信息
func printHelp() {
	fmt.Println(`
命令列表:
  推荐 / r          获取用餐推荐
  历史 / history    查看最近用餐记录
  记录 <餐厅名>     记录本次用餐
  重置 / reset      重置对话上下文
  帮助 / help       显示此帮助
  退出 / quit       退出程序

对话示例:
  "不想吃火锅"      排除火锅类餐厅
  "来点清淡的"      获取清淡食物推荐
  "就吃第一个"      确认选择
	`)
}

// handleRecommend 处理推荐请求
func handleRecommend(mealAgent *agent.MealAgent) {
	fmt.Println("\n助手: 正在为你搜索附近餐厅...")

	hour := time.Now().Hour()
	mealType := "lunch"
	if hour >= 15 {
		mealType = "dinner"
	}

	response, err := mealAgent.GetRecommendation(mealType)
	if err != nil {
		fmt.Printf("\n助手: 抱歉，获取推荐失败: %v\n", err)
		return
	}

	fmt.Printf("\n助手: %s\n", response)
}

// handleHistory 处理历史记录查询
func handleHistory(mealAgent *agent.MealAgent) {
	summary := mealAgent.GetHistorySummary()
	fmt.Printf("\n助手: %s\n", summary)
}

// handleRecord 处理记录用餐
func handleRecord(mealAgent *agent.MealAgent, input string) {
	// 解析: "记录 餐厅名 [类型]"
	parts := strings.Fields(input)
	if len(parts) < 2 {
		fmt.Println("\n助手: 请输入餐厅名称，例如: 记录 海底捞 火锅")
		return
	}

	restaurant := parts[1]
	category := ""
	if len(parts) >= 3 {
		category = parts[2]
	}

	err := mealAgent.RecordMeal(restaurant, category)
	if err != nil {
		fmt.Printf("\n助手: 记录失败: %v\n", err)
		return
	}

	fmt.Printf("\n助手: 已记录本次用餐: %s", restaurant)
	if category != "" {
		fmt.Printf("（%s）", category)
	}
	fmt.Println("\n下次推荐时会避免重复。")
}
