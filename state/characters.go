package state

import (
	"fmt"
	"os"
	"strings"

	"github.com/voocel/ainovel-cli/domain"
)

// SaveCharacters 同时保存 characters.json 和 characters.md。
func (s *Store) SaveCharacters(chars []domain.Character) error {
	if err := s.writeJSON("characters.json", chars); err != nil {
		return err
	}
	return s.writeMarkdown("characters.md", renderCharacters(chars))
}

// LoadCharacters 从 characters.json 读取角色档案。
func (s *Store) LoadCharacters() ([]domain.Character, error) {
	var chars []domain.Character
	if err := s.readJSON("characters.json", &chars); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return chars, nil
}

func renderCharacters(chars []domain.Character) string {
	var b strings.Builder
	b.WriteString("# 角色档案\n\n")
	for _, c := range chars {
		fmt.Fprintf(&b, "## %s（%s）\n\n", c.Name, c.Role)
		fmt.Fprintf(&b, "%s\n\n", c.Description)
		if c.Arc != "" {
			fmt.Fprintf(&b, "**角色弧线**：%s\n\n", c.Arc)
		}
		if len(c.Traits) > 0 {
			fmt.Fprintf(&b, "**特征**：%s\n\n", strings.Join(c.Traits, "、"))
		}
	}
	return b.String()
}
