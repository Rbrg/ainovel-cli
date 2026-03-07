package state

import (
	"fmt"
	"os"

	"github.com/voocel/ainovel-cli/domain"
)

// SaveSummary 保存章节摘要到 summaries/{ch}.json。
func (s *Store) SaveSummary(sum domain.ChapterSummary) error {
	return s.writeJSON(fmt.Sprintf("summaries/%02d.json", sum.Chapter), sum)
}

// LoadSummary 读取指定章节的摘要。
func (s *Store) LoadSummary(chapter int) (*domain.ChapterSummary, error) {
	var sum domain.ChapterSummary
	if err := s.readJSON(fmt.Sprintf("summaries/%02d.json", chapter), &sum); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return &sum, nil
}

// LoadRecentSummaries 加载 current 章之前最近 count 章的摘要。
func (s *Store) LoadRecentSummaries(current, count int) ([]domain.ChapterSummary, error) {
	var result []domain.ChapterSummary
	start := max(current-count, 1)
	for ch := start; ch < current; ch++ {
		sum, err := s.LoadSummary(ch)
		if err != nil {
			return nil, err
		}
		if sum != nil {
			result = append(result, *sum)
		}
	}
	return result, nil
}
