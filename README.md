# ainovel-cli

全自动 AI 长篇小说创作引擎。基于多智能体协作架构，从一句话需求到完整小说，全程无需人工干预。

## 特性

- **多智能体协作** — Coordinator 调度 Architect / Writer / Editor 三个专职智能体，各司其职
- **确定性控制面** — 宿主程序通过信号文件驱动流程，不依赖 LLM 判断控制流
- **场景级断点恢复** — 中断后从上次写到的场景精确续写，不丢失进度
- **自适应上下文策略** — 根据总章节数自动切换全量 / 滑窗 / 分层摘要，支持 500+ 章长篇
- **六维质量评审** — Editor 从设定一致性、角色行为、节奏、叙事连贯、伏笔、钩子六个维度评审
- **用户实时干预** — 写作过程中可随时注入修改意见，系统自动评估影响范围并重写
- **双模式运行** — CLI 一行命令直接跑，TUI 交互界面实时观察进度
- **多 LLM 支持** — OpenRouter / Anthropic / Gemini / OpenAI 随意切换

## 架构

```
┌─────────────────────────────────────────────────┐
│                   Host（控制面）                  │
│  读取信号文件 → 确定性决策 → 注入 FollowUp 指令      │
└────────────┬────────────────────────┬───────────┘
             │                        │
     ┌───────▼───────┐      ┌────────▼────────┐
     │  Coordinator  │◄────►│   State Store   │
     │  （调度中枢）   │      │  （JSON 持久化）  │
     └──┬────┬────┬──┘      └─────────────────┘
        │    │    │
   ┌────▼┐ ┌▼───┐ ┌▼─────┐
   │Arch.│ │Wri.│ │Edit. │
   │建筑师│ │作家 │ │编辑  │
   └─────┘ └────┘ └──────┘
```

### 智能体职责

| 智能体 | 职责 | 工具 |
|--------|------|------|
| **Coordinator** | 调度全局，处理评审裁定和用户干预 | `subagent` `novel_context` `ask_user` |
| **Architect** | 生成前提、大纲、角色档案、世界规则 | `novel_context` `save_foundation` |
| **Writer** | 逐场景写作 → 打磨 → 一致性检查 → 提交 | `novel_context` `plan_chapter` `write_scene` `polish_chapter` `check_consistency` `commit_chapter` |
| **Editor** | 跨章节六维评审，弧/卷级摘要生成 | `novel_context` `save_review` `save_arc_summary` `save_volume_summary` |

### 写作流水线

```
用户需求 → Architect 建基 → Writer 逐章写作 → Editor 评审
                                    ↑                │
                                    └── 重写/打磨 ◄───┘
```

每章写作严格按序执行：

1. `novel_context` — 加载上下文（前情摘要、时间线、伏笔、角色状态）
2. `plan_chapter` — 规划 3-5 个场景
3. `write_scene` × N — 逐场景创作（800-1500 字/场景）
4. `polish_chapter` — 合并打磨，去除 AI 腔
5. `check_consistency` — 校验时间线、角色、世界规则
6. `commit_chapter` — 提交终稿，更新全局状态

### 长篇分层架构

500+ 章小说采用三级结构自动管理上下文：

```
卷（Volume）
└── 弧（Arc）
    └── 章（Chapter）
        └── 场景（Scene）
```

- **卷摘要** — 压缩整卷为一段话，供后续卷参考
- **弧摘要 + 角色快照** — 弧结束时自动生成，追踪角色状态演变
- **章摘要** — 滑窗加载最近 3 章，远处靠弧/卷摘要覆盖
- **弧边界检测** — 自动识别弧/卷结束，触发对应评审和摘要生成

## 快速开始

```bash
# 安装
go install github.com/voocel/ainovel-cli@latest

# 配置 API Key（任选一个 Provider）
export LLM_PROVIDER=openrouter
export OPENROUTER_API_KEY=sk-xxx

# CLI 模式：一行启动
ainovel-cli "写一部12章都市悬疑小说，主角是刑警，暗线是家族秘密"

# TUI 模式：交互界面
ainovel-cli
```

### 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `LLM_PROVIDER` | LLM 提供商 | `openrouter` |
| `OPENROUTER_API_KEY` | OpenRouter API Key | — |
| `ANTHROPIC_API_KEY` | Anthropic API Key | — |
| `GEMINI_API_KEY` | Gemini API Key | — |
| `NOVEL_STYLE` | 写作风格 | `default` |

### 写作风格

通过 `NOVEL_STYLE` 环境变量切换：

- `default` — 通用风格
- `suspense` — 悬疑推理
- `fantasy` — 奇幻仙侠
- `romance` — 言情

## 输出结构

```
output/{novel_name}/
├── chapters/           # 终稿（Markdown）
│   ├── 01.md
│   └── ...
├── summaries/          # 章节摘要（JSON）
├── drafts/             # 场景草稿
├── reviews/            # 评审报告
├── meta/
│   ├── premise.md      # 故事前提
│   ├── outline.json    # 章节大纲
│   ├── characters.json # 角色档案
│   ├── world_rules.json# 世界规则
│   ├── progress.json   # 进度状态
│   ├── timeline.json   # 时间线
│   ├── foreshadow.json # 伏笔台账
│   └── snapshots/      # 角色状态快照（长篇）
└── characters.md       # 角色档案（可读版）
```

## 设计理念

### 全自动闭环

一句话输入，完整小说输出，中间零人工干预。系统自主完成全部创作决策：

```
"写一部悬疑小说" → 构建世界观 → 设计角色 → 规划大纲
                → 逐章写作 → 质量评审 → 自动重写
                → 弧级摘要 → 角色快照 → 完整成书
```

**自主决策能力：**

- **Architect 自主构建** — 从用户一句话需求推导出完整的前提、大纲、角色关系和世界规则
- **Writer 自主创作** — 每章独立完成规划、写作、打磨、一致性校验的完整闭环
- **Editor 自主评审** — 跨章节分析结构问题，输出裁定（通过 / 打磨 / 重写）及影响范围
- **Coordinator 自主调度** — 根据评审裁定安排重写，根据弧边界触发摘要生成，无需外部指令
- **自动伏笔管理** — 埋设、推进、回收全程由 Agent 自行追踪，不会烂尾
- **自动节奏调控** — 追踪叙事线和钩子类型历史，避免连续章节结构雷同

### 确定性控制面

Agent 负责创造，Host 负责兜底。**控制流不交给 LLM 判断**。

Writer 调用 `commit_chapter` 后，宿主程序读取信号文件 `meta/last_commit.json`，确定性地决定下一步：

| 信号 | 宿主动作 |
|------|----------|
| 全部章节完成 | 标记完成，通知 Coordinator 总结全书 |
| `review_required=true` | 注入 Editor 评审指令 |
| `arc_end=true` | 注入弧级评审 + 弧摘要生成指令 |
| `volume_end=true` | 额外注入卷摘要生成指令 |
| 有待重写章节 | 注入重写指令 |
| 以上皆否 | 注入"继续写下一章"指令 |

Editor 评审裁定同理：`accept` → 继续，`polish/rewrite` → 注入修改指令。

这种设计保证：即使 LLM 幻觉或遗忘，宿主层的状态机也能把流程拉回正轨。

## 技术栈

- **Go 1.25** — 主语言
- **[agentcore](https://github.com/voocel/agentcore)** — 多智能体编排框架（tool-calling + streaming）
- **[litellm](https://github.com/voocel/litellm)** — 统一 LLM 接口适配
- **[Bubble Tea](https://github.com/charmbracelet/bubbletea)** — 终端 TUI 框架
- **[Lip Gloss](https://github.com/charmbracelet/lipgloss)** — 终端样式

## License

MIT
