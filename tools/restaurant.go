package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// RestaurantClient 高德地图餐厅搜索客户端
type RestaurantClient struct {
	apiKey string
	client *http.Client
}

// MealCategory 餐厅大类
type MealCategory string

const (
	CategoryQuickMeal MealCategory = "quick"  // 快餐类：面、拌饭、简餐
	CategoryFullMeal  MealCategory = "full"   // 正餐炒菜类
	CategoryOther     MealCategory = "other"  // 其他
)

// Restaurant 餐厅信息
type Restaurant struct {
	Name     string `json:"name"`     // 餐厅名称
	Type     string `json:"type"`     // 餐厅类型（川菜、火锅等）
	Address  string `json:"address"`  // 地址
	Distance string `json:"distance"` // 距离（米）
	Rating   string `json:"rating"`   // 评分
	Cost     string `json:"cost"`     // 人均消费
	Tel      string `json:"tel"`      // 电话
	Weight   int    `json:"-"`        // 计算后的权重（不序列化）
	Category MealCategory `json:"-"`  // 餐厅大类（快餐/正餐）
}

// NewRestaurantClient 创建餐厅搜索客户端
func NewRestaurantClient(apiKey string) *RestaurantClient {
	return &RestaurantClient{
		apiKey: apiKey,
		client: &http.Client{},
	}
}

// SearchNearby 搜索附近餐厅
// lat, lng: 经纬度
// radius: 搜索半径（米）
// keyword: 可选关键词（如"火锅"、"川菜"）
func (r *RestaurantClient) SearchNearby(lat, lng string, radius int, keyword string) ([]Restaurant, error) {
	// 高德 POI 搜索 API
	// types=050000 表示餐饮服务
	url := fmt.Sprintf(
		"https://restapi.amap.com/v3/place/around?key=%s&location=%s,%s&radius=%d&types=050000&offset=20&extensions=all",
		r.apiKey,
		lng, // 高德是 lng,lat 顺序
		lat,
		radius,
	)

	if keyword != "" {
		url += "&keywords=" + keyword
	}

	resp, err := r.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Status string `json:"status"`
		Info   string `json:"info"`
		Pois   []struct {
			Name     flexString      `json:"name"`
			Type     flexString      `json:"type"`
			Address  flexString      `json:"address"`
			Distance flexString      `json:"distance"`
			BizExt   json.RawMessage `json:"biz_ext"` // 可能是对象或空数组
			Tel      flexString      `json:"tel"`
		} `json:"pois"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if result.Status != "1" {
		return nil, fmt.Errorf("高德API错误: %s", result.Info)
	}

	restaurants := make([]Restaurant, 0, len(result.Pois))
	for _, poi := range result.Pois {
		// 解析 biz_ext，处理可能是空数组的情况
		rating, cost := parseBizExt(poi.BizExt)

		restaurants = append(restaurants, Restaurant{
			Name:     string(poi.Name),
			Type:     string(poi.Type),
			Address:  string(poi.Address),
			Distance: string(poi.Distance),
			Rating:   rating,
			Cost:     cost,
			Tel:      string(poi.Tel),
		})
	}

	return restaurants, nil
}

// flexString 处理高德API中可能是字符串或空数组的字段
type flexString string

func (f *flexString) UnmarshalJSON(data []byte) error {
	// 如果是空数组 []
	if string(data) == "[]" {
		*f = ""
		return nil
	}
	// 否则按字符串解析
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		*f = ""
		return nil
	}
	*f = flexString(s)
	return nil
}

// parseBizExt 解析 biz_ext 字段（高德API返回的可能是对象或空数组）
func parseBizExt(raw json.RawMessage) (rating, cost string) {
	if len(raw) == 0 || string(raw) == "[]" {
		return "", ""
	}

	// 尝试解析为对象
	var bizExt struct {
		Rating flexString `json:"rating"`
		Cost   flexString `json:"cost"`
	}
	if err := json.Unmarshal(raw, &bizExt); err == nil {
		return string(bizExt.Rating), string(bizExt.Cost)
	}

	// 解析失败，返回空
	return "", ""
}

// FilterByBlacklist 过滤黑名单餐厅
func FilterByBlacklist(restaurants []Restaurant, blacklist []string) []Restaurant {
	blacklistMap := make(map[string]bool)
	for _, name := range blacklist {
		blacklistMap[name] = true
	}

	filtered := make([]Restaurant, 0)
	for _, r := range restaurants {
		if !blacklistMap[r.Name] {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// FilterByType 按类型过滤（排除某些类型）
func FilterByType(restaurants []Restaurant, excludeTypes []string) []Restaurant {
	filtered := make([]Restaurant, 0)
	for _, r := range restaurants {
		excluded := false
		for _, t := range excludeTypes {
			if strings.Contains(r.Type, t) || strings.Contains(r.Name, t) {
				excluded = true
				break
			}
		}
		if !excluded {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// Describe 返回餐厅描述
func (r *Restaurant) Describe() string {
	desc := fmt.Sprintf("%s", r.Name)
	if r.Type != "" {
		desc += fmt.Sprintf("（%s）", r.Type)
	}
	if r.Distance != "" {
		desc += fmt.Sprintf(" - %s米", r.Distance)
	}
	if r.Rating != "" && r.Rating != "[]" {
		desc += fmt.Sprintf(" - 评分%s", r.Rating)
	}
	if r.Cost != "" && r.Cost != "[]" {
		desc += fmt.Sprintf(" - 人均¥%s", r.Cost)
	}
	return desc
}

// SortByWeight 按权重排序（权重高的在前）
func SortByWeight(restaurants []Restaurant) {
	for i := 0; i < len(restaurants)-1; i++ {
		for j := i + 1; j < len(restaurants); j++ {
			if restaurants[j].Weight > restaurants[i].Weight {
				restaurants[i], restaurants[j] = restaurants[j], restaurants[i]
			}
		}
	}
}

// FilterByWeight 过滤掉权重为0或负数的餐厅
func FilterByWeight(restaurants []Restaurant) []Restaurant {
	filtered := make([]Restaurant, 0)
	for _, r := range restaurants {
		if r.Weight > 0 {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// 快餐类关键词（面、拌饭、简餐、快餐等）
var quickMealKeywords = []string{
	"面", "粉", "拌饭", "盖饭", "快餐", "简餐", "便当", "饭团",
	"包子", "饺子", "馄饨", "小吃", "煎饼", "肉夹馍", "凉皮",
	"麻辣烫", "冒菜", "米线", "酸辣粉", "螺蛳粉",
	"汉堡", "披萨", "炸鸡", "三明治", "沙拉",
	"寿司", "饭卷", "便利店",
}

// 正餐炒菜类关键词
var fullMealKeywords = []string{
	"中餐厅", "川菜", "湘菜", "粤菜", "鲁菜", "苏菜", "浙菜", "徽菜", "闽菜",
	"东北菜", "本帮菜", "家常菜", "私房菜", "农家菜",
	"火锅", "烤肉", "烧烤", "自助餐",
	"西餐", "日料", "韩餐", "泰餐", "东南亚",
}

// ClassifyRestaurant 判断餐厅类型
func ClassifyRestaurant(r *Restaurant) MealCategory {
	nameAndType := r.Name + r.Type

	// 先检查快餐类
	for _, kw := range quickMealKeywords {
		if strings.Contains(nameAndType, kw) {
			return CategoryQuickMeal
		}
	}

	// 再检查正餐类
	for _, kw := range fullMealKeywords {
		if strings.Contains(nameAndType, kw) {
			return CategoryFullMeal
		}
	}

	return CategoryOther
}

// ClassifyAllRestaurants 为所有餐厅分类
func ClassifyAllRestaurants(restaurants []Restaurant) {
	for i := range restaurants {
		restaurants[i].Category = ClassifyRestaurant(&restaurants[i])
	}
}

// GetDistanceInt 获取距离的整数值（米）
func (r *Restaurant) GetDistanceInt() int {
	if r.Distance == "" {
		return 0
	}
	var dist int
	fmt.Sscanf(r.Distance, "%d", &dist)
	return dist
}

// GetRatingFloat 获取评分的浮点值
func (r *Restaurant) GetRatingFloat() float64 {
	if r.Rating == "" || r.Rating == "[]" {
		return 0
	}
	var rating float64
	fmt.Sscanf(r.Rating, "%f", &rating)
	return rating
}
