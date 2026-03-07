package app

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/voocel/agentcore"
	"github.com/voocel/agentcore/llm"
	"github.com/voocel/ainovel-cli/domain"
	"github.com/voocel/ainovel-cli/state"
	"github.com/voocel/ainovel-cli/tools"
)

// Run 启动小说创作流程。
func Run(cfg Config, refs tools.References, prompts Prompts, styles map[string]string) error {
	cfg.FillDefaults()
	if err := cfg.Validate(); err != nil {
		return err
	}

	// 1. 初始化状态
	store := state.NewStore(cfg.OutputDir)
	if err := store.Init(); err != nil {
		return fmt.Errorf("init store: %w", err)
	}

	// 2. 创建模型
	var baseURL []string
	if cfg.BaseURL != "" {
		baseURL = append(baseURL, cfg.BaseURL)
	}
	model, err := llm.NewOpenAIModel(cfg.ModelName, cfg.APIKey, baseURL...)
	if err != nil {
		return fmt.Errorf("create model: %w", err)
	}

	// 3. 组装 Coordinator
	coordinator := BuildCoordinator(cfg, store, model, refs, prompts, styles)

	// 4. 确定性控制面：事件监听 + FollowUp 注入
	coordinator.Subscribe(func(ev agentcore.Event) {
		switch ev.Type {
		case agentcore.EventToolExecStart:
			log.Printf("[tool:start] %s", ev.Tool)

		case agentcore.EventToolExecEnd:
			if ev.IsError {
				log.Printf("[tool:error] %s", ev.Tool)
				return
			}
			log.Printf("[tool:done] %s → %s", ev.Tool, truncateLog(string(ev.Result), 200))

			// 宿主确定性控制：SubAgent 完成后读取信号文件
			if ev.Tool == "subagent" {
				handleSubAgentDone(coordinator, store, cfg.MaxChapters)
				handleEditorDone(coordinator, store)
			}

		case agentcore.EventMessageEnd:
			if ev.Message != nil && ev.Message.GetRole() == agentcore.RoleAssistant {
				log.Printf("[assistant] %s", truncateLog(ev.Message.TextContent(), 300))
			}

		case agentcore.EventError:
			log.Printf("[error] %v", ev.Err)
		}
	})

	// 5. 初始化运行元信息（保留已有 SteerHistory）
	if err := store.InitRunMeta(cfg.Style, cfg.ModelName); err != nil {
		log.Printf("[warn] 初始化运行元信息失败: %v", err)
	}

	// 6. Steer 协程：stdin 读取用户干预
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			text := strings.TrimSpace(scanner.Text())
			if text == "" {
				continue
			}
			log.Printf("[steer] 用户干预: %s", text)
			if err := store.AppendSteerEntry(domain.SteerEntry{
				Input:     text,
				Timestamp: time.Now().Format(time.RFC3339),
			}); err != nil {
				log.Printf("[warn] 追加干预记录失败: %v", err)
			}
			if err := store.SetPendingSteer(text); err != nil {
				log.Printf("[warn] 设置待处理干预失败: %v", err)
			}
			if err := store.SetFlow(domain.FlowSteering); err != nil {
				log.Printf("[warn] 设置流程状态失败: %v", err)
			}
			coordinator.Steer(agentcore.UserMsg(fmt.Sprintf(
				"[用户干预] %s\n请评估影响范围，决定是否需要修改设定或重写已有章节。", text)))
		}
	}()

	// 7. 恢复或启动（按优先级链）
	progress, _ := store.LoadProgress()
	runMeta, _ := store.LoadRunMeta()
	if progress != nil && progress.InProgressChapter > 0 {
		// 场景级恢复：章节写到一半
		ch := progress.InProgressChapter
		scenes := len(progress.CompletedScenes)
		log.Printf("场景级恢复：第 %d 章已完成 %d 个场景", ch, scenes)
		if err := coordinator.Prompt(fmt.Sprintf(
			"第 %d 章正在进行中，已完成 %d 个场景。请调用 writer 从场景 %d 继续写作。总共需要写 %d 章。",
			ch, scenes, scenes+1, progress.TotalChapters,
		)); err != nil {
			return fmt.Errorf("prompt: %w", err)
		}
	} else if progress != nil && len(progress.PendingRewrites) > 0 {
		// 重写恢复：有待重写章节
		log.Printf("重写恢复：%d 章待处理 %v", len(progress.PendingRewrites), progress.PendingRewrites)
		verb := "重写"
		if progress.Flow == domain.FlowPolishing {
			verb = "打磨"
		}
		if err := coordinator.Prompt(fmt.Sprintf(
			"有 %d 章待%s（受影响章节：%v）。原因：%s。请逐章调用 writer %s后继续正常写作。总共需要写 %d 章。",
			len(progress.PendingRewrites), verb, progress.PendingRewrites, progress.RewriteReason, verb, progress.TotalChapters,
		)); err != nil {
			return fmt.Errorf("prompt: %w", err)
		}
	} else if progress != nil && progress.Flow == domain.FlowReviewing {
		// 审阅恢复：审阅中断
		log.Printf("审阅恢复：上次审阅中断")
		if err := coordinator.Prompt(fmt.Sprintf(
			"上次审阅中断，请重新调用 editor 对已完成章节进行全局审阅。已完成 %d 章，共 %d 字。总共需要写 %d 章。",
			len(progress.CompletedChapters), progress.TotalWordCount, progress.TotalChapters,
		)); err != nil {
			return fmt.Errorf("prompt: %w", err)
		}
	} else if progress != nil && progress.IsResumable() && runMeta != nil && runMeta.PendingSteer != "" {
		next := progress.NextChapter()
		log.Printf("Steer 恢复：上次干预未完成，重新注入")
		if err := coordinator.Prompt(fmt.Sprintf(
			"从第 %d 章继续写作。之前已完成 %d 章，共 %d 字。总共需要写 %d 章。\n\n[用户干预-恢复] %s\n请评估影响范围，决定是否需要修改设定或重写已有章节。",
			next, len(progress.CompletedChapters), progress.TotalWordCount, progress.TotalChapters, runMeta.PendingSteer,
		)); err != nil {
			return fmt.Errorf("prompt: %w", err)
		}
	} else if progress != nil && progress.IsResumable() {
		next := progress.NextChapter()
		log.Printf("恢复模式：从第 %d 章继续（已完成 %d 章，共 %d 字）",
			next, len(progress.CompletedChapters), progress.TotalWordCount)
		if err := coordinator.Prompt(fmt.Sprintf(
			"从第 %d 章继续写作。之前已完成 %d 章，共 %d 字。总共需要写 %d 章。",
			next, len(progress.CompletedChapters), progress.TotalWordCount, progress.TotalChapters,
		)); err != nil {
			return fmt.Errorf("prompt: %w", err)
		}
	} else {
		// 新建：初始化进度
		if err := store.InitProgress(cfg.NovelName, cfg.MaxChapters); err != nil {
			return fmt.Errorf("init progress: %w", err)
		}
		log.Printf("新建模式：%s（%d 章）", cfg.NovelName, cfg.MaxChapters)
		if err := coordinator.Prompt(fmt.Sprintf(
			"请创作一部 %d 章的小说。要求如下：\n\n%s",
			cfg.MaxChapters, cfg.Prompt,
		)); err != nil {
			return fmt.Errorf("prompt: %w", err)
		}
	}

	// 6. 等待完成
	coordinator.WaitForIdle()
	finalizeSteerIfIdle(store)

	// 7. 输出结果
	finalProgress, _ := store.LoadProgress()
	if finalProgress != nil {
		log.Printf("创作完成：%d 章，共 %d 字，输出目录：%s",
			len(finalProgress.CompletedChapters), finalProgress.TotalWordCount, store.Dir())
	}
	return nil
}

// handleSubAgentDone 在每次 SubAgent 调用完成后读取文件系统信号，注入确定性任务。
// SubAgent 内部工具事件不冒泡，所以通过 meta/last_commit.json 传递信号。
func handleSubAgentDone(coordinator *agentcore.Agent, store *state.Store, maxChapters int) {
	result, err := store.LoadLastCommit()
	if err != nil || result == nil {
		return // 不是 Writer 的 commit，可能是 Architect 的 SubAgent 调用
	}
	// 消费即清除，防止重复注入 FollowUp
	if err := store.ClearLastCommit(); err != nil {
		log.Printf("[host] 清除 commit 信号失败: %v", err)
	}

	log.Printf("[host] 章节提交信号：第 %d 章，%d 字，%d 个场景",
		result.Chapter, result.WordCount, result.SceneCount)

	// 确定性判断 0：正在重写/打磨流程中
	progress, _ := store.LoadProgress()
	if progress != nil && (progress.Flow == domain.FlowRewriting || progress.Flow == domain.FlowPolishing) {
		if !slices.Contains(progress.PendingRewrites, result.Chapter) {
			log.Printf("[host] 警告：重写期间提交了非队列章节 %d，拒绝并提醒", result.Chapter)
			coordinator.FollowUp(agentcore.UserMsg(fmt.Sprintf(
				"[系统] 当前处于重写流程，但提交了非队列章节（第 %d 章）。请先完成待重写章节 %v 后再继续新章节。",
				result.Chapter, progress.PendingRewrites)))
			return
		}
		if err := store.CompleteRewrite(result.Chapter); err != nil {
			log.Printf("[host] 完成重写标记失败: %v", err)
		}
		clearHandledSteer(store)
		updated, _ := store.LoadProgress()
		if updated != nil && len(updated.PendingRewrites) == 0 {
			log.Printf("[host] 所有重写/打磨已完成，恢复正常写作")
			if err := store.SaveCheckpoint(fmt.Sprintf("ch%02d-commit", result.Chapter)); err != nil {
				log.Printf("[host] 保存检查点失败: %v", err)
			}
			if err := store.SaveCheckpoint("rewrite-done"); err != nil {
				log.Printf("[host] 保存检查点失败: %v", err)
			}
		} else if updated != nil {
			log.Printf("[host] 还有 %d 章待处理：%v", len(updated.PendingRewrites), updated.PendingRewrites)
			if err := store.SaveCheckpoint(fmt.Sprintf("ch%02d-commit", result.Chapter)); err != nil {
				log.Printf("[host] 保存检查点失败: %v", err)
			}
		}
		return // 重写期间不触发全书完成/审阅判断
	}

	// 确定性判断 1：全书完成
	if result.NextChapter > maxChapters {
		log.Printf("[host] 所有 %d 章已完成，注入完成指令", maxChapters)
		if err := store.MarkComplete(); err != nil {
			log.Printf("[host] 标记完成失败: %v", err)
		}
		clearHandledSteer(store)
		if err := store.SaveCheckpoint(fmt.Sprintf("ch%02d-commit", result.Chapter)); err != nil {
			log.Printf("[host] 保存检查点失败: %v", err)
		}
		coordinator.FollowUp(agentcore.UserMsg(fmt.Sprintf(
			"[系统] 全部 %d 章已写完。请总结全书并结束。不要再调用 writer。",
			maxChapters)))
		return
	}

	// 确定性判断 2：需要全局审阅
	if result.ReviewRequired {
		log.Printf("[host] review_required=true（%s），注入审阅指令", result.ReviewReason)
		if err := store.SetFlow(domain.FlowReviewing); err != nil {
			log.Printf("[host] 设置审阅流程失败: %v", err)
		}
		coordinator.FollowUp(agentcore.UserMsg(fmt.Sprintf(
			"[系统] review_required=true，%s。请调用 editor 对已完成章节进行全局审阅，然后根据审阅结果决定继续写第 %d 章还是修正已有章节。",
			result.ReviewReason, result.NextChapter)))
	}
	clearHandledSteer(store)
	if err := store.SaveCheckpoint(fmt.Sprintf("ch%02d-commit", result.Chapter)); err != nil {
		log.Printf("[host] 保存检查点失败: %v", err)
	}
}

// handleEditorDone 在 Editor SubAgent 完成后读取审阅信号。
func handleEditorDone(coordinator *agentcore.Agent, store *state.Store) {
	review, err := store.LoadLastReviewSignal()
	if err != nil {
		log.Printf("[host] 加载审阅信号失败: %v", err)
		return
	}
	if review == nil {
		return
	}
	// 消费即清除，防止重复注入 FollowUp
	if err := store.ClearLastReview(); err != nil {
		log.Printf("[host] 清除审阅信号失败: %v", err)
	}

	log.Printf("[host] 审阅信号：verdict=%s，%d 个问题", review.Verdict, len(review.Issues))

	chaptersInfo := ""
	if len(review.AffectedChapters) > 0 {
		chaptersInfo = fmt.Sprintf("受影响章节：%v。", review.AffectedChapters)
	}

	switch review.Verdict {
	case "rewrite":
		if err := store.SetPendingRewrites(review.AffectedChapters, review.Summary); err != nil {
			log.Printf("[host] 设置重写队列失败: %v", err)
		}
		if err := store.SetFlow(domain.FlowRewriting); err != nil {
			log.Printf("[host] 设置流程状态失败: %v", err)
		}
		coordinator.FollowUp(agentcore.UserMsg(fmt.Sprintf(
			"[系统] Editor 审阅结论：rewrite。%s%s请逐章调用 writer 重写受影响章节，全部完成后继续正常写作。",
			review.Summary, chaptersInfo)))
	case "polish":
		if err := store.SetPendingRewrites(review.AffectedChapters, review.Summary); err != nil {
			log.Printf("[host] 设置打磨队列失败: %v", err)
		}
		if err := store.SetFlow(domain.FlowPolishing); err != nil {
			log.Printf("[host] 设置流程状态失败: %v", err)
		}
		coordinator.FollowUp(agentcore.UserMsg(fmt.Sprintf(
			"[系统] Editor 审阅结论：polish。%s%s请逐章调用 writer 打磨受影响章节，全部完成后继续正常写作。",
			review.Summary, chaptersInfo)))
	default:
		// accept — 审阅通过，清除审阅状态
		if err := store.SetFlow(domain.FlowWriting); err != nil {
			log.Printf("[host] 清除审阅状态失败: %v", err)
		}
	}
	clearHandledSteer(store)
	if err := store.SaveCheckpoint(fmt.Sprintf("review-ch%02d-%s", review.Chapter, review.Verdict)); err != nil {
		log.Printf("[host] 保存检查点失败: %v", err)
	}
}

func truncateLog(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "..."
}

func clearHandledSteer(store *state.Store) {
	if err := store.ClearPendingSteer(); err != nil {
		log.Printf("[host] 清除待处理干预失败: %v", err)
	}
	progress, _ := store.LoadProgress()
	if progress != nil && progress.Flow == domain.FlowSteering {
		if err := store.SetFlow(domain.FlowWriting); err != nil {
			log.Printf("[host] 重置流程状态失败: %v", err)
		}
	}
}

func finalizeSteerIfIdle(store *state.Store) {
	runMeta, _ := store.LoadRunMeta()
	progress, _ := store.LoadProgress()
	if runMeta == nil || runMeta.PendingSteer == "" || progress == nil {
		return
	}
	if progress.Flow != domain.FlowSteering {
		return
	}
	clearHandledSteer(store)
}
