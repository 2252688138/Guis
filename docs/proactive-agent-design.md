# 主动办事能力设计（Proactive Agent）

> 目标读者：`cmd/deepseek` 现有 Agent 运行时（`agent` / `schedule` / `pages` / `tools`）的维护者。
> 本文所有接口都对齐仓库现有实现（`agent.Tool`、`agent.Env`、`agent.MemoryStore`、`agent.SkillRegistry`、`agent.MCPManager`、`schedule.Task`、`agent.PermMode`），可直接落地为 `cmd/deepseek/proactive` 包 + 少量 `agent` 侧包装。

---

## 0. 一页结论

- 新能力不是一个"更聪明的提示词"，而是一个**常驻后台的闭环服务**：`感知（采集信号）→ 洞察（记忆健康/偏好/项目卡）→ 提案（候选任务评分）→ 闸门（风险分级）→ 执行（受限工具集 + 子 Agent）→ 审计与回滚 → 反哺记忆`。
- 三个硬边界必须写进代码而不是文档：**(1) 无审批不越权；(2) 无备份不写入；(3) 无审计不执行**。
- MVP 只做"只读 + 报告 + 提议"，把"能自动改代码"放到第 2 阶段之后；先证明**提案质量**（被采纳率），再放开**执行半径**。
- 复用度预估：`schedule`（触发）、`MemoryStore`（自检对象）、`SkillRegistry`（技能落盘）、`MCPManager`（工具接入）、`SubAgentManager` + `Env`（受限执行）、`file_cache` 备份（回滚）——新代码只增加编排层与判定逻辑，约 3500~4500 行 Go。

---

## 1. 总体架构

### 1.1 分层与模块清单

```
┌──────────────────────────── 触发层 Trigger ────────────────────────────┐
│ 定时(schedule.Task: cron/daily/every) · 事件(会话结束/任务失败/记忆写入) │
│ 变更(schedule 的 after 任务 + git 工作区指纹) · 手动(/proactive 或按钮)   │
└────────────────────────────────┬───────────────────────────────────────┘
                                 ▼
┌──────────────────────────── 感知层 Perception ─────────────────────────┐
│ S1 SignalCollector 信号采集（git/构建/测试/日志/TODO/issue/会话索引）    │
│ S2 MemoryReader    读记忆（MemoryStore.ListEntries / FullText / FTS）    │
│ S3 WorkspaceScan   工作区扫描（目录树、go.mod、docs、符号表、mtime）     │
└────────────────────────────────┬───────────────────────────────────────┘
                                 ▼
┌──────────────────────────── 洞察层 Insight ────────────────────────────┐
│ I1 MemoryAuditor    记忆健康（冲突/过期/重复/缺字段/低置信/孤儿）        │
│ I2 PrefProfiler     用户偏好画像（来源/置信度/半衰期/冲突）              │
│ I3 ProjectSummarizer 项目卡（结构/命令/未完成事项/已知破坏点/近期变更）  │
└────────────────────────────────┬───────────────────────────────────────┘
                                 ▼
┌──────────────────────────── 决策层 Decision ───────────────────────────┐
│ D1 CandidateGenerator 候选任务生成（11 类来源，带证据）                  │
│ D2 Scorer             六维评分（价值/紧急/风险/成本/依赖/可回滚）        │
│ D3 RiskGate  + Budget 分级（L0 自动 / L1 建议 / L2 审批 / L3 禁止）      │
│                       防打扰（配额、静默、去重、抑制、批量摘要）         │
└────────────────────────────────┬───────────────────────────────────────┘
                                 ▼
┌──────────────────────────── 执行层 Execution ──────────────────────────┐
│ E1 Inbox        提案收件箱（UI 卡片 / IM 简报 / 持久化 JSON）            │
│ E2 Executor     受限执行（白名单工具 + 子 Agent + 沙箱 + 超时）          │
│ E3 Verifier     产物验证（go build / go vet / go test / 自定义命令）     │
│ E4 CapabilityBuilder 能力缺口 → 新工具 / 新技能 / 新 MCP                 │
└────────────────────────────────┬───────────────────────────────────────┘
                                 ▼
┌──────────────────────────── 治理层 Governance ──────────────────────────┐
│ G1 Auditor   审计 JSONL（触发者/依据/工具调用/影响面/结果/回滚点）       │
│ G2 Rollback  三级回滚（文件备份 / git stash / 能力卸载）                 │
│ G3 Metrics   采纳率、打扰率、失败率、回滚率、能力增长曲线                │
└────────────────────────────────────────────────────────────────────────┘
```

### 1.2 模块输入 / 输出 / 触发 / 集成点

| ID | 模块 | 输入 | 输出 | 触发条件 | 与现有系统的集成点 |
|----|------|------|------|----------|--------------------|
| S1 | SignalCollector | 工作区路径、`git status/log`、`go build/test` 结果、`JobRegistry` 失败记录、会话索引 | `Signals{BuildErr, TestFail, TODOs, ChangedFiles, FailedJobs, StaleDocs}` | 每 10 分钟 / 会话结束 / 任务失败事件 / 手动 | `agent.Env.Workspace`、`agent.JobRegistry`、`agent.NewJSONStore` 会话索引、`run_command` 同款执行器 |
| S2 | MemoryReader | `MemoryStore` 三作用域（global/project/session） | `[]MemoryEntry` + FTS 命中 | 随 I1 | `agent.NewMemoryStore(projectRoot, globalRoot, sessionID)` |
| S3 | WorkspaceScan | 目录树、`go list ./...`、`go.mod`、`docs/*.md`、导出符号 | `WorkspaceIndex`（文件→包→符号→mtime→hash） | 首次全量 + 增量（mtime/hash） | `file_operation` 的 list/glob/search 同源逻辑 |
| I1 | MemoryAuditor | 记忆条目 + WorkspaceIndex | `HealthReport`（问题清单 + 修复建议） | 每日一次 + 记忆写入计数达阈值 + 手动 | `memory` 工具 `save` 之后打脏标记；报告落到 `.zym/proactive/` |
| I2 | PrefProfiler | 审批历史（用户点同意/拒绝）、设置项、会话行为、技能使用、拒绝记录 | `PrefProfile`（键值 + 置信度 + 证据） | 会话结束事件 + 每日汇总 | `agent.Approver` 包装、`pages` 设置存储、`Env.Perm` 变化 |
| I3 | ProjectSummarizer | WorkspaceIndex、README/docs、`TODO/FIXME`、git log、构建/测试状态、issue 文件 | `ProjectCard`（结构化 YAML/JSON） | 每 24h / 提交后 / 手动 | 与 `memory` 的 project 作用域互补（卡片是派生视图，可重算） |
| D1 | CandidateGenerator | Signals + ProjectCard + PrefProfile + HealthReport + `GoalState` + `TodoItem` | `[]Candidate` | 每次 Tick | `agent.GoalState`（create_goal）、`Env.SetTodo` 的待办 |
| D2 | Scorer | Candidate + Profile（价值权重） | `ScoredCandidate{Score, Dims}` | D1 之后 | 复用 `Env` 中的用户偏好权重 |
| D3 | RiskGate / Budget | ScoredCandidate + 配额状态 | `Decision{Auto\|Suggest\|Approve\|Drop\|Suppress}` | D2 之后 | 映射到 `agent.PermMode`（ReadOnly/Ask/Full） |
| E1 | Inbox | Decision | 提案条目 + 简报文本 | D3 产出时 / 定时摘要 | `pages` 侧栏卡片、`schedule` 会话轮次、`bot`（QQ/微信）推送 |
| E2 | Executor | 已批准的 Proposal + capability 集合 | `RunRecord` | 用户批准 / L0 自动 | `agent.SubAgentManager`、受限 `agent.Env`（Perm=PermAsk，工具白名单） |
| E3 | Verifier | RunRecord 产物（diff/新文件） | 验证结果（通过/失败/超时） | E2 结束 | `run_command`（go build/vet/test） |
| E4 | CapabilityBuilder | GapTicket（重复失败/重复脚本/重复解释） | 新技能目录 / 新工具 Go 文件 / 新 MCP 配置（staged） | 缺口累计达阈值 或 用户指定 | `SkillRegistry`、`PluginManager`、`MCPManager`、`ReloadSkills` |
| G1 | Auditor | 全链路事件 | `.zym/proactive/audit/YYYY-MM.jsonl` | 每个阶段 | `agent` 会话日志之外独立存储，永不随会话清理 |
| G2 | Rollback | RunRecord 的回滚点 | 恢复结果 | 用户点回滚 / 验证失败 / 灰度指标劣化 | `file_cache` 备份目录、`git stash`、MCP/插件卸载 |
| G3 | Metrics | 审计流 | 周报 + 仪表盘数值 | 每日 | 与 `HealthReport` 合并成一份"Agent 自检周报" |

### 1.3 数据流（一次 Tick 的完整链路）

```
Trigger(cause)
   │
   ├─► S1 采集 ─┐
   ├─► S2 读记忆 ┤
   └─► S3 扫描 ─┘
                ▼
        I1/I2/I3 产出（增量：只重算 hash 变化的片段）
                ▼
        D1 候选（每条候选必须带 ≥1 条 evidence 指针）
                ▼
        D2 评分（0..1）+ 风险分类（L0..L3）
                ▼
        D3 闸门 + 配额 + 去重指纹 + 静默时段
                ├── Drop      → 归档，不进 Inbox
                ├── Suppress  → 记为抑制（拒绝历史/重复），到期自动解禁
                ├── Auto(L0)  → E2 执行（仅报告类，写入一律降级为 Suggest）
                └── Suggest/Approve → E1 Inbox → 用户决定 → E2
                ▼
        E2 执行（子 Agent + 白名单 + 超时 + 备份点）
                ▼
        E3 验证（build/vet/test/自定义断言）
                ├── 通过 → G1 审计 (result=ok) → 反哺 I2（偏好证据）、D 层统计
                └── 失败 → G2 自动回滚 → G1 审计 (result=failed)
                ▼
        G3 指标；I3 项目卡消费 API 的记忆（项目事实回流 memory 的 project 作用域）
```

### 1.4 状态机（Proposal 生命周期）

```
draft ──score──► scored ──gate──► queued ──brief──► briefed
                    │                │                 │
                    │                └──expire(72h)──► expired
                    ├──drop──► archived
                    └──suppress──► suppressed ──(TTL 到)──► archived
                                                      
briefed ──accept──► approved ──start──► running ──► verifying
   │                                              │        │
   └──reject──► rejected(+抑制指纹)      fail◄────┘        ok
                                              │            │
                                        rolled_back      done
```

能力创建走另一条状态机：

```
gap_detected → spec_drafted → sandbox_impl → tested → reviewed
   → registered(staged) → canary(3d/50 次) → promoted
                          └── 指标劣化/用户否决 ──► unregistered(保留审计与备份)
```

### 1.5 与现有 Agent 的集成点（代码级）

| 位置 | 改动 |
|------|------|
| `cmd/deepseek/proactive/` | 新增包：编排引擎、采集器、评分器、闸门、Inbox、执行器、审计、回滚、能力工厂 |
| `cmd/deepseek/agent/proactive_tools.go` | 新增 6 个 `agent.Tool`：`proactive_status`、`proactive_cycle`、`proactive_inbox`、`proactive_decide`、`capability_propose`、`capability_apply` |
| `cmd/deepseek/agent/tool.go` (`Env`) | 追加字段 `Proactive *proactive.Engine`（nil 表示未启用，工具集自动隐藏） |
| `cmd/deepseek/agent/memory.go` | `SaveEntry` 成功后调用 `Proactive.NoteMemoryDirty()`（失败静默，不阻断保存） |
| `cmd/deepseek/runhub.go` | `finishRun` 时投递 `EventSessionEnd`（已有 `SessionStopped` 逻辑处顺带） |
| `cmd/deepseek/jobs.go` | 任务结束状态为 failed 时投递 `EventJobFailed` |
| `cmd/deepseek/pages/` | 侧栏新增"Inbox / 自检"卡片：展示简报、单条提案的一键接受/拒绝/回滚 |
| `cmd/deepseek/schedule/` | 复用 `Task{Kind: KindCron}` 注册 3 个任务：`mem-health`(每日 03:10)、`project-card`(每日 04:10)、`digest`(每日 09:00 与 18:00) |

---

## 2. 记忆自检机制

### 2.1 触发条件（三条通道，全部幂等去抖）

| 通道 | 触发 | 频率上限 | 动作范围 |
|------|------|----------|----------|
| 定时 | `schedule.Task{Kind: KindCron, Expr: "10 3 * * *"}` | 1 次/天 | 全量六类检查 + 生成健康报告 |
| 事件 | `memory.SaveEntry` 成功计数每满 20 条；`EventSessionEnd` 且该会话有 ≥3 次保存 | ≤3 次/天 | 增量检查（只查新建/改写条目） |
| 手动 | 用户 `/proactive health` 或 UI 按钮、`proactive_cycle(check="memory")` | 无（去抖 60s） | 指定作用域检查 |

去抖规则：同一 `(scope, fingerprint)` 24h 内只产出一条建议；报告文件按日期覆盖写。

### 2.2 六类缺陷的判定算法（可编码，无需 LLM 也能跑）

设条目 `e = {Scope, Title, Keywords, Content, UpdatedAt}`，`norm(s)` = 去空白/小写/全角转半角。

| 类型 | 判定（规则层，先跑；LLM 只在规则不确定时兜底） | 输出动作 |
|------|----------------------------------------------|----------|
| **重复** | `Jaccard(shingles(norm(content)), 0.85)` 或 `keywords 集合相等且 BM25 top1 命中自身` | 低风险：自动合并（保留 `UpdatedAt` 新者，旧内容进"历史"段） |
| **同名冲突** | 同 `(scope, title)` 存在历史版本，且新内容和旧内容在"取值位"上不同（正则抽取数字/路径/命令/版本号/布尔词表：`用/不用`、`必须/禁止`、`开/关`） | 中风险：出冲突卡，二选一 + 保留旧值到历史 |
| **过期** | 内容引用的路径/符号不存在（`os.Stat`、`go list` 符号表）；或 `now-UpdatedAt > TTL` 且近期信号中未再出现该主题；TTL：global 偏好 180d、project 事实 365d、session 7d | 低风险：标记 `stale:true` 并从注入上下文降权（不删） |
| **缺失字段** | `len(Keywords) < 2`、`len([]rune(Content)) < 20`、`Title` 为空或 >20 字、`Scope` 非法 | 低风险：机器人补全（从 Content 抽关键词）并回写 |
| **低置信** | 记忆来源标记为一次性会话；或被用户纠正 ≥2 次（Inbox 拒绝 + 纠正话术）；或 `Evidence` 为空 | 中风险：降权（注入时排序后置），连续 2 次自检仍低置信 → 建议删除 |
| **孤儿/失效引用** | 内容引用了已删除文件、已重命名包、已下线 MCP server、已卸载技能 | 低风险：生成"引用修复"建议，不自动改写内容 |

语义相反判断（"同名冲突"的加强规则）用计数词表 + 数值差异，不依赖 LLM；LLM 仅在 `0.6 ≤ 相似度 ≤ 0.85` 的灰区做一次性裁决（结果写回审计，避免重复付费）。

### 2.3 用户偏好画像（PrefProfile）

```go
// proactive/prefs.go
type Preference struct {
	Key        string    `json:"key"`        // 如 pref.output_style / pref.approval_scope / pref.quiet_hours
	Value      string    `json:"value"`
	Confidence float64   `json:"confidence"` // 0..1
	Evidence   []string  `json:"evidence"`   // 证据指针：session:12#turn3 / inbox:proposal_xxx#reject
	UpdatedAt  int64     `json:"updatedAt"`
	Source     string    `json:"source"`     // explicit(用户明说) / behavior(行为统计) / inferred(模型推断)
	Pending    []string  `json:"pending,omitempty"` // 冲突候选值，等用户裁决
}
```

- **来源与权重**：`explicit`（权重 1.0，一次性置高置信 0.9）> `behavior`（0.5）> `inferred`（0.2，上限置信 0.6，永不自动执行高风险动作）。
- **置信度更新**（在线、可解释）：
  `c ← clamp(c + α·(obs − c), 0, 1)`，`α = 0.25`（behavior）/ `0.5`（explicit）；`obs ∈ {0,1}` 来自观察是否与当前值一致。
- **时间衰减**：`c ← c · exp(−Δd / 90)`（90 天半衰常量；explicit 不衰减）。
- **冲突处理**：
  1. 新证据产生不同 `Value` 且 `|c_new − c_old| < 0.15` → 不覆盖，写入 `Pending`，在简报里问一次（`ask_user_question`）。
  2. 差异 ≥ 0.15 → 高置信者生效，旧值进 `history`（保留 3 条）。
  3. `explicit` 永远压过 `behavior`，即使置信度较低（用户明确说过的就是规则）。
- **冷启动**：无画像时按"保守默认"：只在 09:00–21:00 推送、每日最多 3 条建议、不自动写入、输出语言跟随会话语言。
- **画像可读可改**：`pref.*` 全部落在 `.zym/proactive/profile.json`，用户可直接编辑；文件被手改 → `source=explicit, confidence=1`。

初始画像键表（MVP 只做前 8 个）：

| Key | 取值示例 | 采集方式 |
|-----|----------|----------|
| `pref.language` | `zh-CN` | 会话检测 |
| `pref.output_style` | `concise` / `detailed` | 用户"太长/说人话"等纠正语料 |
| `pref.approval_scope` | `ask` / `readonly` / `full` | `Env.Perm` 选择分布 |
| `pref.auto_accept` | `report_only` | 历史接受率 |
| `pref.quiet_hours` | `22:00-08:00` | 用户设置 / 交互时间分布 |
| `pref.daily_budget` | `3` | 用户设置 |
| `pref.focus_dirs` | `cmd/deepseek,ui` | 近期编辑目录分布 |
| `pref.verify_cmd` | `go test ./cmd/deepseek/...` | 从会话中提取的验证命令 |

### 2.4 项目功能总结（ProjectCard）

采集源与提取方式：

| 维度 | 采集 | 产物字段 |
|------|------|----------|
| 结构 | `go list ./...`、目录树（跳过 `.git/.zym/dist/bin`） | `packages[]`、`entrypoints[]` |
| 文档 | `docs/*.md`、`README*`、导出符号注释率 | `docs{gaps[]}`（有符号无注释 / 有包无 doc） |
| TODO | `rg -n "TODO|FIXME|XXX|HACK"` 按文件聚合 | `todos[]{file,line,text,age}` |
| 变更 | `git log --since=7d --oneline`、`git status --porcelain`、`git diff --stat` | `recent[]`、`dirty[]` |
| 健康 | `go build ./...`（排除已知破坏包）、`go vet`、`go test` 摘要 | `build{ok,brokenPkgs[]}`、`tests{fail[]}` |
| 未完成 | `GoalState`、会话 `TodoItem` 中 `pending/in_progress` | `openItems[]` |
| 日志 | `jobs` 失败记录、`.zym/proactive/audit` 近期失败 | `anomalies[]` |

缓存策略：每个维度独立 `hash(输入)`，只重算变化的维度；卡片本身持久化到 `.zym/proactive/project-card.json`，注入模型时只给**摘要**（≤400 字）而非全文。

> 已知破坏包 `cmd/smokewebview`（`os` 未使用导致 `go build ./...` 失败）应作为 `build.brokenPkgs` 的**基线**写入卡片：基线内的失败不生成提案（避免每次 Tick 重复提议），基线外的失败 → 立即生成高优先候选。

### 2.5 输出：记忆健康报告与更新建议

```json
{
  "generatedAt": 1763517600,
  "scopeCounts": {"global": 12, "project": 148, "session": 6},
  "issues": [
    {"type": "duplicate", "entries": ["proj#41", "proj#57"], "sim": 0.91,
     "action": "merge", "risk": "L0", "autoApplied": true},
    {"type": "stale", "entry": "proj#88", "reason": "引用路径 cmd/smokewebview/main.go 已修复", "action": "mark_stale"},
    {"type": "conflict", "entry": "global#3", "candidates": ["22:00-08:00", "23:00-07:00"], "action": "ask_user"}
  ],
  "health": {"duplicateRate": 0.04, "staleRate": 0.07, "missingFieldRate": 0.02,
             "conflictRate": 0.01, "lowConfidenceRate": 0.05, "avgConfidence": 0.78}
}
```

- 落盘：`.zym/proactive/reports/memory-health-YYYYMMDD.md`（人读）+ 同名 `.json`（机读）。
- 自动应用范围严格限定：**去重合并（保留历史段）、补关键词、标记 stale**——三类都不改变事实内容。
- 凡是**改内容 / 删条目 / 改 scope** → 一律进 Inbox 等用户点确认。

---

## 3. 主动任务发现与执行

### 3.1 候选来源（11 类，每类必须给出可点击的证据）

| # | 来源 | 提取信号 | 典型候选 |
|---|------|----------|----------|
| C1 | 代码 TODO/FIXME | `rg` 命中 + 文件年龄 | "补 `agents/loop.go:214` 的 TODO：重试上限未实现" |
| C2 | 测试缺口 | 包内无 `_test.go` / 覆盖率 < 阈值 | "为 `proactive/score.go` 补表驱动测试" |
| C3 | 文档缺口 | 导出符号无注释、docs 目录无对应章节 | "补 `MCPManager.Health` 的 doc 注释" |
| C4 | 构建/静态检查 | `go build/vet` 基线外失败 | "修复 `cmd/x/main.go` 未使用 import" |
| C5 | 日志异常 | `jobs` 失败、审计近 7 天错误率上升 | "run_command 超时率 12% → 排查长任务" |
| C6 | 近期变更缺口 | 改了代码未改测试/文档/CHANGELOG | "`ui/layout.go` 变更未同步文档" |
| C7 | 记忆待办 | `GoalState` 非完成、`TodoItem` pending | "上一会话目标未收尾：httpc 重试文档" |
| C8 | 记忆健康 | I1 报告中的中风险项 | "3 条端口记忆互相冲突，需裁决" |
| C9 | 项目约束违反 | 卡片里记录的约定（如"必须排除 smokewebview"）被新代码违反 | "新增 CI 未排除已破坏包" |
| C10 | issue/待办文件 | 工作区 `TODO.md`、`.github/ISSUE_TEMPLATE`、issue 导出 | "完成 #42 的迁移" |
| C11 | 偏好驱动 | 重复出现的流程（用户第三次手工让我跑同一命令） | "把 3 步流程固化成技能" |

候选结构：

```go
// proactive/candidate.go
type Candidate struct {
	ID        string   `json:"id"`        // sha1(source|path|symbol|kind)[:12]
	Source    string   `json:"source"`    // C1..C11
	Kind      string   `json:"kind"`      // fix_bug / add_test / add_doc / refactor / cleanup / capability / decision
	Title     string   `json:"title"`
	Rationale string   `json:"rationale"` // 面向用户的一句话理由
	Evidence  []string `json:"evidence"`  // 文件:行、审计 id、记忆条目 id、命令输出片段
	Scope     []string `json:"scope"`     // 预计触及的文件/目录（用于风险评估与冲突检测）
	EstTokens int      `json:"estTokens"`
	EstMins   int      `json:"estMins"`
	Deps      []string `json:"deps,omitempty"` // 依赖的其他候选 / 需先满足的条件
	CreatedAt int64    `json:"createdAt"`
}
```

### 3.2 六维评分

```go
// proactive/score.go
type Dims struct {
	Value       float64 // 用户价值 0..1
	Urgency     float64 // 紧急度 0..1
	Risk        float64 // 风险 0..1（越高越危险）
	Cost        float64 // 成本 0..1（token/时间/副作用面）
	Dependency  float64 // 依赖就绪度 0..1（1=无依赖）
	Reversible  float64 // 可回滚性 0..1
}

func Score(d Dims) float64 {
	return 0.30*d.Value + 0.20*d.Urgency + 0.15*(1-d.Risk) +
		0.15*(1-d.Cost) + 0.10*d.Dependency + 0.10*d.Reversible
}
```

各维度取值规则（规则表优先，LLM 不参与打分——保证可复现、可审计）：

- `Value`：命中用户显式目标（Goal/Todo）0.9；修复构建/测试中断 0.85；文档/测试缺口 0.5；纯整洁 0.3；用户历史拒绝过的同类 0.1。
- `Urgency`：`min(1, base + 0.1·ageDays)`，`base`：构建中断 0.9、CI 红 0.85、数据/安全 0.8、普通改进 0.3；72h 未处理按 0.05/天衰减（陈旧提案自动过期）。
- `Risk`：见 3.3 分级表的基础分。
- `Cost`：`min(1, scopeFiles/8)` 与 `estTokens/60k` 取大者。
- `Dependency`：依赖已满足=1；依赖其他候选=0.5；依赖用户输入=0.2。
- `Reversible`：只读 1.0；新建文件 0.95；改文件（有备份）0.8；删除 0.4；外部副作用（网络/发布）0.1。

### 3.3 分级与执行策略

| 级别 | 判定 | 例子 | 执行策略 |
|------|------|------|----------|
| **L0** | `Risk ≤ 0.25` 且纯只读/纯报告 | 记忆健康报告、项目卡、TODO 汇总、`go vet` 结果整理 | **自动执行**，结果进简报，不弹窗；任何写入一律降级为 L1 |
| **L1** | `Risk ∈ (0.25, 0.55]`，写操作但**在工作区内且可回滚**（≤3 文件、≤120 行、有备份） | 补 doc 注释、补测试文件、修 lint、补 SKILL.md、记忆合并确认 | **先建议后执行**：一键"接受并执行"卡片，默认不自动跑 |
| **L2** | `Risk ∈ (0.55, 0.8]` 或触及危险面 | 删除/重命名、改 `go.mod`、装依赖、改 CI 配置、改权限设置、外部网络写操作 | **必须审批**：展示 diff 预览 + 影响面 + 回滚方式，二次确认（首次该类操作要求输入确认词） |
| **L3** | 触碰不可逆 / 敏感面 | 凭证与密钥、`git push --force`、发布/部署、清空记忆、批量删除 | **禁止自动，且不生成提案**；只在简报中作为"人工事项"列出 |

闸门伪代码：

```go
func (g *RiskGate) Decide(c ScoredCandidate) Decision {
	if c.Risk >= 0.8 || inForbidden(c.Scope) {
		return Decision{Mode: ModeListOnly, Reason: "L3：仅列人工事项"}
	}
	if c.Risk <= 0.25 && c.Writes == 0 && g.budget.AutoOK(c) {
		return Decision{Mode: ModeAuto}
	}
	if c.Risk > 0.55 || touchesConfig(c.Scope) {
		return Decision{Mode: ModeApprove, NeedDouble: true}
	}
	if g.budget.CanSuggest(c) {
		return Decision{Mode: ModeSuggest}
	}
	return Decision{Mode: ModeDrop, Reason: "预算耗尽"}
}
```

执行侧强约束（不依赖提示词）：

```go
// proactive/executor.go
type CapabilitySet struct {
	Tools      []string      // 白名单工具名（agent.Tool.Spec().Name）
	Perm       agent.PermMode
	MaxFiles   int           // 允许写出的文件数上限
	MaxLines   int
	Timeout    time.Duration
	AllowNet   bool
	AllowExec  bool          // 是否允许 run_command
	SandboxDir string        // 相对工作区，默认 .zym/proactive/sandbox/<runID>
}
```

`Executor.Run` 通过 `agent.SubAgentManager` 派遣子 Agent，构造专用 `agent.Env`：`Perm = PermAsk`（L1）/用户已批准时临时提升为 `PermFull` 但工具集裁剪；`Approve` 替换为"自动拒绝 + 记录"回调——即**任何子 Agent 试图越界都会失败而不是静默获批**。

### 3.4 防打扰策略

| 机制 | 规则 | 实现 |
|------|------|------|
| 每日配额 | 推送 ≤ `pref.daily_budget`（默认 3）；L0 不进推送只进摘要 | `Budget` 计数器持久化到 `.zym/proactive/budget.json`，跨重启 |
| 静默时段 | `pref.quiet_hours`（默认 22:00–08:00）内只落 Inbox 不推送 | 推送前检查，积累到窗口结束合并 |
| 批量汇报 | 每天 2 次摘要（09:00/18:00），单条即时推送仅限 `Urgency ≥ 0.85` | `schedule.Task{Kind: KindDaily}` |
| 去重 | `ID = sha1(source|path|symbol|kind)[:12]`；同 ID 7 天内不重复提议 | Inbox 索引 |
| 拒绝抑制 | 单条拒绝 → 同 ID 抑制 14d；同类（`source+kind`）累计拒绝 2 次 → 抑制 90d 并降 `Value` 0.35 | `suppress.json` |
| 陈旧过期 | `queued` 超 72h 未处理 → `expired`，不再出现在摘要 | 状态机 |
| 冲突检测 | 与正在执行的 run 触及相同 `Scope` → 排队，不并发 | Scope 锁 |
| 打扰反馈 | 用户点"别再提这类" → 永久 `suppress(source+kind)` 并写入偏好 `pref.suppressed` | 偏好 + 抑制表 |

### 3.5 一次主动执行的最小闭环（时序）

```
Tick → Candidate(C4 修复未使用 import, Risk 0.3, Writes 1)
     → Gate: L1 Suggest → Inbox
     → UI 卡片：「修复 cmd/x/main.go 未使用 import？预计 1 文件 / 2 行 / 可回滚」
     → 用户点接受
     → Executor: 备份文件 → 子 Agent 受限执行（工具白名单：read/edit/run_command(test)）
     → Verifier: go build ./cmd/x && go test ./cmd/x
     → 通过：审计 result=ok，偏好 +1 证据（用户接受门槛），摘要里标记"已完成"
     → 失败：自动回滚（file_cache 还原），审计 result=failed + 错误摘要，抑制同 ID 3d
```

---

## 4. 主动创建工具 / 技能 / MCP 工具

### 4.1 能力缺口识别（什么时候才该造东西）

| 信号 | 阈值 | 判定 | 产出形态 |
|------|------|------|----------|
| 同一意图回合内重复失败 | 同一 `(intent, tool)` 失败 ≥2 次且错误同类 | 现有工具能力不足 | `GapTicket{kind: tool}` |
| 同类 shell/HTTP 操作手工重复 | 7 天内同模板 ≥3 次（命令归一化后指纹相同） | 值得固化为工具 | `GapTicket{kind: tool}`（脚本→工具） |
| 用户重复解释同一流程 | 同一流程描述相似度 ≥0.8 出现 3 次 | 值得固化为技能 | `GapTicket{kind: skill}` |
| 反复手写长提示词/步骤清单 | 会话中出现 ≥2 次 8 步以上清单 | 技能（SKILL.md） | `GapTicket{kind: skill}` |
| 需要外部系统且无对应 server | 用户要求访问某系统 API，且 `MCPManager` 无匹配 | MCP 接入 | `GapTicket{kind: mcp}` |
| 现有工具参数表达不了（连续 3 次因 schema 缺字段失败） | 结构性问题 | 工具改造或新工具 | `GapTicket{kind: tool, mode: extend}` |
| 一次性任务 | 只出现 1 次 | **不造**，直接执行 | — |

决策树（先判"要不要"，再判"造哪种"）：

```
可复用？ ── 否 → 直接执行（一次性）
   │是
   ├─ 纯知识/流程/提示词，无确定性 I/O → 技能（SKILL.md，零代码，最易回滚）
   ├─ 需要确定性输入输出/校验/超时/重试 → 工具（Go ToolFunc / PluginHTTPAction / 脚本工具）
   └─ 需要跨进程或第三方系统、独立生命周期 → MCP server（配置 + 适配器）
```

### 4.2 创建流水线（10 步，每步有产物和门禁）

| 步 | 动作 | 产物 | 门禁（不满足则退回） |
|----|------|------|----------------------|
| 1 | 需求描述：从 GapTicket 生成 `CapSpec` | `capability/<id>/spec.md`（用途、非目标、入参出参、失败模式） | 必须写明"为什么不复用现有工具" |
| 2 | 接口设计 | `schema.json`（JSON Schema）+ 命名 + 权限声明 | `agent.ValidSkillName` 风格校验；参数 ≤6 个；错误码表齐备 |
| 3 | 沙箱实现 | `.zym/proactive/sandbox/<id>/` 下的实现（Go 片段 / 脚本 / SKILL.md） | 只允许写沙箱；禁止 `go get` |
| 4 | 单元测试 | `*_test.go` 或 `test.sh`（happy + 参数错误 + 超时 + 边界） | 覆盖率 ≥70%；全部通过 |
| 5 | 安全审查 | `security-review.md`（见 4.4 清单逐项打勾） | 任一项失败 → 拒绝注册 |
| 6 | 注册（staged） | 工具：插件目录 `plugins/<id>/` + `manifest.json`；技能：`.deepseek/skills/<id>/SKILL.md`；MCP：`mcp_servers.json` 追加 `enabled:false` | 校验和写入 manifest；默认 staged |
| 7 | 文档 | `SKILL.md` 或工具 doc + `.zym/proactive/capabilities.md` 登记行 | 必须含"何时不要用" |
| 8 | 灰度 | `scope=session` 或 `enabled` 仅对发起会话；shadow 模式（只记录调用不下发模型）3 天或 50 次 | 期间失败率 >20% → 自动退回 |
| 9 | 监控 | `CapMetric{calls, errors, p50/p95, rejected}` 写审计 | 每日评估，进入周报 |
| 10 | 回滚/提升 | 达标 → `promoted`（全局启用）；劣化 → `unregistered` | 回滚必须 ≤60s 完成且保留审计 |

### 4.3 工具/技能规范（写进校验器，不靠自觉）

| 维度 | 规则 | 校验点 |
|------|------|--------|
| 命名 | 工具 `动词_名词`（snake，≤32）、技能 kebab（`^[a-z0-9][a-z0-9-]{0,63}$`，复用 `agent.ValidSkillName`）、MCP server `kebab-id` | 注册时 |
| 入参/出参 | JSON Schema draft-07；`required` 明确；出参统一 `{"ok":bool,"data":{},"error":{"code","message","retryable"}}` | 校验器 |
| 错误处理 | 必须区分 `invalid_args / not_found / timeout / permission_denied / upstream_error`；不得把堆栈回灌模型 | 单测断言 |
| 权限 | `manifest.perms = {read:[...], write:[...], net:[...], exec:bool}`，注册时与 `agent.PermMode` 取交集 | 注册时求交，运行时二次校验 |
| 超时 | 默认 30s，可配；超时必须释放资源（context） | 单测用 `-timeout` 场景 |
| 幂等 | 有副作用的工具必须接受 `idempotency_key`，重复调用返回同结果 | 单测两次调用 |
| 日志 | 结构化、脱敏（键名含 key/token/password/secret 一律 `<redacted>`）、带 `run_id` | 日志扫描单测 |
| 版本 | `semver` + `apiVersion:1`；破坏性变更必须新 id | manifest 校验 |
| 描述 | ≤300 字，含"何时用/何时不用/示例" | 长度与关键词检查 |

### 4.4 安全审查清单（工具/技能/MCP 通用）

1. 密钥与凭证：源码/配置/日志中零明文；引用走环境变量或 `.zym/secrets`（不入库）。
2. 路径：所有文件操作必须在工作区内（复用 `agent` 的路径沙箱），禁止 `..` 逃逸与绝对路径写入。
3. 命令执行：白名单命令；禁止 `rm -rf`、`format`、`curl|sh`、`powershell -enc`、`git push --force`。
4. 网络：默认无网络；需要时声明域名白名单（不允许通配 `*`）。
5. 依赖：只允许标准库或已在 `go.mod` 的模块；**禁止静默安装**（`go get`/`npm i`/`pip install` 必须人工批准并记录）。
6. 数据外泄：出参不得包含工作区路径之外的敏感内容；日志不落原始凭证。
7. 权限最小化：`perms` 只声明实际用到的；只读能力不得声明写权限。
8. 可回滚：注册前记录原状（配置片段、目录快照），卸载脚本必须存在。
9. 提示注入：技能正文不得包含"忽略之前的指令"类内容；来自外部的 MCP 工具描述需原样保留并标注来源。
10. 速率：单工具 ≤10 次/分钟、≤1000 次/天（防自激循环）。

### 4.5 MCP 工具接入

配置示例（追加到现有 `mcp_servers.json` 结构，字段与 `agent.MCPServerConfig` 对齐）：

```json
{
  "id": "local-kb",
  "label": "本地知识库",
  "transport": "stdio",
  "command": "node",
  "args": ["C:/tools/kb-mcp/index.js"],
  "env": { "KB_ROOT": "${WORKSPACE}" },
  "enabled": false,
  "staged": true,
  "timeoutMs": 30000,
  "perms": { "read": ["docs/**"], "write": [], "net": [], "exec": false },
  "allowTools": ["kb_search", "kb_read"],
  "denyTools": ["kb_delete", "kb_bulk_import"],
  "healthCheck": { "tool": "kb_search", "args": { "q": "ping" }, "everyMs": 300000, "failThreshold": 3 }
}
```

- **健康检查**：启动时 1 次 + 每 5 分钟 1 次轻量调用；连续 3 次失败 → 熔断（`disabled_by_health`），从模型工具表中移除，并在摘要里告知"能力暂时缺失"，不静默失败。
- **失败降级**：超时 → 重试 1 次（幂等工具）→ 仍失败则该 server 本轮禁用；影响面记录到审计；若该 server 是唯一来源，提案"改用替代方案/安装本地工具"。
- **审计记录**：每次 MCP 调用记 `{server, tool, args_hash, durationMs, ok, error_code}`（args 只存 hash 与脱敏摘要，防泄密）。
- **卸载/回滚**：`enabled=false` → 清理工具缓存与适配器注册 → 保留配置与审计 30 天 → 可一键恢复；删除配置需二次确认。

### 4.6 明确禁止事项（硬约束，代码级拦截）

| 禁止 | 落地手段 |
|------|----------|
| 绕过权限 | 所有副作用工具必经 `NeedsApproval`；`CapabilitySet.Tools` 是白名单（非黑名单），清单外工具直接 `permission_denied` |
| 静默安装高风险依赖 | `run_command` 中出现 `go get/npm i/pip install` 且无 `approved=true` → 拒绝并提案 |
| 泄露密钥 | 日志/审计/descriptor 统一过 `redact()`；注册前扫描 `sk-`、`AKIA`、`-----BEGIN`、`password=` 等模式 |
| 未审批的高危操作 | L3 类操作（4.3/3.3）不在可执行集合内，即使模型构造了同名参数也走 `permission_denied` |
| 自我提权 | `CapabilityBuilder` 生成的 manifest 权限若超过用户当前 `PermMode` → 强制人工审批，不允许自动提升 |
| 静默改记忆事实 | 记忆自动动作仅限"去重合并/补字段/标记 stale"，内容改写必须确认 |

---

## 5. 安全、权限与审计

### 5.1 四级权限模型（映射到现有 `agent.PermMode`）

| 主动级别 | 语义 | 对应 `PermMode` | 允许动作 |
|----------|------|-----------------|----------|
| P0 只读 | 采集、分析、出报告 | `PermReadOnly` | read/glob/grep/run_command(只读命令白名单) |
| P1 建议 | 只能产出提案，不能改 | `PermReadOnly` | 同上 + 写 Inbox |
| P2 低风险写入 | 工作区内、有备份、可回滚 | `PermAsk`（自动放行白名单内动作） | edit/write 限 `Scope`，`MaxFiles/MaxLines` 硬上限 |
| P3 高风险 | 危险面或不可逆 | 不做模式映射：**必须人工** | 执行前 `Approve` 拦截，且要求二次确认 |

关键点：**P2 的"自动放行"是白名单 + 配额 + 回滚点三件套同时满足**，缺一即为 P3。

### 5.2 审计日志（JSONL，只追加）

路径：`.zym/proactive/audit/YYYY-MM.jsonl`（按月分片，永不被会话清理删除）。

```json
{
  "ts": 1763517600123,
  "runId": "run_7f3a91",
  "trigger": "cron:mem-health|event:job-failed|user:inbox-accept",
  "actor": "proactive|user:1001",
  "proposalId": "sha1:9c1f4a2b7e",
  "basis": {
    "memory": ["project#41", "global#3"],
    "evidence": ["cmd/deepseek/agent/loop.go:214", "audit:run_19ab"],
    "profile": {"pref.auto_accept": "report_only"},
    "card": "project-card.json#L120"
  },
  "action": { "kind": "tool_call|file_edit|capability_register|mcp_enable",
              "tool": "run_command", "argsHash": "sha1:ab12", "summary": "go build ./cmd/deepseek" },
  "impact": { "files": ["cmd/x/main.go"], "lines": 3, "outsideWorkspace": false },
  "result": { "ok": true, "durationMs": 4210, "verify": "go build ok" },
  "rollback": { "kind": "file_backup", "ref": ".zym/file_cache/cmd/x/main.go_20261118_123010_11.go" },
  "cost": { "tokens": 1840, "usd": 0.0009 }
}
```

必填字段（缺一则该记录视为缺陷，单测校验）：`ts/runId/trigger/actor/proposalId/basis/action/result/rollback`。
红线：`argsHash` 只存哈希；任何可能含密钥的字符串过 `redact()` 后再落盘。

### 5.3 回滚三级

| 级别 | 场景 | 手段 | 时限 |
|------|------|------|------|
| R1 文件级 | 单/少量文件写入 | 复用 `file_cache` 备份（写前自动快照）→ 原样写回 | ≤5s |
| R2 变更集级 | 多文件/跨目录改动 | 执行前 `git stash create` 记录快照 id（不动工作区）→ 失败时 `git checkout <sha> -- <paths>` | ≤30s |
| R3 能力级 | 新工具/技能/MCP | `enabled=false` + 反注册适配器 + 删除/还原配置片段（备份在 `capability/<id>/backup/`） | ≤60s |

自动回滚触发条件：Verifier 失败、执行超时、越权被拒 ≥1 次、灰度指标劣化（失败率 >20% 或用户否决 ≥2 次）。

### 5.4 失败处理与降级矩阵

| 故障 | 检测 | 降级行为 | 用户可见 |
|------|------|----------|----------|
| LLM 不可用/限流 | 调用错误 | 只跑规则层（自检、评分、报告），提案生成暂停 | 摘要标注"只读模式" |
| FTS 索引损坏 | 查询报错 | 线性扫描文本（降速不失败），并触发索引重建 | 无感 |
| 磁盘写满 | 写审计失败 | 停止一切提案与执行，只保留内存态与告警 | 显著告警横幅 |
| 记忆库被删/损坏 | 读失败 | 停止记忆类提案，不清理任何东西 | 提示恢复 |
| MCP server 不健康 | 健康检查 | 该 server 工具下线，能力缺失告知 | 摘要提示 |
| 用户连续拒绝 ≥3 次 | 计数 | 全局降频到 1 条/天，进入"观察期"7 天 | 摘要征询偏好 |
| 子 Agent 反复越权 | 拦截图 | 该 Proposal 立即终止 + 回滚 + 同类抑制 90d | 审计可见 |

---

## 6. 实施计划

### 6.1 目录结构建议

```
cmd/deepseek/proactive/
  engine.go        // Engine：Tick/Start/Stop/Trigger，编排全部阶段
  config.go        // Config 解析（.zym/proactive/config.json）+ 默认值
  signal.go        // SignalCollector：git/构建/测试/TODO/日志/会话
  scan.go          // WorkspaceScan：目录树、包与符号、hash 缓存
  memory_audit.go  // I1 六类检查 + 报告生成
  prefs.go         // I2 偏好画像（更新/衰减/冲突/持久化）
  project.go       // I3 项目卡
  candidate.go     // D1 候选生成（C1..C11）
  score.go         // D2 评分 + 风险分类
  gate.go          // D3 分级 + Budget + Suppress
  inbox.go         // E1 提案存储/去重/摘要
  executor.go      // E2 受限执行（CapabilitySet + 子 Agent）
  verifier.go      // E3 验证
  capability.go    // E4 能力工厂（spec→sandbox→test→review→register→canary）
  capability_gen.go// 模板生成（SKILL.md / ToolFunc / MCP 配置）
  audit.go         // G1 审计 JSONL + redact
  rollback.go      // G2 三级回滚
  metrics.go       // G3 指标与周报
  *_test.go        // 单测（规则表、评分、闸门、去重、回滚）

cmd/deepseek/agent/
  proactive_tools.go  // 6 个 agent.Tool 包装
  proactive_hooks.go  // NoteMemoryDirty / EventSessionEnd / EventJobFailed

cmd/deepseek/pages/
  proactive_panel.go  // Inbox 卡片 + 一键接受/拒绝/回滚

.zym/proactive/        // 运行期数据（全部 gitignore）
  config.json  profile.json  project-card.json  budget.json  suppress.json
  inbox/*.json  reports/  audit/YYYY-MM.jsonl  sandbox/  capability/
```

### 6.2 核心接口（Go，可直接落文件）

```go
// proactive/engine.go
package proactive

type Cause string

const (
	CauseCron        Cause = "cron"
	CauseSessionEnd  Cause = "event:session-end"
	CauseJobFailed   Cause = "event:job-failed"
	CauseMemoryDirty Cause = "event:memory-dirty"
	CauseUser        Cause = "user"
)

// Deps 只注入"能用什么"，不含业务逻辑，便于单测打桩。
type Deps struct {
	Workspace   string
	SessionID   string
	Memory      *agent.MemoryStore
	Skills      *agent.SkillRegistry
	MCP         *agent.MCPManager
	Jobs        *agent.JobRegistry
	Goal        *agent.GoalState
	SubAgents   *agent.SubAgentManager
	Run         func(ctx context.Context, p ExecParams) (ExecResult, error) // 走 Env+子 Agent
	Approver    agent.Approver
	Notify      func(text string, items []Proposal)                          // UI/IM 推送
	Log         *slog.Logger
}

type Engine struct {
	cfg   Config
	deps  Deps
	audit *Auditor
	inbox *Inbox
	gate  *RiskGate
	exec  *Executor
	mu    sync.Mutex
	last  time.Time
}

func New(cfg Config, deps Deps) (*Engine, error)
func (e *Engine) Start(ctx context.Context)                     // 注册 schedule 任务与事件钩子
func (e *Engine) Trigger(ctx context.Context, c Cause)         // 去抖 60s 后异步 Tick
func (e *Engine) Tick(ctx context.Context, c Cause) (*CycleResult, error)
func (e *Engine) Accept(ctx context.Context, id string) (*RunRecord, error)
func (e *Engine) Reject(ctx context.Context, id, reason string) error
func (e *Engine) Rollback(ctx context.Context, runID string) error
func (e *Engine) Status() Status
```

```go
// proactive/capability.go
type GapTicket struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`    // skill | tool | mcp
	Mode    string `json:"mode"`    // new | extend
	Signal  string `json:"signal"`  // repeat_fail | repeat_cmd | repeat_explain | missing_mcp | schema_gap
	Count   int    `json:"count"`
	Intent  string `json:"intent"`
	Evidence []string `json:"evidence"`
}

type CapSpec struct {
	ID      string          `json:"id"`
	Name    string          `json:"name"`
	Kind    string          `json:"kind"`
	Purpose string          `json:"purpose"`
	NonGoals string         `json:"nonGoals"`
	Schema  json.RawMessage `json:"schema"`
	Perms   Perms           `json:"perms"`
	Timeout time.Duration   `json:"timeout"`
	Idempotent bool         `json:"idempotent"`
	Version string          `json:"version"`
	Tests   []string        `json:"tests"`
}

type CapabilityFactory struct { /* deps: sandbox dir, test runner, registry, mcp manager, auditor */ }
func (f *CapabilityFactory) Detect(ctx context.Context) ([]GapTicket, error)
func (f *CapabilityFactory) Draft(ctx context.Context, t GapTicket) (CapSpec, error)
func (f *CapabilityFactory) Build(ctx context.Context, s CapSpec) (BuildResult, error) // 沙箱实现+单测
func (f *CapabilityFactory) Review(ctx context.Context, s CapSpec, b BuildResult) (ReviewResult, error)
func (f *CapabilityFactory) Register(ctx context.Context, s CapSpec, staged bool) error
func (f *CapabilityFactory) Canary(ctx context.Context, id string) (CanaryState, error)
func (f *CapabilityFactory) Promote(ctx context.Context, id string) error
func (f *CapabilityFactory) Unregister(ctx context.Context, id string) error
```

### 6.3 关键伪代码

```go
// Tick：一次完整闭环（幂等、可中断、每阶段独立超时）
func (e *Engine) Tick(ctx context.Context, cause Cause) (*CycleResult, error) {
	e.mu.Lock(); defer e.mu.Unlock()
	if time.Since(e.last) < 60*time.Second && cause == CauseCron { return nil, ErrDebounced }
	e.last = time.Now()

	cycleID := newID()
	budgetLeft := e.gate.Budget().Remaining(time.Now())
	res := &CycleResult{CycleID: cycleID, Cause: cause}

	// 1) 感知（并行，各自 60s 超时；失败只降级不中断）
	var sig Signals; var card ProjectCard; var health HealthReport
	group, gctx := errgroup.WithContext(ctx)
	group.Go(func() error { sig, _ = e.collectSignals(gctx); return nil })
	group.Go(func() error { card, _ = e.summarizer.Get(gctx, e.deps.Workspace); return nil })
	group.Go(func() error {
		if e.needMemoryAudit(cause) { health, _ = e.auditMemory(gctx) }
		return nil
	})
	_ = group.Wait()

	// 2) 洞察 → 候选
	profile := e.prefs.Load()
	cands := e.generate(sig, card, health, profile, e.deps.Goal)

	// 3) 评分 + 闸门
	for _, c := range cands {
		if e.inbox.Seen(c.ID) { continue }           // 去重
		if e.gate.Suppressed(c) { continue }         // 抑制
		sc := Score(classify(c, card, profile))
		d := e.gate.Decide(sc)
		switch d.Mode {
		case ModeDrop:
			e.audit.Drop(c, d)
		case ModeAuto:
			if budgetLeft-- <= 0 { e.inbox.Queue(c); continue }
			e.executeAndVerify(ctx, c, d)             // L0 只读，结果进 res.Report
		default:
			e.inbox.Queue(c)                          // 进 Inbox/摘要，等用户
		}
	}
	e.inbox.Persist()
	e.metrics.Observe(res)
	return res, nil
}
```

```go
// 记忆自检主循环
func (e *Engine) auditMemory(ctx context.Context) (HealthReport, error) {
	entries := e.deps.Memory.ListEntries()           // 已有 API
	idx := e.scan.Symbols()

	var rep HealthReport
	groups := groupByScopeTitle(entries)
	for _, g := range groups {
		if h := detectDuplicate(g); h != nil { rep.Issues = append(rep.Issues, *h) }
		if h := detectConflict(g, e.history, e.redact); h != nil { rep.Issues = append(rep.Issues, *h) }
	}
	for _, en := range entries {
		if h := detectStale(en, idx, time.Now()); h != nil { rep.Issues = append(rep.Issues, *h) }
		if h := detectMissingFields(en); h != nil { rep.Issues = append(rep.Issues, *h) }
		if h := classifyConfidence(en); h != nil { rep.Issues = append(rep.Issues, *h) }
		if h := detectOrphanRefs(en, idx); h != nil { rep.Issues = append(rep.Issues, *h) }
	}
	// 自动应用白名单：merge / fill_keywords / mark_stale
	for _, is := range rep.Issues {
		if autoApplicable(is) && e.cfg.Memory.AutoApply {
			e.applyMemoryFix(is)                     // 写前 file_cache 备份，失败即回滚
			e.audit.RecordMemoryFix(is)
		}
	}
	e.writeReport(rep)                               // .zym/proactive/reports/
	return rep, nil
}
```

```go
// 能力创建流水线（每一步失败即止，产物留档）
func (f *CapabilityFactory) Build(ctx context.Context, s CapSpec) (BuildResult, error) {
	dir := filepath.Join(f.root, "sandbox", s.ID)
	if err := f.writeSpec(dir, s); err != nil { return BuildResult{}, err }
	files, err := f.gen(s)                           // 模板：SKILL.md / ToolFunc / MCP 配置
	if err != nil { return BuildResult{}, err }
	if err := writeSandbox(dir, files); err != nil { return BuildResult{}, err }
	if !withinWorkspace(dir) { return BuildResult{}, ErrPathEscape }

	out, err := f.runTests(ctx, dir)                 // 沙箱内：go test / test.sh，30s 超时，无网络
	if err != nil || out.Coverage < 0.70 { return BuildResult{Files: files, Test: out}, ErrGateTest }
	if hits := scanSecrets(files); len(hits) > 0 { return BuildResult{}, ErrGateSecret }
	if bad := scanDanger(files); len(bad) > 0 { return BuildResult{}, ErrGatePerm }
	return BuildResult{Files: files, Test: out, Checks: []string{"test","secret","perm","path"}}, nil
}
```

### 6.4 配置示例（`.zym/proactive/config.json`）

```json
{
  "enabled": true,
  "quietHours": "22:00-08:00",
  "dailyBudget": 3,
  "digestTimes": ["09:00", "18:00"],
  "triggers": {
    "cron": ["10 3 * * *", "10 4 * * *"],
    "onSessionEnd": true,
    "onJobFailed": true,
    "memoryDirtyEvery": 20
  },
  "memory": {
    "autoApply": true,
    "autoApplyKinds": ["merge", "fill_keywords", "mark_stale"],
    "ttlDays": { "global": 180, "project": 365, "session": 7 },
    "conflictAskThreshold": 0.15
  },
  "scoring": { "wValue": 0.30, "wUrgency": 0.20, "wRisk": 0.15, "wCost": 0.15, "wDep": 0.10, "wRev": 0.10 },
  "gate": {
    "autoMaxRisk": 0.25,
    "approveMinRisk": 0.55,
    "forbiddenRisk": 0.80,
    "maxFilesAuto": 3,
    "maxLinesAuto": 120
  },
  "executor": {
    "timeoutSec": 300,
    "allowNet": false,
    "allowExec": true,
    "allowTools": ["file_operation","batch_file_operation","run_command","read","grep","glob"],
    "verifyCmd": "go build ./cmd/deepseek/... && go vet ./cmd/deepseek/..."
  },
  "capability": {
    "enabled": true,
    "autoDraft": true,
    "autoRegister": false,
    "canaryDays": 3,
    "canaryMinCalls": 50,
    "minCoverage": 0.70
  },
  "mcp": { "healthEverySec": 300, "failThreshold": 3, "autoDisable": true }
}
```

### 6.5 分阶段实施（MVP → 迭代）

| 阶段 | 周期 | 目标 | 交付物 | 验证方式 |
|------|------|------|--------|----------|
| **P0 MVP** | 1.5~2 周 | 只读闭环：能自检记忆、能出项目卡、能列候选、能审计 | `proactive` 包（signal/scan/memory_audit/project/candidate/score/inbox/audit）+ 3 个只读工具 + UI Inbox 只读列表 + 3 条 schedule 任务 | 人工构造 30 条缺陷记忆语料，跑 1 天：报告落盘、审计完整、零写入 |
| **P1 建议与执行** | 2 周 | 加偏好画像 + 闸门配额 + L1 一键执行与自动验证/回滚 | prefs/gate/executor/verifier/rollback + 一键接受卡片 + budget/suppress | 连续 7 天真机：采纳率 ≥0.5、打扰 ≤3 条/天、失败自动回滚 100% |
| **P2 技能自动创建** | 2 周 | 缺口识别 → SKILL.md 草稿 → 校验 → staged 注册 → 灰度 | capability.go（skill 分支）+ capability_gen.go + 4 个能力工具 | 造 5 个缺口场景，产出技能可用率 ≥80%，越权 0 |
| **P3 工具自动创建** | 2~3 周 | 沙箱实现 + 单测 + 安全审查 + 注册 + canary + 监控 | tool 分支（模板生成 `ToolFunc`/插件）+ 沙箱与测试执行器 | 生成 10 个工具：首次单测通过率 ≥70%，安全审查零漏放，回滚 ≤60s |
| **P4 MCP 编排** | 2 周 | MCP 自动接入 + 健康检查 + 降级 + 卸载 | mcp 分支 + 健康探针 + 熔断 + 审计字段扩展 | 断网/杀进程故障注入：3 次失败内熔断，能力缺失可见，恢复后自动回归 |
| **P5 指标与自适应** | 持续 | 指标驱动调权重（用采纳率/拒绝率反馈 w*） | metrics.go + 周报 + 权重自适应（每次只调 ±0.05，防止振荡） | 8 周内采纳率单调不降、打扰率不升 |

---

## 7. 验收标准

### 7.1 记忆自检有效性

| 指标 | 定义 | 目标 |
|------|------|------|
| 检出召回率 | 已知缺陷被检出比例（30 条基准语料：冲突 5/过期 5/重复 5/缺字段 5/低置信 5/正常 5） | ≥ 0.80 |
| 检出准确率 | 报出的问题中真问题比例 | ≥ 0.85 |
| 误伤率 | 正常条目被自动改写/删除的比例 | **= 0** |
| 自动修复正确率 | merge/fill/mark_stale 自动应用后被回滚的比例 | ≤ 5% |
| 记忆健康度 | `1 − (重复率+过期率+低置信率)/3`，周环比 | 4 周内 +0.10 |

验证方式：`go test ./cmd/deepseek/proactive -run TestMemoryAuditGolden`（golden 语料 + 快照断言）+ 每周一次真机报告人工抽检 10 条。

### 7.2 主动任务发现"不跑偏、不打扰"

| 指标 | 定义 | 目标 |
|------|------|------|
| 提案采纳率 | 接受数 / 推送数 | ≥ 0.50（P1 起），≥0.60（P5） |
| 打扰率 | 每日推送条数（用户主动请求外） | ≤ `dailyBudget`，默认 3；7 天滚动均值 ≤2 |
| 静默守约率 | 静默时段内推送数 | = 0 |
| 重复建议率 | 7 天内同 ID 二次出现比例 | ≤ 5% |
| 跑偏率 | 提案目标路径不在 `pref.focus_dirs` 且未被接受的比例 | ≤ 10% |
| 拒绝学习率 | 用户拒绝后 90 天内同类再提的比例 | ≤ 10% |
| 证据完备率 | 提案中带可点击证据（文件:行/审计 id）的比例 | = 100% |
| 陈旧率 | 72h 后仍停留在 queued 的比例 | ≤ 5%（超出即过期） |

### 7.3 工具/技能/MCP 创建的安全与可回滚

| 指标 | 目标 |
|------|------|
| 生成物首次通过单测率 | ≥ 70%（P3） |
| 安全审查高危漏放 | **= 0**（密钥/路径逃逸/`go get`/网络通配 任一漏放即为失败） |
| 注册前 staged 比例 | = 100%（无 staged 不得 enabled） |
| 灰度劣化自动退回时延 | ≤ 1 次评估周期（≤24h），且 ≤3 次失败调用内熔断 |
| 回滚成功率 | 100%（R1 ≤5s / R2 ≤30s / R3 ≤60s） |
| 审计完整率 | 执行类事件审计字段齐全比例 = 100% |
| 越权拦截率 | 注入"尝试写工作区外/装依赖"的测试用例，拦截率 = 100% |

### 7.4 全局红线（任一不达标即回退阶段）

1. 出现一次"未审批的高危写入"→ 立即关闭自动执行（`gate.autoMaxRisk=0`）。
2. 出现一次密钥落盘/日志泄露 → 停用能力创建分支，全量重扫。
3. 连续 5 天采纳率 <0.2 → 降频到 1 条/天并复盘候选来源。

---

## 8. 假设、风险与边界

### 8.1 假设

- Agent 运行在用户本机，工作区可读写，`git` 与 `go` 工具链可用（MVP 验证命令依赖它们；缺失时自动跳过相关检查）。
- 用户接受"Agent 在后台定期自我检查"，但默认只以简报形式呈现，不弹窗。
- 记忆以 Markdown 条目（`MemoryStore`）为唯一事实源；项目卡是派生视图，可随时重算，不写回记忆正文。
- 单机单用户，不做多租户；IM 推送（`bot`）可选。

### 8.2 风险与缓解

| 风险 | 影响 | 缓解 |
|------|------|------|
| 自激循环（提案→执行→新提案） | 烧 token、打扰 | `ID` 去重 + 每轮候选上限 20 + 每日预算 + `Scope` 锁 + 执行后 30 分钟内不再对同 Scope 提案 |
| 规则误判（如把"取值不同"判为冲突） | 误改记忆 | 自动动作仅三类且都有备份；其余走确认；误判可回滚 |
| 提案噪声大（TODO 类刷屏） | 用户失去信任 | TODO 类默认只在摘要里聚合，不单条推送；`Value` 上限 0.6 |
| 项目卡过大塞满上下文 | 上下文成本 | 卡片刻意做"摘要视图"（≤400 字），细节按需检索 |
| 子 Agent 越权/跑飞 | 破坏工作区 | 白名单工具 + `PermAsk` + `MaxFiles/MaxLines` + 超时 + 自动回滚 |
| 生成工具带后门/危险默认 | 安全事件 | 10 项安全审查 + staged + canary + 审计 + 一键卸载 |
| 依赖外部 LLM 质量 | 提案质量波动 | 规则层决定"是否提案/是否可执行"，LLM 只写"描述与补丁草案" |
| 用户反感"AI 自作主张" | 卸载能力 | 一句话关闭：`enabled=false`（保留审计）；偏好里可永久 `suppress` 某类 |

### 8.3 边界（明确不做）

- 不做"无人值守的大规模重构"：单次执行 ≤3 文件 / ≤120 行；超限必须人工拆解。
- 不做跨项目/跨机器的主动干预（只作用当前工作区）。
- 不自动发布/部署/推送远程分支（L3 永久禁止自动）。
- 不为"不确定价值"的猜测生成提案：无证据指针的候选直接丢弃。
- 不修改 Agent 自身的权限配置与安全阈值（阈值只由用户改），防止自我提权。

---

## 附：落地第一步清单（可直接开工）

1. `git checkout -b feat/proactive`。
2. 建 `cmd/deepseek/proactive/{config.go,engine.go,audit.go,inbox.go}` 骨架，`Engine.Tick` 先只跑 `collectSignals` + 写审计（可观测性优先）。
3. 接 `memory_audit.go`（六类检查 + golden 语料测试）——这是 ROI 最高、风险最低的一块。
4. 接 `project.go` 项目卡（注意把 `cmd/smokewebview` 编译失败写成基线）。
5. `agent/proactive_tools.go` 暴露 `proactive_status` / `proactive_cycle` 两个工具，先在会话里手动触发跑通。
6. 再上 `score.go` + `gate.go` + Inbox UI（P1）。

---

## 附二：实施状态（P0 已完成）

已落地代码（`cmd/deepseek/proactive/`，`cmd/deepseek/` 在本仓被 .gitignore 忽略）：

| 文件 | 内容 |
|------|------|
| `config.go` | Config / 默认值 / 读写 `.zym/proactive/config.json`（保守默认：不自动改记忆） |
| `audit.go` | AuditEvent 必填校验、按月 JSONL 追加、`Redact` 脱敏、ID 生成 |
| `inbox.go` | 提案状态机、去重、拒绝抑制（单条 14d / 同类 2 次 90d）、配额、静默时段 |
| `memory_audit.go` | 六类缺陷判定（重复/冲突/过期/缺字段/低置信/孤儿引用）+ 健康度 + Markdown 报告 |
| `score.go` | 六维评分、风险闸门（L0/L1/L2/L3 映射）、指纹去重 ID |
| `engine.go` | Tick 闭环（自检→提案→收件箱→报告→审计）、60s 去抖、Accept/Reject |
| `memory_audit_test.go` / `gate_inbox_test.go` | golden 语料（含误伤断言）、闸门表驱动、收件箱/审计/引擎端到端 |

接线：

- `agent/proactive_bridge.go`：`MemoryStore → proactive.MemoryReader` 适配 + 4 个工具（`proactive_status` / `proactive_cycle` / `proactive_inbox` / `proactive_decide`），引擎缺失时优雅降级。
- `agent/tool.go`：`Env.Proactive *proactive.Engine`；`agent/loop.go`：`BuildRegistry` 注册工具组。
- `pages/proactive_wiring.go` + `pages/state.go`：按工作区缓存引擎并注入 Env，简报用 Toast 提示。

验证：`go build ./cmd/deepseek/...`、`go vet`、`gofmt` 全通过；`go test ./cmd/deepseek/proactive/`（含 gate/inbox/audit/engine）与 `go test ./cmd/deepseek/agent/ -run TestProactive` 全绿。

一处实现修正（与上文 3.2/2.2 表述的差异）：非同名记忆判"冲突"时额外要求正文相似度 ≥ 0.20——只共享标签（如都带 `约定/已验证`）而正文毫无交集的条目属于不同主题，否则误报率过高。

已知既有失败（与本次改动无关）：`agent.TestParseSSEServerToolFlow`（SSE server_tool_event 分步合并），属 SSE 解析既有问题。

## 附三：实施状态（P1 受限执行与回滚已完成）

新增文件：

| 文件 | 内容 |
|------|------|
| `proactive/rollback.go` | `FileSnapshotter`：执行前把影响范围内的现存文件复制到 `.zym/proactive/backups/<runID>/`，登记执行前不存在的路径；`Diff` 以快照为基准算出改动文件与行数；`Restore` 还原内容 + 删除执行期新建文件 + 拒绝 `..` 越界 |
| `proactive/executor.go` | `Runner/Verifier/Snapshotter` 接口 + `Execute`：先备份 → 受限执行 → 影响面 diff 校验（MaxFiles/MaxLines）→ 验证命令 → 任一失败立即回滚 → 审计 |
| `proactive/executor_test.go` | 成功 / 验证失败回滚 / 越界回滚（含执行方谎报影响面）/ 缺少影响范围与验证命令拒绝 / 引擎级 accept 与 confirm |
| `agent/proactive_exec.go` | `RestrictRegistry`（白名单工具集）、`DenyApprover`（主动执行不得自行提权）、`CommandVerifier`（shell 验证命令）、`ProactiveRunner`（白名单 Loop，禁孙 Agent、禁递归触发） |

语义变化：

- `Engine.Accept(ctx, id)` 真正执行（低风险），`Engine.Confirm(ctx, id)` 用于高风险（`ModeApprove`）二次确认；返回 `RunRecord`。
- 没有执行后端 / 记忆类提案仍走「仅记录接受」，不会误触发执行。
- 状态机新增 `running` / `rolled_back`：验证失败或越界 → 自动回滚 → `rolled_back`。
- 影响面以快照 diff 为准，执行方自述的 `Files/Lines` 不参与上限判定。
- `proactive_decide` 工具新增 `confirm` 动作。

配置：`executor.verifyCmd`（缺省为空时按「验证失败」处理并回滚，必须显式配置验证命令才会真正落地改动）、`executor.timeoutSec`、`executor.allowTools`、`gate.maxFilesAuto` / `gate.maxLinesAuto`。

验证：`go build ./cmd/deepseek/...`、`go vet`（proactive/agent/pages）、`gofmt` 全通过；`go test ./cmd/deepseek/proactive/` 与 `go test ./cmd/deepseek/agent -run TestProactive` 全绿。

### P1 已落地：受限执行 + 自动回滚

| 文件 | 内容 |
|------|------|
| `proactive/rollback.go` | `FileSnapshotter`：执行前按影响范围备份到 `.zym/proactive/backups/<runID>/`（含 manifest 记录"原本不存在"的路径 + scope），`Restore` 还原内容、删除执行期新建文件，`Diff` 以快照为基准算出真实影响面 |
| `proactive/executor.go` | 受限执行器：备份 → 限时执行 → `Diff` 测真实影响面 → 上限校验（文件数/行数）→ 验证命令 → 任一不达标立即回滚；每次执行写审计（`file_edit` + 回滚点 + 验证输出） |
| `agent/proactive_exec.go` | `RestrictRegistry`（白名单即全部工具）、`DenyApprover`（后台一律拒绝提权）、`CommandVerifier`、`ProactiveRunner`（独立 Loop：不派遣孙 Agent、不递归触发主动能力） |
| `pages/proactive_wiring.go` | 组装执行后端：白名单工具集来自配置、验证命令取自配置或按工程类型推断（go.mod → `go build ./...`） |

安全性质（均有单测）：未声明影响范围拒绝执行；超过文件/行数上限回滚；缺验证命令按失败回滚；验证失败回滚且文件字节还原；执行方谎报影响面以快照 diff 为准；高风险提案必须 `Confirm` 二次确认。

`proactive_decide` 的 action 因此扩展为 `accept` / `confirm` / `reject`。

### P1 补充：收件箱 UI 与定时自检

| 文件 | 内容 |
|------|------|
| `pages/proactive_panel.go` | 收件箱弹窗：标题栏（启用状态 / 待处理数 / 可执行性 + 待处理·全部筛选）、提案卡片（标题 / 理由 / 证据 / 分值 / 范围 + 执行·确认执行·拒绝）、底部（立即自检 / 刷新 / 上次执行结果 / 错误）。执行走后台 goroutine，完成后 `app.Post` 回 UI 线程刷新 |
| `pages/sidebar_more.go` | 「更多」菜单新增「主动建议」入口 |
| `pages/shell.go` / `pages/state.go` | 浮层注册到根节点、ESC 关闭接入既有优先级链 |
| `pages/proactive_wiring.go` | 取引擎时若已启用则启动自检循环；节奏为「开启后 10 秒首次，之后每 30 分钟」，关闭时通过 cancel 整体退出；简报经 `app.Post` 回到 UI 线程再 Toast |

UX 约束：只弹一次轻提示、不抢焦点、不自动切路由；执行按钮文案区分风险等级（高风险显示「执行（需确认）」）。

**主动模式开关**：`pages/proactive_panel.go` 提供 `proactiveModeChip()`（标题栏「主动」胶囊，绿点=已开启，点击即切换）与面板内的 `主动模式：开/关` 按钮，二者都调用 `Engine.SetEnabled` —— 立即写 `.zym/proactive/config.json`，开启时顺手跑一次自检并 Toast 提示；落盘失败会回滚内存状态，避免「界面开着、重启没了」。

**自检 → 新会话 → 模型主动干活**（`pages/proactive_session.go`）：

1. 触发：主动模式开启后 10 秒首次，之后每 30 分钟一次（`startProactiveLoop` / `proactiveDelay`）；面板「立即自检」同样触发。
2. 条件：每轮都开一个会话——有建议就带着建议去做，无新建议时提示词改为「轻量只读巡查 + 两行报平安」，不硬找活干；同时只允许一轮（`proactiveBusy`）。
3. 步骤：UI 线程建会话「主动自检 MM-DD HH:MM」→ 写入自检报告（统计 + 待处理建议 + 行动指令）作为用户消息 + 助手占位 → 后台跑一轮 `agent.Loop`（模型 + 工具，`runBlocking` 走统一并发额度）→ UI 线程收尾。
4. 约束：不切换用户当前会话、不抢焦点；权限沿用当前 Perm 设置，但审批钩子只放行 `Risk == ""` 的低风险操作，带风险提示的一律拒绝；同一时刻只有一轮。

