package state

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/voocel/ainovel-cli/domain"
)

// SaveChapterPlan 保存章节规划到 drafts/{ch}.plan.json。
func (s *Store) SaveChapterPlan(plan domain.ChapterPlan) error {
	return s.writeJSON(fmt.Sprintf("drafts/%02d.plan.json", plan.Chapter), plan)
}

// LoadChapterPlan 读取章节规划。
func (s *Store) LoadChapterPlan(chapter int) (*domain.ChapterPlan, error) {
	var plan domain.ChapterPlan
	if err := s.readJSON(fmt.Sprintf("drafts/%02d.plan.json", chapter), &plan); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return &plan, nil
}

// SaveSceneDraft 保存场景草稿到 drafts/{ch}.scene-{n}.md。
func (s *Store) SaveSceneDraft(draft domain.SceneDraft) error {
	rel := fmt.Sprintf("drafts/%02d.scene-%d.md", draft.Chapter, draft.Scene)
	return s.writeMarkdown(rel, draft.Content)
}

// LoadSceneDrafts 加载指定章节的所有场景草稿，按场景编号排序。
func (s *Store) LoadSceneDrafts(chapter int) ([]domain.SceneDraft, error) {
	pattern := filepath.Join(s.dir, "drafts", fmt.Sprintf("%02d.scene-*.md", chapter))
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	sort.Strings(matches)

	var drafts []domain.SceneDraft
	for _, m := range matches {
		base := filepath.Base(m)
		sceneNum := parseSceneNum(base)
		content, err := os.ReadFile(m)
		if err != nil {
			return nil, fmt.Errorf("read scene draft %s: %w", base, err)
		}
		drafts = append(drafts, domain.SceneDraft{
			Chapter:   chapter,
			Scene:     sceneNum,
			Content:   string(content),
			WordCount: utf8.RuneCountInString(string(content)),
		})
	}
	return drafts, nil
}

// SavePolished 保存打磨后的章节正文到 drafts/{ch}.polished.md。
func (s *Store) SavePolished(chapter int, content string) error {
	return s.writeMarkdown(fmt.Sprintf("drafts/%02d.polished.md", chapter), content)
}

// LoadPolished 读取打磨后的章节正文。不存在时返回空字符串。
func (s *Store) LoadPolished(chapter int) (string, error) {
	data, err := s.readFile(fmt.Sprintf("drafts/%02d.polished.md", chapter))
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// LoadChapterContent 加载章节正文：优先 polished，否则 merge scenes。
func (s *Store) LoadChapterContent(chapter int) (string, int, error) {
	polished, err := s.LoadPolished(chapter)
	if err != nil {
		return "", 0, err
	}
	if polished != "" {
		return polished, utf8.RuneCountInString(polished), nil
	}
	drafts, err := s.LoadSceneDrafts(chapter)
	if err != nil {
		return "", 0, err
	}
	content, wc := domain.MergeScenes(drafts)
	return content, wc, nil
}

// SaveFinalChapter 保存最终章节正文到 chapters/{ch}.md。
func (s *Store) SaveFinalChapter(chapter int, content string) error {
	return s.writeMarkdown(fmt.Sprintf("chapters/%02d.md", chapter), content)
}

// parseSceneNum 从文件名如 "01.scene-2.md" 提取场景编号。
func parseSceneNum(filename string) int {
	// 格式：{ch}.scene-{n}.md
	parts := strings.Split(filename, "scene-")
	if len(parts) < 2 {
		return 0
	}
	numStr := strings.TrimSuffix(parts[1], ".md")
	n, _ := strconv.Atoi(numStr)
	return n
}
