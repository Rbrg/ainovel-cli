package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/voocel/agentcore/schema"
	"github.com/voocel/ainovel-cli/domain"
	"github.com/voocel/ainovel-cli/state"
)

// CheckConsistencyTool 对照状态文件检查章节一致性。
// 返回上下文数据和已知约束供 LLM 判断，不做 AI 推理。
type CheckConsistencyTool struct {
	store *state.Store
}

func NewCheckConsistencyTool(store *state.Store) *CheckConsistencyTool {
	return &CheckConsistencyTool{store: store}
}

func (t *CheckConsistencyTool) Name() string { return "check_consistency" }
func (t *CheckConsistencyTool) Description() string {
	return "检查章节一致性。返回章节内容、全部状态数据和具体检查清单，你需要逐项对照并以 JSON 格式返回冲突项"
}
func (t *CheckConsistencyTool) Label() string { return "一致性检查" }

func (t *CheckConsistencyTool) Schema() map[string]any {
	return schema.Object(
		schema.Property("chapter", schema.Int("要检查的章节号")).Required(),
	)
}

func (t *CheckConsistencyTool) Execute(_ context.Context, args json.RawMessage) (json.RawMessage, error) {
	var a struct {
		Chapter int `json:"chapter"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("invalid args: %w", err)
	}
	if a.Chapter <= 0 {
		return nil, fmt.Errorf("chapter must be > 0")
	}

	result := map[string]any{"chapter": a.Chapter}

	// 加载章节内容（polished 优先）
	content, wordCount, err := t.store.LoadChapterContent(a.Chapter)
	if err != nil {
		return nil, fmt.Errorf("load chapter content: %w", err)
	}
	if content == "" {
		return nil, fmt.Errorf("no content found for chapter %d", a.Chapter)
	}
	result["content"] = content
	result["word_count"] = wordCount

	// 加载全部状态数据供 LLM 对照
	if timeline, _ := t.store.LoadTimeline(); len(timeline) > 0 {
		result["timeline"] = timeline
	}
	if foreshadow, _ := t.store.LoadForeshadowLedger(); len(foreshadow) > 0 {
		result["foreshadow_ledger"] = foreshadow
		if active := filterActive(foreshadow); len(active) > 0 {
			result["unresolved_foreshadow"] = active
		}
	}
	if relationships, _ := t.store.LoadRelationships(); len(relationships) > 0 {
		result["relationships"] = relationships
	}
	if chars, _ := t.store.LoadCharacters(); len(chars) > 0 {
		result["characters"] = chars
	}

	if rules, _ := t.store.LoadWorldRules(); len(rules) > 0 {
		result["world_rules"] = rules
		// 提取边界清单，方便 LLM 逐条对照
		var boundaries []string
		for _, r := range rules {
			if r.Boundary != "" {
				boundaries = append(boundaries, fmt.Sprintf("[%s] %s", r.Category, r.Boundary))
			}
		}
		if len(boundaries) > 0 {
			result["world_rules_boundaries"] = boundaries
		}
	}

	// 加载前两章摘要
	if summaries, _ := t.store.LoadRecentSummaries(a.Chapter, 2); len(summaries) > 0 {
		result["recent_summaries"] = summaries
	}

	result["instruction"] = `请逐项对照以上状态数据检查本章内容，返回 JSON 数组格式的冲突项：
[
  {
    "type": "timeline|foreshadow|relationship|character|world_rules",
    "severity": "error|warning",
    "description": "具体冲突描述",
    "suggestion": "建议修正范围和方式"
  }
]

检查清单：
1. 时间线：本章事件时间是否与已有 timeline 矛盾
2. 伏笔：unresolved_foreshadow 中是否有本章应推进但遗漏的
3. 人物关系：角色互动是否与 relationships 当前状态矛盾
4. 角色一致性：行为是否符合 characters 中的性格和弧线
5. 世界规则：逐条检查 world_rules_boundaries 中的边界约束，本章内容是否违反任何一条

如果没有发现冲突，返回空数组 []。不要返回其他格式。`

	return json.Marshal(result)
}

func filterActive(entries []domain.ForeshadowEntry) []domain.ForeshadowEntry {
	var active []domain.ForeshadowEntry
	for _, e := range entries {
		if e.Status != "resolved" {
			active = append(active, e)
		}
	}
	return active
}
