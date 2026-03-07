package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"unicode/utf8"

	"github.com/voocel/agentcore/schema"
	"github.com/voocel/ainovel-cli/state"
)

// PolishChapterTool 保存打磨后的章节正文，替换原始场景拼接。
type PolishChapterTool struct {
	store *state.Store
}

func NewPolishChapterTool(store *state.Store) *PolishChapterTool {
	return &PolishChapterTool{store: store}
}

func (t *PolishChapterTool) Name() string { return "polish_chapter" }
func (t *PolishChapterTool) Description() string {
	return "保存打磨后的章节正文。在 write_scene 全部完成后、commit_chapter 之前调用。提交时会优先使用打磨版本"
}
func (t *PolishChapterTool) Label() string { return "打磨章节" }

func (t *PolishChapterTool) Schema() map[string]any {
	return schema.Object(
		schema.Property("chapter", schema.Int("章节号")).Required(),
		schema.Property("content", schema.String("打磨后的完整章节正文")).Required(),
	)
}

func (t *PolishChapterTool) Execute(_ context.Context, args json.RawMessage) (json.RawMessage, error) {
	var a struct {
		Chapter int    `json:"chapter"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("invalid args: %w", err)
	}
	if a.Chapter <= 0 {
		return nil, fmt.Errorf("chapter must be > 0")
	}
	if a.Content == "" {
		return nil, fmt.Errorf("content must not be empty")
	}

	if err := t.store.SavePolished(a.Chapter, a.Content); err != nil {
		return nil, fmt.Errorf("save polished: %w", err)
	}

	return json.Marshal(map[string]any{
		"polished":   true,
		"chapter":    a.Chapter,
		"word_count": utf8.RuneCountInString(a.Content),
	})
}
