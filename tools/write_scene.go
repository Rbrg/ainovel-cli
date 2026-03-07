package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"unicode/utf8"

	"github.com/voocel/agentcore/schema"
	"github.com/voocel/ainovel-cli/domain"
	"github.com/voocel/ainovel-cli/state"
)

// WriteSceneTool 写入单个场景草稿。
type WriteSceneTool struct {
	store *state.Store
}

func NewWriteSceneTool(store *state.Store) *WriteSceneTool {
	return &WriteSceneTool{store: store}
}

func (t *WriteSceneTool) Name() string { return "write_scene" }
func (t *WriteSceneTool) Description() string {
	return "写入单个场景草稿。严格按场景级写作，每次只写一个场景。必须先调用 plan_chapter"
}
func (t *WriteSceneTool) Label() string { return "写入场景" }

func (t *WriteSceneTool) Schema() map[string]any {
	return schema.Object(
		schema.Property("chapter", schema.Int("章节号")).Required(),
		schema.Property("scene", schema.Int("场景编号，从 1 开始")).Required(),
		schema.Property("content", schema.String("场景正文")).Required(),
	)
}

func (t *WriteSceneTool) Execute(_ context.Context, args json.RawMessage) (json.RawMessage, error) {
	var a struct {
		Chapter int    `json:"chapter"`
		Scene   int    `json:"scene"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("invalid args: %w", err)
	}
	if a.Chapter <= 0 || a.Scene <= 0 {
		return nil, fmt.Errorf("chapter and scene must be > 0")
	}
	if a.Content == "" {
		return nil, fmt.Errorf("content must not be empty")
	}

	wordCount := utf8.RuneCountInString(a.Content)
	draft := domain.SceneDraft{
		Chapter:   a.Chapter,
		Scene:     a.Scene,
		Content:   a.Content,
		WordCount: wordCount,
	}

	if err := t.store.SaveSceneDraft(draft); err != nil {
		return nil, fmt.Errorf("save scene draft: %w", err)
	}

	// 场景级 checkpoint
	if err := t.store.MarkSceneComplete(a.Chapter, a.Scene); err != nil {
		return nil, fmt.Errorf("mark scene complete: %w", err)
	}

	return json.Marshal(map[string]any{
		"written":    true,
		"chapter":    a.Chapter,
		"scene":      a.Scene,
		"word_count": wordCount,
	})
}
