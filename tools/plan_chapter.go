package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/voocel/agentcore/schema"
	"github.com/voocel/ainovel-cli/domain"
	"github.com/voocel/ainovel-cli/state"
)

// PlanChapterTool 生成章节规划。
type PlanChapterTool struct {
	store *state.Store
}

func NewPlanChapterTool(store *state.Store) *PlanChapterTool {
	return &PlanChapterTool{store: store}
}

func (t *PlanChapterTool) Name() string { return "plan_chapter" }
func (t *PlanChapterTool) Description() string {
	return "创建章节写作规划，包括目标、冲突、场景拆分和钩子设计。必须在 write_scene 之前调用"
}
func (t *PlanChapterTool) Label() string { return "规划章节" }

func (t *PlanChapterTool) Schema() map[string]any {
	sceneSchema := schema.Object(
		schema.Property("index", schema.Int("场景编号，从 1 开始")).Required(),
		schema.Property("summary", schema.String("场景概要")).Required(),
		schema.Property("pov", schema.String("视角人物")),
		schema.Property("location", schema.String("场景地点")),
	)
	return schema.Object(
		schema.Property("chapter", schema.Int("章节号")).Required(),
		schema.Property("title", schema.String("章节标题")).Required(),
		schema.Property("goal", schema.String("本章目标")).Required(),
		schema.Property("conflict", schema.String("核心冲突")).Required(),
		schema.Property("scenes", schema.Array("场景列表", sceneSchema)).Required(),
		schema.Property("hook", schema.String("章末钩子")).Required(),
		schema.Property("emotion_arc", schema.String("情绪曲线")),
	)
}

func (t *PlanChapterTool) Execute(_ context.Context, args json.RawMessage) (json.RawMessage, error) {
	var plan domain.ChapterPlan
	if err := json.Unmarshal(args, &plan); err != nil {
		return nil, fmt.Errorf("invalid args: %w", err)
	}
	if plan.Chapter <= 0 {
		return nil, fmt.Errorf("chapter must be > 0")
	}
	if len(plan.Scenes) == 0 {
		return nil, fmt.Errorf("scenes must not be empty")
	}

	if err := t.store.SaveChapterPlan(plan); err != nil {
		return nil, fmt.Errorf("save chapter plan: %w", err)
	}

	return json.Marshal(map[string]any{
		"planned":     true,
		"chapter":     plan.Chapter,
		"scene_count": len(plan.Scenes),
	})
}
