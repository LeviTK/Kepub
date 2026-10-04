# M2-B：metadata.set 与 workspace plan/apply/diff 库

日期：2026-10-04。共同基线：[77821e6](https://github.com/LeviTK/Kepub/commit/77821e6b163f45f81134394873d35f3ede8774ad)。本阶段是独立库交付，不是公共 CLI 或审核/导出闭环。

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

限制：flock/mutex 是协作单写者，不是同用户恶意进程沙箱；计划记录不是对整个可写工作区的密码学认证。外部写者必须停止。未证明断电、ENOSPC、任意阶段磁盘损坏恢复；不可恢复/不完整记录拒绝打开而非猜测成功。macOS 仅交叉编译，没有 Mac 实机测试。没有运行 EPUBCheck，不可宣称 conformance passed、已接受或可正式导出。
