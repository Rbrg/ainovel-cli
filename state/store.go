package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Store 封装小说输出目录，提供所有状态读写操作。
type Store struct {
	dir string
}

// NewStore 创建状态管理器，dir 为小说输出根目录。
func NewStore(dir string) *Store {
	return &Store{dir: dir}
}

// Dir 返回输出根目录。
func (s *Store) Dir() string { return s.dir }

// Init 创建所需的子目录结构。
func (s *Store) Init() error {
	dirs := []string{"chapters", "summaries", "drafts", "reviews", "meta"}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(s.dir, d), 0o755); err != nil {
			return fmt.Errorf("create dir %s: %w", d, err)
		}
	}
	return nil
}

func (s *Store) path(rel string) string {
	return filepath.Join(s.dir, rel)
}

func (s *Store) readFile(rel string) ([]byte, error) {
	return os.ReadFile(s.path(rel))
}

func (s *Store) writeFile(rel string, data []byte) error {
	p := s.path(rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}

func (s *Store) readJSON(rel string, v any) error {
	data, err := s.readFile(rel)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

func (s *Store) writeJSON(rel string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return s.writeFile(rel, data)
}

func (s *Store) writeMarkdown(rel string, content string) error {
	return s.writeFile(rel, []byte(content))
}

func (s *Store) removeFile(rel string) error {
	err := os.Remove(s.path(rel))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
