package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// WeatherClient 和风天气客户端
type WeatherClient struct {
	apiKey string
	client *http.Client
}

// WeatherInfo 天气信息
type WeatherInfo struct {
	Temp      string // 温度
	FeelsLike string // 体感温度
	Text      string // 天气描述（晴、多云等）
	WindDir   string // 风向
	WindScale string // 风力等级
	Humidity  string // 湿度
}

// NewWeatherClient 创建天气客户端
func NewWeatherClient(apiKey string) *WeatherClient {
	return &WeatherClient{
		apiKey: apiKey,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetWeather 获取实时天气（带重试）
func (w *WeatherClient) GetWeather(city string) (*WeatherInfo, error) {
	var lastErr error
	for retry := 0; retry < 3; retry++ {
		if retry > 0 {
			time.Sleep(time.Duration(retry) * time.Second)
		}

		info, err := w.getWeatherOnce(city)
		if err == nil {
			return info, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("获取天气失败（已重试3次）: %v", lastErr)
}

// getWeatherOnce 单次获取天气
func (w *WeatherClient) getWeatherOnce(city string) (*WeatherInfo, error) {
	// 先查询城市 ID
	locationID, err := w.getCityID(city)
	if err != nil {
		return nil, fmt.Errorf("查询城市失败: %v", err)
	}

	// 获取实时天气
	weatherURL := fmt.Sprintf(
		"https://devapi.qweather.com/v7/weather/now?location=%s&key=%s",
		locationID,
		w.apiKey,
	)

	resp, err := w.client.Get(weatherURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Code string `json:"code"`
		Now  struct {
			Temp      string `json:"temp"`
			FeelsLike string `json:"feelsLike"`
			Text      string `json:"text"`
			WindDir   string `json:"windDir"`
			WindScale string `json:"windScale"`
			Humidity  string `json:"humidity"`
		} `json:"now"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if result.Code != "200" {
		return nil, fmt.Errorf("天气API错误，code: %s", result.Code)
	}

	return &WeatherInfo{
		Temp:      result.Now.Temp,
		FeelsLike: result.Now.FeelsLike,
		Text:      result.Now.Text,
		WindDir:   result.Now.WindDir,
		WindScale: result.Now.WindScale,
		Humidity:  result.Now.Humidity,
	}, nil
}

// getCityID 获取城市 ID
func (w *WeatherClient) getCityID(city string) (string, error) {
	geoURL := fmt.Sprintf(
		"https://geoapi.qweather.com/v2/city/lookup?location=%s&key=%s",
		url.QueryEscape(city),
		w.apiKey,
	)

	resp, err := w.client.Get(geoURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result struct {
		Code     string `json:"code"`
		Location []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"location"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	if result.Code != "200" || len(result.Location) == 0 {
		return "", fmt.Errorf("城市未找到: %s", city)
	}

	return result.Location[0].ID, nil
}

// Describe 返回天气描述文本
func (w *WeatherInfo) Describe() string {
	return fmt.Sprintf(
		"当前天气：%s，温度 %s°C，体感温度 %s°C，%s %s级，湿度 %s%%",
		w.Text, w.Temp, w.FeelsLike, w.WindDir, w.WindScale, w.Humidity,
	)
}

// SuggestFoodType 根据天气推荐食物类型
func (w *WeatherInfo) SuggestFoodType() string {
	// 简单的规则引擎
	temp := 0
	fmt.Sscanf(w.Temp, "%d", &temp)

	switch {
	case temp <= 5:
		return "天气寒冷，推荐热汤、火锅、羊肉等暖身食物"
	case temp <= 15:
		return "天气偏凉，推荐热食、炖菜、面食等"
	case temp <= 25:
		return "天气舒适，各类食物都适合"
	case temp <= 32:
		return "天气炎热，推荐清淡、凉菜、冷面等解暑食物"
	default:
		return "天气酷热，推荐解暑降温的食物，注意多喝水"
	}
}