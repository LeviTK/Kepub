# M2-B：metadata.set、workspace 库与编辑审核导出闭环

第一批日期：2026-10-04。共同基线：[77821e6](https://github.com/LeviTK/Kepub/commit/77821e6b163f45f81134394873d35f3ede8774ad)。以下保留第一批独立库交付的范围与证据；第二批公共 CLI / 审核导出闭环见本文末节。

## 范围与接口

仅修改 `internal/metadata`、`internal/workspace` 与本文。保留 M2-A 的 Create/Open/Close、State、NewCandidate/Candidate、Checkpoint/Checkpoints/Restore、HashTree API 和 state v1。没有新增 accepted 指针、accept/export、rename、校验布尔绕过或 capabilities 声明；没有改动 archive/publication/references/testfixture/app/cmd、锁文件、setup 或主契约。

导出接口：

```go
// metadata
type Set struct {
    Namespace, LocalName string
    ID                   string // 可选；省略时也必须唯一匹配
    ExpectedOldValue      string
    NewValue              string
}
func Apply(input []byte, request Set) ([]byte, bool, error)

// workspace：均使用既有 handle 的 mutex 与单写者 flock
func (*Workspace) ID() (string, error)
func (*Workspace) Plan(requestJSON []byte) (Plan, error)
func (*Workspace) Apply(planJSON []byte) (Execution, error)
func (*Workspace) Diff() (Diff, error)
func (*Workspace) Execution() (Execution, error)
```

还导出 Operation、Request、Plan、Change、Diff、Execution 和 ErrStalePlan、ErrCandidateConflict。已有 ErrBusy/ErrClosed/ErrRecovery/ErrReadOnly 继续适用；JSON/schema/支持子集拒绝及 I/O 错误通过返回 error 表达，本阶段没有公共 CLI 错误码映射。

调用方必须按显式目录 Create/Open，不能从计划的 workspaceId 查找“最新工作区”或自动发现任务。典型调用为 `w.Plan(requestBytes)` → `json.Marshal(plan)` → `w.Apply(planBytes)`；也可读取返回 planId 对应的 `plans/<planId>.json`。Apply 必须核对本工作区实际持久计划，不接受未登记的任意自造计划。

请求示例：

```json
{
  "schemaVersion": 1,
  "operations": [{
    "operationId": "metadata.set",
    "operationVersion": 1,
    "params": {
      "namespace": "http://purl.org/dc/elements/1.1/",
      "localName": "title",
      "id": "t",
      "expectedOldValue": "Old & Title",
      "newValue": "New < Title"
    }
  }]
}
```

恰好一个操作；除可选 id 外字段必须存在，不用缺省旧值代替显式空字符串。拒绝未知/重复 JSON 字段、null 代替值、尾随文档、错误版本、多操作、未知操作。显式空 id 没有选择意义，须省略；JSON 上限 32 MiB、嵌套上限 128，拒绝非法 UTF-8。摘要以类型化 Go JSON 为规范形式：固定 struct 字段顺序、无空白、标准 Go 字符转义、可选 id 省略，然后 SHA-256；输入键顺序/空白不影响操作摘要。不是 RFC 8785/JCS 实现。

## 支持子集与保真

- EPUB2/3 OPF，已有且唯一选中的直接 metadata 子元素 dc:title/dc:creator。namespace + localName + 可选明确 id 选择；expectedOldValue 是解码后的完整文本，不能借旧值从多个同名字段中挑一个。全 OPF 重复非空 id 拒绝，以免产生关联歧义。
- 目标仅简单文本；子元素、目标内注释、CDATA、PI、自闭合目标拒绝，包括 no-op。显式 `<dc:title></dc:title>` 的空文本可修改。identifier/language 不支持。
- 使用 XML token 的准确原始字节偏移，只替换选中开始/结束标签之间的文本。只 escape 新值，并完整重解析结果、核对解码后新值；不重序列化 OPF，不替换同文注释/另一个字段，不加修改时间。
- 目标之外的属性、命名空间、refinement、BOM、CRLF、注释、顺序及其他资源字节保持原样。no-op 返回 changed:false，且不写候选 OPF。
- 完整验证 OPF 而不是找到目标后提前返回：UTF-8/内建及数字实体；拒绝 DTD/directive、自定义实体、xml:base、坏 XML、重复属性、未声明/非法命名空间与不匹配 QName。继承命名空间使用链式词法作用域，避免深度导致重复复制全部 namespace map。
- 与既有解析策略使用相同 8 MiB XML、128 层、200,000 tokens、累计文本/位置 32 MiB 上限；额外拒绝错位 XML 声明及非法 namespace 用法。这里是有界安全解析与局部编辑，不是 EPUB 合规验证，也不是全面 XML 标准认证。

## 身份、计划、执行与恢复

`identity.json` 是 version 1 的随机 16-byte lowercase hex workspaceId。新 Create 写入；旧 M2-A 工作区 Open 不静默迁移，第一次显式 ID/Plan 调用补建。state v1 保持原格式。一旦已有计划或身份被当前 handle 读取，身份缺失不能再生成。计划同时绑定实际 ID 与绝对路径；工作区搬迁后旧计划 stale。

Plan 只读 immutable initial publication，保存 rootfile、baseRevision、精确 Tree SHA-256、操作版本/参数摘要、固定策略摘要及预期写集。只写身份/计划报告，不建候选、不改出版文件。此阶段仍只支持 initial 基线；没有伪造多 accepted revision 生命周期。

Apply 严格解码计划，核对已登记报告和身份/目录，再验证 original/initial 完整哈希、从实际 OPF 重算操作和写集。不能用 applicable/writeSet 掩盖修改；已有候选或重跑同一计划均冲突。

候选发布时同时保存 edit-intent，之后建独立 checkpoint、持久 edit-start，才进行局部写入。OPF 新字节先写独立 staging 文件，再替换候选内单个文件。扫描整个实际候选，核对所有增删改/类型变化及精确预期 OPF 内容，而不是只检查声明中的文件。越界写、错误内容或阶段失败恢复 checkpoint；保存失败记录，结果报告本身发布失败也回滚。结果 JSON 完整写好后才发布。异常碰撞文件不擅自删除。

Open/Execution 核对 task、intent、已登记 plan、checkpoint、执行状态与实际树。只有 intent 或 start、尚未有 result 的中断任务恢复为 failed，不自动重跑、不继承成功。不完整/篡改记录明确拒绝。成功 review 记录须匹配重新观测的 diff、预期写集和重算的新字节；即使伪造与实际树相符的报告，也不能越过原计划限制。候选完成后再被外部修改会使 review 记录 stale；Diff 仍在当前持锁 handle 上提供实际文件变化，不能把旧 review 状态视作接受授权。

成功只返回 `status:"review_required"`、`reviewRequired:true`、`conformance:"not_run"`。No-op 同样需要审核，Diff.changed 为 false。任务固定为唯一 active 候选；没有外部 writer 管理/停止证明、正式检查、accept 或 export。后续父线程需在已集成校验 API 上实现这些门槛，不能传 passed:true 替代验证。

Diff 从两树实际路径/类型/大小/SHA 比较得到 added/deleted/modified。目录和非 manifest 文件也纳入；同路径文件→目录记 modified，改名表现为删+增，不臆断语义 rename。Tree 算法继续使用 M2-A 的按精确路径排序、JSON Entry 数组及 `kepub-tree-v1\n` 前缀 SHA-256，不使用 mtime。

## 验证

Linux orb，Go 1.27.1；测试生成小型 EPUB，不使用用户书籍或外部模型。执行：

```sh
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
GOOS=darwin GOARCH=arm64 go build ./...
go test ./internal/metadata -run '^$' -fuzz '^FuzzSimpleTextPreservation$' -fuzztime=10s -parallel=2
git diff --check
```

结果：全仓单测与 race 均通过；vet 与 Darwin arm64 build 退出 0，git diff --check 无输出。另对 workspace、metadata 两包执行 Darwin arm64 `go test -c`，仅编译测试二进制、不运行。最终 metadata 保真 fuzz 为 PASS，98,468 次执行（本 orb 的缓存语料参与 seed 阶段）；此前一次独立运行为 PASS、128,853 次执行。根 `go test ./...` 不运行独立 `experiments/amp-sdk` 模块，本线没有改动或重新认证该实验。

单测覆盖：

- 精确 BOM/CRLF/namespace/属性/refinement/注释 byte preservation，同文多字段由 id 精确选择，旧值错/重复 id/同名字段歧义拒绝；no-op 原字节、creator、内建/数字实体及特殊字符、显式空文本。
- 注释/CDATA/混合子元素/PI/自闭合目标、目标后坏 XML、DTD/实体、UTF-8/编码错误、xml:base、重复/别名属性、命名空间继承和非法/错位声明；XML 8 MiB 边界及超限、depth/tokens/text-index 超限；no-op 同样不能绕过解析/子集限制。
- Plan 不改 publication；schema/版本/数量/字段拒绝、过期基线、计划身份/路径/摘要/操作/写集篡改；篡改已登记写集仍要重算。
- 候选与 initial/checkpoint/original 独立；正常写入、真实越界新增/删除/类型变化、同写集错误 OPF 内容、受控 I/O 失败与结果报告碰撞均触发回滚/拒绝。
- intent、checkpoint 后和写入后中断，再次 Open 为 failed；正常与失败结果再次打开保留状态；重复 apply 拒绝、两个并发 apply 仅一个成功；复用 M2-A 跨进程 flock/进程退出恢复测试。
- 真实 Diff 的新增/删除/类型变化/同大小不同 SHA；伪造自洽 diff（新增文件或同 OPF 路径不同内容）不能得到可审核状态；旧身份迁移、持久身份丢失、目录搬迁。

保真 fuzz 的受支持输入约束避开大部分随机解析拒绝，以独立转义 oracle 构造 old/new 文本，并检查 changed 语义与完整字节结果。通过普通单测中的种子，并单独运行有界 fuzz。

第一批限制：flock/mutex 是协作单写者，不是同用户恶意进程沙箱；计划记录不是对整个可写工作区的密码学认证。外部写者必须停止。未证明断电、ENOSPC、任意阶段磁盘损坏恢复；不可恢复/不完整记录拒绝打开而非猜测成功。macOS 仅交叉编译，没有 Mac 实机测试。第一批没有运行 EPUBCheck，不宣称 conformance passed、已接受或可正式导出。

## 第二批：可执行编辑→审核接受→accepted-only 导出

日期：2026-10-05，Medium，亲自实现，无子线程。本批从父提供的精确本地基线 `94854e87470508fbcf200c306510415dbfd87e33` 开始：已含第一批 M2-B、Q1 校验/pack 和最终取消/进程组修复、Q3 环境准备。输入 bundle SHA-256 为 `a15ebeba2c8bca8d896c89cc17ed1dd1bc6c6c85838d697c88d13a97d9d9b7be`，验证 prerequisite 后快进并删除传输文件。父另行负责 README/DEVELOPMENT_PLAN，本批不修改 archive/validation/setup/锁文件，不推送。

### 导出 API 与命令

第一批及 M2-A API 保留。新增：

```go
// workspace：生命周期均保持 mutex + 跨进程 owner lock
func (*Workspace) TaskID() (string, error)
func (*Workspace) TaskDiff(taskID string) (Review, error)
func (*Workspace) Accept(ctx context.Context, taskID string, opts validation.Options) (Decision, error)
func (*Workspace) Reject(taskID string) (Decision, error)
func (*Workspace) AcceptedSnapshot() (*archive.Archive, archive.Tree, Revision, error)
func (*Workspace) OutputPath(output string) (string, error)
func (*Workspace) WritePlanReport(plan Plan, output string) error
func ReadEditFile(file string) ([]byte, error)

// app：CLI/其他入口共用应用用例，不要求模型或 GUI
func OpenWorkspace(book, dir, rootfile string) (WorkspaceResult, error)
func PlanWorkspace(dir, operationsFile, output string) (workspace.Plan, error)
func ApplyWorkspace(dir, planFile string) (workspace.Execution, error)
func WorkspaceTask(ctx context.Context, dir, taskID, action string, opts validation.Options) (any, error)
func ExportWorkspace(ctx context.Context, dir, output string, opts validation.Options) (WorkspaceExportResult, error)
```

`Review` 包含实际完整 `Diff`、baseRevision、taskId、matchesExecution，以及所选字段的 oldValue / plannedValue / 实际 newValue。复杂、歧义或无法解析的实际字段返回 newValue:null 和 unavailable，不把计划新值冒充实测。`Execution` 增加 taskId；新增 ErrTaskConflict / ErrCandidateDrift。CLI 映射 WORKSPACE_BUSY、TASK_CONFLICT、INPUT_DRIFT、RECOVERY_REQUIRED 到退出4，参数/非法输出到2，依赖/检查超时沿用 Q1 退出5，普通 I/O 到6，取消到130；stdout 始终单 envelope，嵌套命令 command 使用顶层 workspace/task。

```sh
kepub workspace open BOOK.epub --output WORKSPACE_DIR --json
kepub plan --workspace WORKSPACE_DIR --operations operations.json --output PLAN.json --json
kepub apply --workspace WORKSPACE_DIR --plan PLAN.json --json
kepub task diff TASK_ID --workspace WORKSPACE_DIR --json
kepub task accept TASK_ID --workspace WORKSPACE_DIR --json
kepub workspace export WORKSPACE_DIR --output OUT.epub --json
# 或显式拒绝：保留审计，之后生成新计划再编辑
kepub task reject TASK_ID --workspace WORKSPACE_DIR --json
```

workspace open 仅新建，已有目录通过其他命令 Open；无隐式注册表、发现或 latest。TASK_ID 必须取 apply 返回的持久 ID，不能猜测 active/latest。Plan/export 输出均要求在整个 workspace 根之外、父目录已存在且所有祖先为真实目录、输出不存在，包括保护树内不存在的新文件。符号链接别名不能绕过该边界。operations/plan 输入拒绝符号链接、硬链接和特殊文件；复用既有 O_NONBLOCK/O_NOFOLLOW + 前后 inode 核对，FIFO 无 writer 也不会卡在打开阶段。

capabilities 仅开放已实现的 metadata.set（上述单字段子集）、workspace.open/export、plan/apply、task.diff/accept/reject；resource.rename、workspace.list/registry、Amp task.run、preview 和 doctor 保持 planned。不改变 Q1 PackSnapshot 签名或成功发布后的取消语义。

### 持久状态和提交/恢复语义

- state v1 和 original/initial 永不推进或标记为已验证。`accepted.json` 独立绑定持久 workspaceId 与当前 revision；不存在时仅允许未推进的 legacy initial。随机 task record v2 保留唯一 active 目录。旧 task v1 用显式 legacy ID active 读取/diff/reject，接受须重新生成现代计划；旧 M2-B initial-only policy 可以读，现代计划策略改为 accepted-baseline v2，操作仍是 metadata.set v1。
- Plan/apply/checkpoint/restore/diff 从当前 accepted revision 工作。每次接受建立独立 revision/pub 和精确 manifest，包含父 revision、task、rootfile、执行摘要与真实 validation；全部父链、来源计划/执行/决策/检查点和实际树在重开时验证。候选、checkpoint、initial、已接受 revision 都是独立内容副本。No-op 不写 OPF；显式接受仍建立审计 revision，Tree SHA 与字节不变。
- `plans/<id>.used.json` 将计划消费绑定任务；拒绝或接受后旧计划不能再次当新任务运行，基线推进即使内容 hash 相同也使旧 generation 计划 stale。Apply 仍只到 review_required / conformance:not_run，不借用 accept 后的检查结果改写执行历史。
- Accept 检查独立冻结树，直接调用实际 validation.Validate；没有 passed:true、可替换 checker 回调或 draft acceptance。archive、parse.structure、固定 EPUBCheck 5.3.0 必须完整 passed，InputTree/Archive/Config/check evidence 精确绑定；error/fatal 或缺检查阻断。Q1 的 CSS partial coverage/诊断保留，但它不是 conformance 必需检查，也不开放 rename。
- 检查尝试单独保留 checks_passed/checks_failed，检查通过不等于已接受。冻结复制、检查、重扫与 revision/日志准备完成后，紧邻持久 settlement journal 发布前最后核对 ctx；这是取消可撤回的最后边界。Journal 发布提交不可撤回意图；随后独立 revision 发布、accepted 指针原子替换安装可见基线、decision 写入、task 归档、journal 清理。发布意图后的重开永远 roll-forward，不重跑检查；冻结 bytes/来源/报告不一致则拒绝恢复。中断失败返回 accept_pending 并要求重开；意图发布前取消只留下真实检查尝试，不推进指针。
- Reject 使用同一小型结算 journal 保留 task/检查点/检查尝试与 decision，不删原书、revision 或审计。没有执行的旧 M2-A 手工候选可安全拒绝，不冒充检查通过。已结算 task ID 不可再接受/拒绝，新的 edit 使用新任务。
- 完成候选被普通外部写者修改后，Open 允许安全文件树重开 diff/reject，Execution/Accept 返回 ErrCandidateDrift，matchesExecution:false。路径逃逸或记录损坏仍拒绝，而不是为便利跳过来源验证。
- Export 只从 AcceptedSnapshot（初始为未验证 initial）传入 app.PackSnapshot(ctx,a,approved,output,options)。未接受 active 候选从不进入产物；正式导出独立检查最终 ZIP 并按 Q1 no-replace 边界发布。显式 --draft 保留未正式验证标记，缺依赖绝不自动降级。导入状态不因 export 前已有候选报告而冒称 pass。

### 验证环境、用例与证据

本 orb 执行仓库 `.agents/setup`，安装 Java `17.0.20.1+1-1~deb12u1`，校验 EPUBCheck 5.3.0 release/main JAR/完整 JAR 集；正式测试用全新干净 login shell 获取 profile 中工具路径，而非继承旧 shell 或因缺变量跳过：

```sh
env -i HOME="$HOME" USER=user PATH=/usr/bin:/bin /bin/bash -lc 'go test ./... -count=1'
env -i HOME="$HOME" USER=user PATH=/usr/bin:/bin /bin/bash -lc 'go test -race ./... -count=1'
env -i HOME="$HOME" USER=user PATH=/usr/bin:/bin /bin/bash -lc 'go vet ./...'
GOOS=darwin GOARCH=arm64 go build ./...
# go test -c 对所有根模块包交叉编译测试，只编译、不运行 Mac 二进制
git diff --check
```

新增区分性测试：

- 独立生成合规 EPUB2/3，真实连续 title/creator edit/accept，重开后基线推进；accepted 基线上的越界写失败回滚和拒绝后再次编辑；旧 plan/task 拒绝；no-op 字节和树不变。
- 编译真正 CLI 二进制并逐命令执行 EPUB2/3 闭环，每步重新 Open；候选 diff 有实际 old/new 文本与完整资源变化；接受前正式 export 精确输出 initial，而不是候选；两次接受后正式 export 检查最终 ZIP，并逐资源比较原字节，OPF 只存在独立推导出的两个局部替换。
- 参数/未实现边界、未知 passed 标志、输出已存在不覆盖，保护树内此前不存在的新文件不能成为 Plan 或 export 输出，符号链接别名拒绝；无 writer 的真实 FIFO operations/plan 各在3秒有界进程内非零退出、单 envelope、无候选。
- 缺检查器、真实非法 EPUB、draft acceptance、取消均不推进 accepted；持久检查尝试是真实成功/失败，不改变 apply 的 not_run。最终取消检查观察已准备的 settlement 文件，证明晚于所有昂贵扫描；取消后可重开重试该显式任务。
- 候选实际 tamper 加/删/改仍可重开 review/reject，接受拒绝继承状态；accepted bytes 和归档执行来源 tamper 使重开失败。
- 接受 journal 的无意图预备、意图已发布、revision 已发布、指针已安装、decision 已写、task 已归档、journal 已清理边界逐一重开恢复；被冻结内容篡改不能推进指针。保留已有锁/跨进程/中断 apply/restore/byte-preservation 测试。

最后两处增量为 initial snapshot manifest slice 隔离，以及接受提交后晚到取消不能重写成功的 CLI 回归测试。早先 PID39300 的整仓检查完成时仍有这些增量，不把该次检查笼统算为最终版本。PID49951 启动前已完成增量、gofmt 与重读；此后 Go 源码保持不变，仅补充本文的事实证据。该进程退出0，最终输出 `FINAL TREE VERIFIED`：

| 最终稳定源码检查 | 结果 / 决定性输出 |
|---|---|
| `go test ./... -count=1`，干净 login / 真实 EPUBCheck | 全部通过；cmd 111.242s、workspace 130.693s、validation 180.656s |
| `go test -race ./... -count=1`，相同环境 | 全部通过；cmd 104.887s、workspace 135.465s、validation 172.716s |
| `go vet ./...` | 退出0，无诊断 |
| `GOOS=darwin GOARCH=arm64 go build ./...` | 退出0 |
| `GOOS=darwin GOARCH=arm64 go test -c -o "$cross/" ./...` | 所有根模块测试包编译；`file` 显示9个 Mach-O 64-bit arm64 executable；不运行 Mac 二进制，临时目录清理 |
| `git diff --check` | 无输出，退出0 |

交付提交/bundle 见本线程最终报告。限制仍是协作单写者、非同 UID 沙箱；不宣称断电/任意磁盘损坏恢复。Mac 仅 Darwin arm64 交叉编译，无实机运行；不执行真实 Amp、用户书籍、GUI、rename、Calibre 或独立 amp-sdk 模块认证。
