package domain

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// MergeScenes 将多个场景草稿按顺序合并为完整章节正文。
// 返回合并后的正文和总字数（按 rune 计）。
func MergeScenes(scenes []SceneDraft) (string, int) {
	var b strings.Builder
	for i, s := range scenes {
		if i > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(s.Content)
	}
	content := b.String()
	return content, utf8.RuneCountInString(content)
}

// ReviewInterval 全局审阅间隔（每 N 章触发一次）。
const ReviewInterval = 5

// ShouldReview 根据已完成章节数判断是否需要全局审阅。
func ShouldReview(completedCount int) (bool, string) {
	if completedCount > 0 && completedCount%ReviewInterval == 0 {
		return true, fmt.Sprintf("已完成 %d 章，触发全局审阅", completedCount)
	}
	return false, ""
}
