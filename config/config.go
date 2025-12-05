package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Location Location  `yaml:"location"`
	Schedule Schedule  `yaml:"schedule"`
	Blacklist []string `yaml:"blacklist"`
	TempExclude []string `yaml:"temp_exclude"`
	API      APIConfig `yaml:"api"`
	LLM      LLMConfig `yaml:"llm"`
}

type Location struct {
	Lat    string `yaml:"lat"`
	Lng    string `yaml:"lng"`
	City   string `yaml:"city"`
	Radius int    `yaml:"radius"`
}

type Schedule struct {
	Lunch  string `yaml:"lunch"`
	Dinner string `yaml:"dinner"`
}

type APIConfig struct {
	AmapKey    string `yaml:"amap_key"`
	WeatherKey string `yaml:"weather_key"`
}

type LLMConfig struct {
	Provider string `yaml:"provider"`
	APIKey   string `yaml:"api_key"`
	BaseURL  string `yaml:"base_url"`
	Model    string `yaml:"model"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// 设置默认值
	if cfg.Location.Radius == 0 {
		cfg.Location.Radius = 1000
	}

	return &cfg, nil
}

// Save 保存配置（用于更新临时排除列表）
func (c *Config) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// IsBlacklisted 检查餐厅是否在黑名单中
func (c *Config) IsBlacklisted(name string) bool {
	for _, b := range c.Blacklist {
		if b == name {
			return true
		}
	}
	for _, t := range c.TempExclude {
		if t == name {
			return true
		}
	}
	return false
}

// AddTempExclude 添加临时排除
func (c *Config) AddTempExclude(name string) {
	c.TempExclude = append(c.TempExclude, name)
}

// ClearTempExclude 清空临时排除（每天清空）
func (c *Config) ClearTempExclude() {
	c.TempExclude = []string{}
}