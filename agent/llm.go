package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"meal-agent/config"
)

// LLM 定义 LLM 接口
type LLM interface {
	Chat(messages []Message) (string, error)
}

// Message 聊天消息
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OpenAICompatibleLLM 兼容 OpenAI 格式的 LLM（大部分国产模型都支持）
type OpenAICompatibleLLM struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

// NewLLM 根据配置创建 LLM 实例
func NewLLM(cfg config.LLMConfig) LLM {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		// 根据 provider 设置默认 URL
		switch cfg.Provider {
		case "openai":
			baseURL = "https://api.openai.com/v1"
		case "claude":
			// Claude 需要单独实现，这里先用兼容模式
			baseURL = "https://api.anthropic.com/v1"
		case "zhipu":
			baseURL = "https://open.bigmodel.cn/api/paas/v4"
		case "deepseek":
			baseURL = "https://api.deepseek.com/v1"
		case "moonshot":
			baseURL = "https://api.moonshot.cn/v1"
		case "qwen":
			baseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"
		default:
			baseURL = "https://api.openai.com/v1"
		}
	}

	return &OpenAICompatibleLLM{
		apiKey:  cfg.APIKey,
		baseURL: baseURL,
		model:   cfg.Model,
		client:  &http.Client{},
	}
}

// Chat 发送聊天请求
func (l *OpenAICompatibleLLM) Chat(messages []Message) (string, error) {
	reqBody := map[string]interface{}{
		"model":    l.model,
		"messages": messages,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", l.baseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+l.apiKey)

	resp, err := l.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error: %s", string(body))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no response from LLM")
	}

	return result.Choices[0].Message.Content, nil
}