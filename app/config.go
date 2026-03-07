package app

import (
	"fmt"
	"path/filepath"
)

// Config 小说应用配置。
type Config struct {
	Prompt      string // 用户的小说需求
	NovelName   string // 小说名（用作输出目录名）
	OutputDir   string // 输出根目录，默认 output/{NovelName}
	ModelName   string // LLM 模型名
	APIKey      string // API Key
	BaseURL     string // API Base URL（可选）
	MaxChapters int    // 最大章节数
	Style       string // 写作风格（default/suspense/fantasy/romance）
}

// Prompts 嵌入的提示词。
type Prompts struct {
	Coordinator string
	Architect   string
	Writer      string
	Editor      string
}

// Validate 校验配置。
func (c *Config) Validate() error {
	if c.Prompt == "" {
		return fmt.Errorf("prompt is required")
	}
	if c.APIKey == "" {
		return fmt.Errorf("api key is required (set OPENAI_API_KEY)")
	}
	return nil
}

// FillDefaults 填充默认值。
func (c *Config) FillDefaults() {
	if c.NovelName == "" {
		c.NovelName = "novel"
	}
	if c.OutputDir == "" {
		c.OutputDir = filepath.Join("output", c.NovelName)
	}
	if c.ModelName == "" {
		c.ModelName = "gpt-4o"
	}
	if c.Style == "" {
		c.Style = "default"
	}
	if c.MaxChapters <= 0 {
		c.MaxChapters = 3
	}
}
