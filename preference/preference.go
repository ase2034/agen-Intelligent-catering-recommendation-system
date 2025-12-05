package preference

import (
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// RestaurantPreference 单个餐厅的偏好设置
type RestaurantPreference struct {
	Name   string `yaml:"name"`
	Weight int    `yaml:"weight"` // 权重，100为基准
	Note   string `yaml:"note"`   // 备注
}

// CategoryPreference 菜系偏好设置
type CategoryPreference struct {
	Type   string `yaml:"type"`
	Weight int    `yaml:"weight"`
	Note   string `yaml:"note"`
}

// Preferences 偏好配置
type Preferences struct {
	Restaurants []RestaurantPreference `yaml:"restaurants"`
	Categories  []CategoryPreference   `yaml:"categories"`

	// 内部索引
	restaurantMap map[string]int // name -> weight
	categoryMap   map[string]int // type -> weight
}

// Load 加载偏好配置
func Load(path string) (*Preferences, error) {
	p := &Preferences{
		Restaurants:   []RestaurantPreference{},
		Categories:    []CategoryPreference{},
		restaurantMap: make(map[string]int),
		categoryMap:   make(map[string]int),
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// 文件不存在，返回空配置
			return p, nil
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, p); err != nil {
		return nil, err
	}

	// 构建索引
	for _, r := range p.Restaurants {
		p.restaurantMap[r.Name] = r.Weight
	}
	for _, c := range p.Categories {
		p.categoryMap[c.Type] = c.Weight
	}

	return p, nil
}

// Save 保存偏好配置
func (p *Preferences) Save(path string) error {
	data, err := yaml.Marshal(p)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// GetRestaurantWeight 获取餐厅权重
// 返回：权重值（未配置返回100）
func (p *Preferences) GetRestaurantWeight(name string) int {
	if weight, ok := p.restaurantMap[name]; ok {
		return weight
	}
	return 100 // 默认权重
}

// GetCategoryWeight 获取菜系权重
// typeStr: 高德返回的类型字符串，如 "餐饮服务;中餐厅;川菜"
func (p *Preferences) GetCategoryWeight(typeStr string) int {
	for category, weight := range p.categoryMap {
		if strings.Contains(typeStr, category) {
			return weight
		}
	}
	return 100 // 默认权重
}

// SetRestaurantWeight 设置餐厅权重
func (p *Preferences) SetRestaurantWeight(name string, weight int, note string) {
	// 更新或添加
	found := false
	for i, r := range p.Restaurants {
		if r.Name == name {
			p.Restaurants[i].Weight = weight
			p.Restaurants[i].Note = note
			found = true
			break
		}
	}
	if !found {
		p.Restaurants = append(p.Restaurants, RestaurantPreference{
			Name:   name,
			Weight: weight,
			Note:   note,
		})
	}
	p.restaurantMap[name] = weight
}

// IsBlacklisted 检查餐厅是否被排除（权重为0）
func (p *Preferences) IsBlacklisted(name string) bool {
	if weight, ok := p.restaurantMap[name]; ok {
		return weight == 0
	}
	return false
}