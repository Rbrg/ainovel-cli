package main

import (
	"embed"
	"fmt"
	"os"
	"strings"

	"github.com/voocel/ainovel-cli/app"
	"github.com/voocel/ainovel-cli/tools"
)

//go:embed prompts/*.md
var promptsFS embed.FS

//go:embed references
var referencesFS embed.FS

//go:embed styles/*.md
var stylesFS embed.FS

func main() {
	prompt := parsePrompt()
	if prompt == "" {
		fmt.Fprintf(os.Stderr, "用法: novel <小说需求描述>\n")
		fmt.Fprintf(os.Stderr, "示例: novel \"写一部3章的都市悬疑短篇，讲述一个程序员在深夜收到神秘代码后卷入一场阴谋\"\n")
		os.Exit(1)
	}

	style := envOr("NOVEL_STYLE", "default")
	refs := loadReferences(style)
	prompts := loadPrompts()
	styles := loadStyles()

	cfg := app.Config{
		Prompt:    prompt,
		NovelName: "novel",
		APIKey:    os.Getenv("OPENAI_API_KEY"),
		BaseURL:   os.Getenv("OPENAI_BASE_URL"),
		ModelName: envOr("MODEL_NAME", ""),
		Style:     style,
	}
	if v := os.Getenv("MAX_CHAPTERS"); v != "" {
		fmt.Sscanf(v, "%d", &cfg.MaxChapters)
	}

	if err := app.Run(cfg, refs, prompts, styles); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func parsePrompt() string {
	if len(os.Args) < 2 {
		return ""
	}
	return strings.Join(os.Args[1:], " ")
}

func loadReferences(style string) tools.References {
	refs := tools.References{
		ChapterGuide:      mustRead(referencesFS, "references/chapter-guide.md"),
		HookTechniques:    mustRead(referencesFS, "references/hook-techniques.md"),
		QualityChecklist:  mustRead(referencesFS, "references/quality-checklist.md"),
		OutlineTemplate:   mustRead(referencesFS, "references/outline-template.md"),
		CharacterTemplate: mustRead(referencesFS, "references/character-template.md"),
		ChapterTemplate:   mustRead(referencesFS, "references/chapter-template.md"),
		Consistency:       mustRead(referencesFS, "references/consistency.md"),
		ContentExpansion:  mustRead(referencesFS, "references/content-expansion.md"),
		DialogueWriting:   mustRead(referencesFS, "references/dialogue-writing.md"),
	}
	// 加载风格补充参考（可选）
	if style != "" && style != "default" {
		path := "references/" + style + "/style-references.md"
		if data, err := referencesFS.ReadFile(path); err == nil {
			refs.StyleReference = string(data)
		}
	}
	return refs
}

func loadPrompts() app.Prompts {
	return app.Prompts{
		Coordinator: mustRead(promptsFS, "prompts/coordinator.md"),
		Architect:   mustRead(promptsFS, "prompts/architect.md"),
		Writer:      mustRead(promptsFS, "prompts/writer.md"),
		Editor:      mustRead(promptsFS, "prompts/editor.md"),
	}
}

func mustRead(fs embed.FS, path string) string {
	data, err := fs.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("embed read %s: %v", path, err))
	}
	return string(data)
}

func loadStyles() map[string]string {
	styles := make(map[string]string)
	entries, err := stylesFS.ReadDir("styles")
	if err != nil {
		return styles
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".md")
		data, err := stylesFS.ReadFile("styles/" + e.Name())
		if err != nil {
			continue
		}
		styles[name] = string(data)
	}
	return styles
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
