package state

import (
	"fmt"
	"os"
	"time"

	"github.com/voocel/ainovel-cli/domain"
)

// SaveRunMeta 保存运行元信息到 meta/run.json。
func (s *Store) SaveRunMeta(meta domain.RunMeta) error {
	return s.writeJSON("meta/run.json", meta)
}

// LoadRunMeta 读取运行元信息。
func (s *Store) LoadRunMeta() (*domain.RunMeta, error) {
	var meta domain.RunMeta
	if err := s.readJSON("meta/run.json", &meta); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return &meta, nil
}

// InitRunMeta 初始化或更新运行元信息，保留已有的 SteerHistory。
func (s *Store) InitRunMeta(style, model string) error {
	existing, _ := s.LoadRunMeta()
	meta := domain.RunMeta{
		StartedAt: time.Now().Format(time.RFC3339),
		Style:     style,
		Model:     model,
	}
	if existing != nil {
		meta.SteerHistory = existing.SteerHistory
		meta.PendingSteer = existing.PendingSteer
	}
	return s.SaveRunMeta(meta)
}

// AppendSteerEntry 追加用户干预记录到 meta/run.json。
func (s *Store) AppendSteerEntry(entry domain.SteerEntry) error {
	meta, err := s.LoadRunMeta()
	if err != nil {
		return err
	}
	if meta == nil {
		meta = &domain.RunMeta{}
	}
	meta.SteerHistory = append(meta.SteerHistory, entry)
	return s.SaveRunMeta(*meta)
}

// SetPendingSteer 记录未完成的 Steer 指令，用于中断恢复。
func (s *Store) SetPendingSteer(input string) error {
	meta, err := s.LoadRunMeta()
	if err != nil {
		return err
	}
	if meta == nil {
		meta = &domain.RunMeta{}
	}
	meta.PendingSteer = input
	return s.SaveRunMeta(*meta)
}

// ClearPendingSteer 清除已处理的 Steer 指令。
func (s *Store) ClearPendingSteer() error {
	meta, err := s.LoadRunMeta()
	if err != nil {
		return err
	}
	if meta == nil || meta.PendingSteer == "" {
		return nil
	}
	meta.PendingSteer = ""
	return s.SaveRunMeta(*meta)
}

// SaveCheckpoint 保存当前进度快照到 meta/checkpoints/。
func (s *Store) SaveCheckpoint(label string) error {
	p, err := s.LoadProgress()
	if err != nil || p == nil {
		return err
	}
	ts := time.Now().Format("20060102-150405")
	rel := fmt.Sprintf("meta/checkpoints/%s-%s.json", ts, label)
	return s.writeJSON(rel, p)
}
