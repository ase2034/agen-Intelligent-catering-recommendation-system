# Meal Agent 🍽️

一个基于 LLM 的智能饮食推荐助手，根据天气、位置、用餐历史和个人偏好推荐附近餐厅。

## 功能特性

- 🌤️ **天气感知** - 根据天气推荐合适的食物（冷天推荐热食，热天推荐清淡）
- 📍 **位置服务** - 基于高德地图搜索附近餐厅
- 📊 **智能权重** - 避免连续推荐相同餐厅，支持自定义偏好
- 💬 **对话交互** - 支持自然语言排除不想吃的类型
- ⏰ **定时提醒** - 后台模式可定时推送午餐/晚餐建议

## 快速开始

### 1. 安装依赖

```bash
go mod tidy
```

### 2. 配置

复制示例配置文件并填写：

```bash
cp config.example.yaml config.yaml
cp restaurants.example.yaml restaurants.yaml
```

需要配置：
- **高德地图 API Key** - 用于搜索附近餐厅
- **和风天气 API Key** - 用于获取天气信息
- **LLM API** - 支持 OpenAI 兼容接口（如阿里云通义千问）

### 3. 运行

```bash
# 交互模式
go run main.go

# 或指定配置文件
go run main.go -config config.yaml -pref restaurants.yaml

# 后台定时模式
go run main.go -mode daemon
```

## 使用方法

### 交互命令

| 命令 | 说明 |
|------|------|
| `推荐` / `r` | 获取用餐推荐 |
| `历史` | 查看最近用餐记录 |
| `记录 餐厅名 [类型]` | 手动记录用餐 |
| `重置` | 清空对话上下文 |
| `退出` / `q` | 退出程序 |

### 对话示例

```
你: 推荐
助手: 根据今天的天气和你的位置，我推荐：
1. 海底捞（火锅，适合今天的天气）
2. ...

你: 不想吃火锅
助手: 好的，已排除火锅类，重新推荐...

你: 就吃第一个
助手: 好的，已记录本次午餐选择：XXX
```

## 配置说明

### config.yaml

```yaml
location:
  lat: "31.xxxxx"      # 纬度
  lng: "121.xxxxx"     # 经度
  city: "上海"
  radius: 1000         # 搜索半径（米）

api:
  amap_key: "xxx"      # 高德地图 Key
  weather_key: "xxx"   # 和风天气 Key

llm:
  provider: "openai"
  api_key: "xxx"
  base_url: "https://xxx/v1"
  model: "qwen-plus"
```

### restaurants.yaml（可选）

自定义餐厅权重：

```yaml
restaurants:
  - name: "海底捞"
    weight: 150        # >100 更喜欢
  - name: "某餐厅"
    weight: 0          # =0 永久排除
  - name: "快餐店"
    weight: 60         # <100 不太喜欢
```

## 权重机制

基础权重 100，最终权重 = 基础 + 偏好调整 + 历史惩罚

**历史惩罚：**
- 今天吃过：-80
- 昨天吃过：-50
- 2天前：-30
- 3天前：-15

## 项目结构

```
meal-agent/
├── main.go              # 入口
├── agent/
│   ├── agent.go         # 核心逻辑
│   ├── llm.go           # LLM 调用
│   └── scheduler.go     # 定时任务
├── config/
│   └── config.go        # 配置加载
├── tools/
│   ├── restaurant.go    # 高德地图 API
│   └── weather.go       # 天气 API
├── memory/
│   └── history.go       # 历史记录
└── preference/
    └── preference.go    # 用户偏好
```

## License

MIT