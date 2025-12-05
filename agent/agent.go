package agent

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"meal-agent/config"
	"meal-agent/memory"
	"meal-agent/preference"
	"meal-agent/tools"
)

// MealAgent 饮食建议 Agent
type MealAgent struct {
	cfg        *config.Config
	llm        LLM
	weather    *tools.WeatherClient
	restaurant *tools.RestaurantClient
	history    *memory.History
	pref       *preference.Preferences // 餐厅偏好配置

	// 对话上下文
	messages        []Message
	tempExclude     []string           // 本次对话临时排除的类型
	lastRestaurants []tools.Restaurant // 上次推荐的餐厅列表（用于确认选择）
}

// NewMealAgent 创建 Agent
func NewMealAgent(cfg *config.Config, history *memory.History, pref *preference.Preferences) *MealAgent {
	return &MealAgent{
		cfg:             cfg,
		llm:             NewLLM(cfg.LLM),
		weather:         tools.NewWeatherClient(cfg.API.WeatherKey),
		restaurant:      tools.NewRestaurantClient(cfg.API.AmapKey),
		history:         history,
		pref:            pref,
		messages:        []Message{},
		tempExclude:     []string{},
		lastRestaurants: []tools.Restaurant{},
	}
}

// GetRecommendation 获取用餐推荐
func (a *MealAgent) GetRecommendation(mealType string) (string, error) {
	// 1. 获取天气信息
	weatherInfo, err := a.weather.GetWeather(a.cfg.Location.City)
	if err != nil {
		weatherInfo = &tools.WeatherInfo{Text: "未知", Temp: "20"}
	}

	// 2. 搜索附近餐厅
	restaurants, err := a.restaurant.SearchNearby(
		a.cfg.Location.Lat,
		a.cfg.Location.Lng,
		a.cfg.Location.Radius,
		"",
	)
	if err != nil {
		return "", fmt.Errorf("搜索餐厅失败: %v", err)
	}

	// 3. 过滤黑名单（按餐厅名称）
	allBlacklist := append([]string{}, a.cfg.Blacklist...)
	allBlacklist = append(allBlacklist, a.cfg.TempExclude...)
	restaurants = tools.FilterByBlacklist(restaurants, allBlacklist)

	// 4. 过滤排除的类型（按餐厅类型关键词）
	if len(a.tempExclude) > 0 {
		restaurants = tools.FilterByType(restaurants, a.tempExclude)
	}

	// 5. 计算权重并排序（替代简单的过滤）
	penalties := a.history.GetAllPenalties()
	for i := range restaurants {
		// 基础权重 100
		weight := 100

		// 加上用户偏好权重
		if a.pref != nil {
			prefWeight := a.pref.GetRestaurantWeight(restaurants[i].Name)
			if prefWeight == 0 {
				// 权重为0表示黑名单，跳过
				weight = 0
			} else {
				weight = prefWeight
			}
			// 加上菜系偏好
			catWeight := a.pref.GetCategoryWeight(restaurants[i].Type)
			if catWeight != 100 {
				weight = weight * catWeight / 100
			}
		}

		// 减去历史惩罚（最近吃过的降权）
		if penalty, ok := penalties[restaurants[i].Name]; ok {
			weight += penalty
		}

		restaurants[i].Weight = weight
	}

	// 过滤掉权重<=0的餐厅
	restaurants = tools.FilterByWeight(restaurants)

	// 按权重排序
	tools.SortByWeight(restaurants)

	if len(restaurants) == 0 {
		return "附近没有找到合适的餐厅，考虑扩大搜索范围或减少排除条件", nil
	}

	// 保存推荐的餐厅列表（用于后续确认）
	a.lastRestaurants = restaurants

	// 6. 构建 prompt，让 LLM 推荐
	prompt := a.buildPrompt(mealType, weatherInfo, restaurants)

	// 添加系统消息
	if len(a.messages) == 0 {
		a.messages = append(a.messages, Message{
			Role:    "system",
			Content: systemPrompt,
		})
	}

	a.messages = append(a.messages, Message{
		Role:    "user",
		Content: prompt,
	})

	// 7. 调用 LLM
	response, err := a.llm.Chat(a.messages)
	if err != nil {
		return "", fmt.Errorf("LLM 调用失败: %v", err)
	}

	a.messages = append(a.messages, Message{
		Role:    "assistant",
		Content: response,
	})

	return response, nil
}

// Chat 对话模式
func (a *MealAgent) Chat(userInput string) (string, error) {
	// 检查是否要排除某些选项
	if strings.Contains(userInput, "不想吃") || strings.Contains(userInput, "不要") ||
		strings.Contains(userInput, "不吃") || strings.Contains(userInput, "换一个") {
		a.parseExclusion(userInput)
	}

	// 检查是否确认选择
	if a.isConfirmation(userInput) {
		return a.confirmChoice(userInput)
	}

	// 检查是否请求推荐
	if strings.Contains(userInput, "推荐") || strings.Contains(userInput, "吃什么") ||
		strings.Contains(userInput, "有什么") {
		hour := time.Now().Hour()
		mealType := "lunch"
		if hour >= 15 {
			mealType = "dinner"
		}
		return a.GetRecommendation(mealType)
	}

	// 添加用户消息
	a.messages = append(a.messages, Message{
		Role:    "user",
		Content: userInput,
	})

	// 调用 LLM
	response, err := a.llm.Chat(a.messages)
	if err != nil {
		return "", err
	}

	a.messages = append(a.messages, Message{
		Role:    "assistant",
		Content: response,
	})

	return response, nil
}

// isConfirmation 检查是否是确认选择
func (a *MealAgent) isConfirmation(input string) bool {
	confirmKeywords := []string{"就这个", "就吃", "好的", "确定", "就它", "选这个", "第一个", "第二个", "第三个"}
	for _, kw := range confirmKeywords {
		if strings.Contains(input, kw) {
			return true
		}
	}
	return false
}

// parseExclusion 解析排除项
func (a *MealAgent) parseExclusion(input string) {
	// 扩展关键词列表
	keywords := []string{
		"火锅", "川菜", "湘菜", "烧烤", "日料", "韩餐", "西餐",
		"面", "米饭", "快餐", "麻辣", "清淡", "油腻",
		"粤菜", "东北菜", "本帮菜", "鲁菜", "徽菜",
		"披萨", "汉堡", "炸鸡", "烤肉", "寿司", "拉面",
		"饺子", "包子", "小吃", "甜品", "奶茶",
	}

	for _, kw := range keywords {
		if strings.Contains(input, kw) && !a.containsExclude(kw) {
			a.tempExclude = append(a.tempExclude, kw)
		}
	}
}

// containsExclude 检查是否已在排除列表
func (a *MealAgent) containsExclude(kw string) bool {
	for _, e := range a.tempExclude {
		if e == kw {
			return true
		}
	}
	return false
}

// confirmChoice 确认选择并记录
func (a *MealAgent) confirmChoice(input string) (string, error) {
	// 尝试从用户输入中提取选择
	selectedRestaurant := a.extractSelection(input)

	if selectedRestaurant == nil {
		// 如果无法确定，让用户明确
		return "请告诉我你选择哪个餐厅，可以说餐厅名称或者「第一个」「第二个」等", nil
	}

	// 记录到历史
	mealType := "lunch"
	hour := time.Now().Hour()
	if hour >= 15 {
		mealType = "dinner"
	}

	err := a.history.Add(memory.MealRecord{
		Date:       time.Now().Format("2006-01-02"),
		MealType:   mealType,
		Restaurant: selectedRestaurant.Name,
		Category:   extractCategory(selectedRestaurant.Type),
	})
	if err != nil {
		return "", fmt.Errorf("记录失败: %v", err)
	}

	mealName := map[string]string{"lunch": "午餐", "dinner": "晚餐"}[mealType]
	return fmt.Sprintf("好的，已记录本次%s选择：%s。下次会避免重复推荐。祝用餐愉快！🍽️",
		mealName, selectedRestaurant.Name), nil
}

// extractSelection 从用户输入中提取选择的餐厅
func (a *MealAgent) extractSelection(input string) *tools.Restaurant {
	if len(a.lastRestaurants) == 0 {
		return nil
	}

	// 检查是否指定了序号
	orderPatterns := []struct {
		pattern string
		index   int
	}{
		{"第一", 0}, {"1号", 0}, {"第1", 0},
		{"第二", 1}, {"2号", 1}, {"第2", 1},
		{"第三", 2}, {"3号", 2}, {"第3", 2},
	}

	for _, p := range orderPatterns {
		if strings.Contains(input, p.pattern) && p.index < len(a.lastRestaurants) {
			return &a.lastRestaurants[p.index]
		}
	}

	// 检查是否包含餐厅名称
	for i := range a.lastRestaurants {
		if strings.Contains(input, a.lastRestaurants[i].Name) {
			return &a.lastRestaurants[i]
		}
	}

	// 如果只说"就这个"、"好的"之类，且只有一个推荐，默认选第一个
	if len(a.lastRestaurants) > 0 && (strings.Contains(input, "就这个") ||
		strings.Contains(input, "就它") || strings.Contains(input, "好的")) {
		return &a.lastRestaurants[0]
	}

	return nil
}

// extractCategory 从高德类型字符串中提取主要分类
func extractCategory(typeStr string) string {
	// 高德返回的类型格式类似 "餐饮服务;中餐厅;川菜"
	parts := strings.Split(typeStr, ";")
	if len(parts) >= 3 {
		return parts[2]
	}
	if len(parts) >= 2 {
		return parts[1]
	}
	return typeStr
}

// RecordMeal 记录用餐
func (a *MealAgent) RecordMeal(restaurant, category string) error {
	mealType := "lunch"
	hour := time.Now().Hour()
	if hour >= 15 {
		mealType = "dinner"
	}

	return a.history.Add(memory.MealRecord{
		Date:       time.Now().Format("2006-01-02"),
		MealType:   mealType,
		Restaurant: restaurant,
		Category:   category,
	})
}

// GetHistorySummary 获取历史记录摘要
func (a *MealAgent) GetHistorySummary() string {
	return a.history.Summary()
}

// Reset 重置对话上下文
func (a *MealAgent) Reset() {
	a.messages = []Message{}
	a.tempExclude = []string{}
	a.lastRestaurants = []tools.Restaurant{}
}

// buildPrompt 构建推荐 prompt
func (a *MealAgent) buildPrompt(mealType string, weather *tools.WeatherInfo, restaurants []tools.Restaurant) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("现在是%s时间，请推荐用餐选择。\n\n",
		map[string]string{"lunch": "午餐", "dinner": "晚餐"}[mealType]))

	sb.WriteString("【天气信息】\n")
	sb.WriteString(weather.Describe() + "\n")
	sb.WriteString(weather.SuggestFoodType() + "\n\n")

	sb.WriteString("【附近餐厅】\n")
	for i, r := range restaurants {
		if i >= 15 { // 最多展示15个
			break
		}
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, r.Describe()))
	}

	sb.WriteString("\n【历史记录】\n")
	sb.WriteString(a.history.Summary())

	if len(a.tempExclude) > 0 {
		sb.WriteString("\n【本次排除】\n")
		sb.WriteString("用户表示不想吃：" + strings.Join(a.tempExclude, "、"))
	}

	sb.WriteString("\n\n请根据以上信息，推荐 3 个最合适的选择，并说明推荐理由。")

	return sb.String()
}

// GetExcludeList 获取当前排除列表（用于调试）
func (a *MealAgent) GetExcludeList() []string {
	return a.tempExclude
}

const systemPrompt = `你是一个贴心的饮食建议助手。你的任务是根据天气、用户位置附近的餐厅、以及用户的历史用餐记录，给出合适的用餐建议。

注意事项：
1. 根据天气推荐合适的食物类型（冷天推荐热食，热天推荐清淡）
2. 避免连续几天推荐相同的餐厅
3. 推荐时考虑餐厅评分和距离
4. 如果用户说不想吃某种类型，要记住并排除
5. 回复要简洁实用，不要太啰嗦
6. 给出 2-3 个选择，让用户决定

回复格式示例：
根据今天的天气和你的位置，我推荐：
1. XXX（推荐理由）
2. YYY（推荐理由）
3. ZZZ（推荐理由）

想吃哪个？或者告诉我你不想吃什么，我再推荐。`

// 用于从 LLM 回复中提取推荐的餐厅（备用）
var restaurantPattern = regexp.MustCompile(`\d+\.\s*([^\n（(]+)`)
