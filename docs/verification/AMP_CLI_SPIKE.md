# P3 / M0-A：Go 直接管理 Amp CLI

验证日期：2026-10-05（Asia/Shanghai；orb 为 2026-10-04 UTC）。

**结论：Linux 离线协议和受管进程组实验通过；不是生产 Agent，也不是已完成真实 Amp 或 Mac 联调。** 原型只在 `experiments/amp-cli`；`cmd/kepub`、capabilities、工作区、根模块及 setup 均未修改。原型自身只用 Go 标准库，核心仍无模型依赖。

## 1. 基线与证据范围

- 从父线程提供的 Git bundle 导入 M1-A / 开发方案 v0.4 基线，分支 `experiment/amp-cli-m0a`；输入为 `docs/DEVELOPMENT_PLAN.md` §8.1.1 和 §11.1。续接绑定及 terminal data 使用父线程后续统一的实验 v1 补充契约。
- 实测：Go 1.27.1、Linux amd64、真实 OS pipe/process/process-group/signal、自编译 fake CLI、一次性 `t.TempDir()` 候选。
- 本机仅查询了 `amp --version`、`amp --help`、`amp threads continue --help`。观测版本 `0.0.1791148688-gddc1cf`，发布于 2026-10-04T21:18:08Z。这只是观测版本，不是已认证或运行时强制锁定的生产版本。
- 没有运行真实 Amp execute，没有调用模型、上传用户书籍、访问真实线程、启用全权限或执行 SDK。文档中的 session/thread ID 与结果全部是明确的 fixture。
- 已交叉编译 `darwin/arm64` Mach-O；**未执行 Mac 二进制**，不据此证明 macOS 清理、签名、公证或干净机器打包通过。

协议依据：[CLI execute mode](https://ampcode.com/docs/cli/execute-mode)、[CLI Streaming JSON](https://ampcode.com/docs/cli/streaming-json)、[SDK TypeScript 消息定义](https://ampcode.com/docs/sdk/typescript) 和上述 CLI help。SDK 公共对象不直接等于 stdin wire；本次复核按下文 §2.1 修正 request ID 字段。没有引入 SDK 依赖。

## 2. 同一实验 v1 输入和事件

启动输入是一行 UTF-8 JSON，必须以换行结束：

```json
{"schemaVersion":1,"type":"start","requestId":"demo-1","workspaceId":"workspace-1","taskId":"task-1","baseRevision":"revision-1","cwd":"/absolute/disposable/candidate","prompt":"仅验证一次性候选，无真实书籍。","bookPath":"EPUB/ch01.xhtml","selectedText":"假数据","generation":7}
```

`bookPath/fragment/progression/selectedText/generation` 全部是顶层可选字段。generation 为缺省/null/非负 int64，progression 为缺省/null/0～1；三个文本字段为缺省或字符串，允许空串、拒绝 null。事件 envelope 的 generation 缺省/null 均为 null，模型上下文则保留缺省与显式 null 的区别。start 必须包含显式非空 request/workspace/task/revision/prompt 及现存、clean 的绝对 cwd。未知字段忽略。cwd 不是 OS 沙箱。

第一行 start 到达即执行，不等待宿主 stdin EOF。每个宿主进程只执行一个任务。模型文本默认逐字保留原 prompt；仅显式提供任一上述五个字段时，追加 `\n\nKepub task context (JSON):\n` 和只含这些已提供字段的 JSON 对象。空字符串和数值 null 保留，不发送未知字段、内部 request/workspace/task/cwd/revision/绑定信息。不会按 bookPath 读取文件，不会自动附加整书。发送一条 user JSONL 后关闭 Amp stdin。宿主 stdin 仍可收控制消息：

```json
{"schemaVersion":1,"type":"cancel","requestId":"demo-1"}
```

cancel 不传给模型。重复 cancel 幂等；不同 requestId、第二个 start 或非法控制消息失败。宿主 stdin 正常 EOF 仅表示没有更多控制，不自动取消。终止后宿主退出，不消费下一任务或再次发 terminal。调用者必须持续读取 stdout/stderr。

参数/绑定文件解析发生在会话建立前，启动配置无效时只有 stderr 和退出码 2；有效启动后的任务使用 JSONL terminal。stdout 断开无法交付 terminal，宿主处理 EPIPE 并回收进程组后非零退出，不被默认 SIGPIPE 提前杀死。

实际 argv（不是 shell 字符串）：

```text
amp --execute --stream-json --stream-json-input --executor local --visibility private --mode ultra --no-ide --no-color
```

续接另追加 `threads continue T-<uuid>`，绝不使用最近线程/`--last`。不自动启用 `dangerouslyAllowAll`，不改用户配置、不自动安装/更新 CLI。`--no-ide` 阻止自动编辑器上下文；**不代表关闭全局/项目配置、AGENTS、MCP 或插件发现**。

续接必须由可信宿主参数 `--bindings FILE` 提供：

```json
{"schemaVersion":1,"threads":[{"threadId":"T-11111111-2222-4333-8444-555555555555","workspaceId":"workspace-1","taskId":"task-1","cwd":"/absolute/disposable/candidate","baseRevision":"revision-1"}]}
```

Go 逐一核对 threadId/workspaceId/taskId/cwd/baseRevision；缺失、不匹配或重复绑定均拒绝。绑定文件不能来自不可信书籍内容或候选中的 Agent 输出。原型不自动持久化绑定。Amp 续接已有远程线程时未必受 `--executor local` 控制，可信宿主必须仅登记已确认本机/private 的线程；原型没有联网核验线程执行位置/可见性。此项仍是生产接入门槛。

以下是实际离线 smoke 输出中的事件（省略两个 tool 事件，但 sequence 未改写）：

```jsonl
{"schemaVersion":1,"requestId":"demo-1","workspaceId":"workspace-1","taskId":"task-1","sequence":1,"generation":7,"type":"started","data":{"executor":"local","mode":"ultra","threadId":null,"visibility":"private"}}
{"schemaVersion":1,"requestId":"demo-1","workspaceId":"workspace-1","taskId":"task-1","sequence":2,"generation":7,"type":"assistant","data":{"text":"中文🙂分块","threadId":"T-11111111-2222-4333-8444-555555555555"}}
{"schemaVersion":1,"requestId":"demo-1","workspaceId":"workspace-1","taskId":"task-1","sequence":5,"generation":7,"type":"completed","data":{"cleanup":{"confirmed":true,"scope":"process-group"},"code":null,"result":"fixture only","reviewRequired":true,"threadId":"T-11111111-2222-4333-8444-555555555555"}}
```

`assistant.message.content` 的 text 映射为 assistant；tool_use 和 user 中 tool_result 映射为 tool，保留原 content。未知事件/内容块在 result 前忽略；result 后任何事件、缺必需结果字段、线程 ID 漂移或 init cwd 不匹配均失败。sequence 自 1 递增，stdout 不含日志。终止恰好一个；stdout 断开导致无法交付时返回非零，不假称 terminal 已被接收。

### 2.1 requestId 是 SDK API，request_id 才是固定 SDK 发出的 wire

复核了 P4 固定的 `@ampcode/sdk@0.1.0-20260918210405-g81edbf0` [公开 dist/index.js](https://unpkg.com/@ampcode/sdk@0.1.0-20260918210405-g81edbf0/dist/index.js)。`createUserMessage`（第 4612～4623 行）把 `options.requestId` 写入公共对象 `message.requestId`；`serializeUserInputMessage`（第 4552～4559 行）则明确转换：

```js
const validatedMessage = UserInputMessage.parse(message);
return JSON.stringify({
  type: validatedMessage.type,
  request_id: validatedMessage.requestId,
  message: validatedMessage.message
}) + "\n";
```

[npm 固定版本元数据](https://registry.npmjs.org/@ampcode%2Fsdk/0.1.0-20260918210405-g81edbf0) 的 tarball SHA-1 为 `a5dd8e8459341b1ec9a881c7a548851feb1948e2`；包本身依赖 `@ampcode/cli:latest`，不是某个固定 CLI parser 的认证。重新抓取的官方 SDK 文档仍写 `requestId`，描述的是公共 API；**这不是已证实的字段版本改名，而是对象层与 wire 层的转换**。

A 的 CLI stdin 现改为顶层 `request_id`，与固定 SDK serializer 对齐；Kepub start/cancel/event envelope 仍为 `requestId`。模型文本不包含内部 request ID。`TestContextPresenceAndWireRequestID` 反解 wire 并验证没有顶层 `requestId`，缺省、空串、null、选区和 int64 最大值不混淆。

可证范围止于**固定 SDK 实现如何序列化**。本机 CLI `0.0.1791148688-gddc1cf` 的 help 只解释 JSONL flags，不列 request 字段；官方 CLI Streaming JSON 输入 schema 也未列 `requestId` 或 `request_id`。因此本机版本与 P4 固定的 CLI `0.0.1791146283-g5c3f72` 是否接受两种拼写、是否忽略其中一个、是否真正去重，均未验证。没有以 fake 接收或公开 SDK 代码冒充 CLI parser/服务端幂等性证据；本次也未执行真实 Amp。

## 3. 只有清理完成才发 terminal

`completed` 要同时满足：单一协议成功 result、完整 newline/EOF、stdin 成功写入、stderr 完整、有界输出、CLI `Wait` 成功、正常根退出时没有残余受管组成员，以及最终清理确认。收到 result 不杀仍在结束的 CLI；等待正常退出或 task timeout。**根进程正常退出也检查并回收残余进程组，但发现残余成员即 failed/WRITERS_AFTER_CLI**，清理成功只令 `cleanup.confirmed:true`，不能把任务升级为 completed。此处为父复核后的保守标准，取代首版“强杀残余写者后仍 completed”的行为。

回收使用独立 PGID，TERM → 200 ms 宽限 → KILL → 最长 2 s 检查。另给管道/reaping 200 ms 收尾预算；测试缩短为 40/400 ms。直接根进程由 `cmd.Wait` 回收，独立拥有的 pipe 不被 Wait 提前关闭。取消、错误和宿主 SIGTERM/SIGINT 都走相同流程；不使用只杀根进程的 `exec.CommandContext` 代替进程组回收。

- Linux 在 KILL 后检查 `/proc` 中该 PGID 的非 Z/X 成员；僵尸不能写，但宿主不能 `wait` 非自身子进程，孤儿僵尸仍可能由 PID 1 延后回收。不存在活写者不等于进程表完全无记录。
- 根正常退出后的首次残余检查用 `kill(-pgid,0)`，保守包括僵尸；无法观察时 `GROUP_PROBE_ERROR`，不凭可能漏掉并发 fork 的快照判成功。后续即使确认清理完成，任务仍失败。
- macOS 使用 `kill(-pgid, 0)` 的组消失检查，不能证明消失则保守失败；未实机测试。
- 组存活/权限问题/观测错误/管道不结束/根进程未回收时，`failed`、`code:"CLEANUP_UNCONFIRMED"`、`cleanup.confirmed:false`，禁止进入可冻结成功状态。失败候选应保留，不能自动删除或接受。
- 其它失败/取消使用稳定 code，`reviewRequired:false`。正常完成为 `code:null`、`reviewRequired:true`，**不是已接受、已冻结或已通过出版物检查**。threadId 未知时为 null。

**保证范围只有仍属于受管 process-group 的协作进程。** `setsid/setpgid`、独立 daemon、远程工具写者不在组内。测试证明保留管道的 escaped writer 导致保守失败；如果 escaped writer 同时关闭管道，单靠 PGID 无法发现，也不能证明全局无写者。原型不对恶意同用户进程提供隔离。正式冻结还需要上层写租约/候选保护与受控执行边界，不能直接把 `cleanup.confirmed:true` 当成任意后代已停止。

其他明确限制：宿主被 SIGKILL/崩溃时没有持久恢复监督；调用者永久不读 stdout/stderr 时同步输出背压可能阻塞控制处理；没有无条件传输取消保证。生产需解决或约束这些生命周期条件。PGID 是短生命周期 OS 标识，不是工作区锁。

## 4. 与 SDK 方案可比较的故障矩阵

全部为 Linux fake/真实 OS 测试，不是模型联调。`run_test.go` 的 table 子测试名可用于逐项重跑。

| 共同场景 | 测试与独立断言 | 结果 |
|---|---|---|
| 分块 UTF-8 | `TestSharedScenarios/chunked-utf8-streaming-input-unknown-fields-stderr`：宿主每字节输入，fake 每字节输出；精确恢复 `中文🙂分块` | completed |
| 流式输入 | 同测试：宿主输入一直开放到 terminal，fake 已得到完整 user JSONL/EOF；核对完整显式上下文，无 prompt argv/shell 插值 | completed |
| 未知字段/类型 | 上游 unknown field/event/content block；`TestHostExecutableBindingsAndSIGTERM` 增加 start 未知字段 | 兼容 |
| stderr 并行输出 | fake 在 stdout 同时向 stderr 写 190,000 bytes；stdout 仍只有 JSONL | completed |
| 与 B 同量 stderr | `TestMatchingBoundaryScenarios/stderr_parallel`：2,883,584 bytes，8192-byte 阻塞写，全部写出前不发 result；检查 started 存在、finished 不存在 | 快速 STDERR_ERROR；仅保留 1,048,577 bytes，不声称全部排空 |
| 消息超限 | `oversize` 单行超 1 MiB；`total-limit` 总 stdout 超 8 MiB；`stderr-limit` stderr 超 1 MiB；`TestFrameBoundaries` 同时测恰好界限和越界 | STREAM_ERROR / OUTPUT_LIMIT / STDERR_ERROR |
| 无换行原始输出保持存活 | `TestMatchingBoundaryScenarios/unterminated_oversize`：恰好 1,048,576 bytes `x`、无 JSON/换行，之后保持存活；所需换行已无法容纳 | 快速 STREAM_ERROR，不等 EOF 或 TIMEOUT |
| 无 result 的 EOF | `no-result` | MISSING_RESULT |
| 异常 EOF/非法编码 | `truncated` 是完整 JSON 但无换行；`invalid-utf8`；`malformed` | STREAM_ERROR / PROTOCOL_ERROR |
| success 后非零退出 | `nonzero` 在 success result 后退出 7 | EXIT_ERROR，不 completed |
| root exit 先于管道 EOF | `TestMatchingBoundaryScenarios/exit_before_eof`：先发 success result，根退出 23，子进程持 stdout 250 ms | EXIT_ERROR；捕获 root Wait 后回收组，不靠 EOF 才注册退出观察 |
| 结果错误/字段缺失/重复 | `amp-error`、`missing-error-flag`、`missing-result-text`、`duplicate-result`、`wrong-thread` | AMP_ERROR / PROTOCOL_ERROR |
| 取消前/后/重复 | `TestCancelBeforeSpawnAndTimeout` 预取消不启动；`TestCancellationAndDelayedWriters/wait,result-wait` result 前/后重复 cancel | 单一 cancelled，退出 130 |
| 延迟写入子进程清理 | `child-cancel,child-success,child-closed-pipes`：child 忽略 TERM，计划 700 ms 写入；terminal 后等 800 ms 检查文件不存在；最后一种 child 已关闭输出管道 | 无迟延写；取消为 cancelled，正常 root 后残余写者为 failed/WRITERS_AFTER_CLI，cleanup 均确认 |
| 无法确认清理 | `TestEscapedPipeIsNotCompleted` 用真实 setsid 脱组并保留 stdout/stderr；测试自身显式清理 escaped fixture | CLEANUP_UNCONFIRMED |
| 明确 threadId | `TestExplicitThreadBinding`：精确检查 argv，缺绑定及五字段逐项不匹配不启动 CLI | 匹配完成，其余 INVALID_INPUT |
| 真实宿主信号 | `TestHostExecutableBindingsAndSIGTERM` 编译宿主二进制，加载公共 bindings 格式，发送 OS SIGTERM | 单一 cancelled，退出 130 |
| 错误请求/工具缺失/超时 | `TestInvalidControlAndUnavailableCLI`、`TestCancelBeforeSpawnAndTimeout` | INVALID_CONTROL / SPAWN_ERROR / TIMEOUT |
| 测试替身正向对照/输出断开 | `TestDelayedWriterPositiveControl` 证明未被清理时 fixture 确会写文件；`TestClosedHostOutputDoesNotSIGPIPE` 检查真实宿主 EPIPE 路径 | 正向写入；受控退出 1 而非 SIGPIPE |
| 统一模型上下文 | `TestContextPresenceAndWireRequestID`、`TestInvalidContextRejectedBeforeSpawn`：反解只含五字段，区分缺省/空串/null，验证非负 int64（含最大值）及 progression 边界 | 保真；非法值启动前 INVALID_INPUT |

总量阈值为实验策略，不宣称是 Amp 上游限制。单行和总量计数均含空白及换行，`whitespace-limit` 验证空白不能绕过上限；非换行终止的最后一条按异常 EOF，输入封装后再检查 1 MiB。stderr 多读至 1 MiB+1 用于检测超限，随后回收进程。

## 5. 可复现命令

在仓库根执行，不需要 Node、Amp 登录或 API key：

```sh
go test -count=3 ./experiments/amp-cli
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
```

上述普通/race/vet 均在本 orb 执行通过。`TestMain` 只编译本目录的宿主和 fake Go 程序，自动测试不会使用真实 `amp`。生产 `go list -deps ./cmd/kepub` 保持项目自身 + x/sys + x/text，原型只有标准库。

离线 smoke（stdout 5 个事件、stderr 190,000 bytes；自动清理临时文件）：

```sh
set -eu
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
mkdir "$tmp/candidate"
go build -o "$tmp/amp-cli-spike" ./experiments/amp-cli
go build -o "$tmp/fake-amp" ./experiments/amp-cli/testdata/fake-cli
jq -nc --arg cwd "$tmp/candidate" \
  '{schemaVersion:1,type:"start",requestId:"demo-1",workspaceId:"workspace-1",taskId:"task-1",baseRevision:"revision-1",cwd:$cwd,prompt:"仅验证一次性候选，无真实书籍。",bookPath:"EPUB/ch01.xhtml",selectedText:"假数据",generation:7}' \
  | "$tmp/amp-cli-spike" --cli "$tmp/fake-amp" > "$tmp/events.jsonl" 2> "$tmp/diagnostics.log"
cat "$tmp/events.jsonl"
wc -c < "$tmp/diagnostics.log"
```

单独重现取消/延迟写者证据：

```sh
go test -count=1 -v ./experiments/amp-cli -run 'TestCancellationAndDelayedWriters|TestEscapedPipeIsNotCompleted|TestHostExecutableBindingsAndSIGTERM'
```

父复核新增三场景和上下文的独立重跑：

```sh
go test -count=1 -v ./experiments/amp-cli -run 'TestMatchingBoundaryScenarios|TestContextPresenceAndWireRequestID|TestInvalidContextRejectedBeforeSpawn'
```

一次 Linux 非 race 记录：`exit_before_eof` 27 ms、`unterminated_oversize` 34 ms、`stderr_parallel` 15 ms；均 cleanup confirmed，测试硬断言在 2 s 内返回且不是配置的 3 s TIMEOUT。这是受控 fixture 的故障分类证据，不是 Amp 性能基准。190,000-byte 在限内 stderr 场景仍完整完成。

仅交叉编译、不运行（已得到 `Mach-O 64-bit arm64 executable`）：

```sh
tmp=$(mktemp -d)
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -o "$tmp/amp-cli" ./experiments/amp-cli
file "$tmp/amp-cli"
rm -rf "$tmp"
```

## 6. 评选建议与剩余门槛

方案 A 已证明无需新增运行时/包依赖即可实现本次同一矩阵；维护成本主要是 Amp JSON 消息演进和本机进程监督。没有证据时不声称比 SDK 快或更安全，也没有用假模型结果推断真实权限交互。

建议保留 A 作为简化候选，待同门槛比较 P4 的实际 SDK 包参数映射、错误/取消及 Node 打包成本再选型。安全门槛不能因 SDK 帮助解析事件而降低。

仍需用户另行授权的一次性真实 Amp/Mac 验证：新建 local/private Ultra 任务的实际 JSONL、`session_id == threadId`、明确续接绑定、结果/错误和取消顺序、真实工具后代是否留在 PGID、权限询问与 AGENTS/MCP/插件发现、凭证与自动更新隔离、Mac 组回收/僵尸语义、Apple Silicon 干净环境定位 CLI 与打包。仅用 help/schema 和 fake 不能关闭这些门槛。
