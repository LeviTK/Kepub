# P4 / M0-B：Amp TypeScript SDK 辅助进程原型

验证日期：2026-10-05（Asia/Shanghai）。范围仅 `experiments/amp-sdk/` 和本报告；基于父线程 v0.4/M1-A bundle 的本地基线，未接入生产 CLI、核心或 WebView。

**结论：可运行对照原型已完成，B 不通过本轮晋级门槛。** 真实固定 SDK 的加载、参数映射及受控 Linux 进程组回收已验证；stderr 并行排空、退出事件时序、原始消息大小上限存在可复现缺口。测试断言这些缺口被拒绝/超时收敛，所以测试套件通过不代表所有评选门槛通过。建议在 A 通过同等数据保护测试的前提下优先 A，不为 B 引入生产 Node 运行时。

## 可复现安装与运行

全部命令在 `experiments/amp-sdk/` 执行；不要在仓库根安装 npm 依赖。启动需要 Linux、Node **26.10.0**、npm **10.9.9**；构建监督程序还需要 Go **1.27.1**。`.node-version`、严格 npm engines 和运行时预检拒绝版本漂移；不安装或修改全局 Amp。

```sh
npm ci
npm run versions
npm test
npm run check
npm run demo

# 单独重现失败门槛，事件为 failed/TIMEOUT，命令预期退出 1：
npm run demo -- stderr_parallel
npm run demo -- exit_before_eof
npm run demo -- unterminated_oversize

# 单独测试取消边界：
node --test --test-name-pattern='AbortSignal boundary|delayed_writer|cancel_after_result' test/spike.test.mjs

# 普通 Go 检查；npm test 已含 -race 单测：
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
```

`npm test` 先构建 TypeScript 和 Go，再运行实包/受控进程测试。`demo` 强制用测试 preload 重定向 CLI，临时候选结束后删除，不调用付费模型。真实 CLI 仅用于 `--version` 和手动核对 `--help`，没有执行真实 prompt、登录、建线程或发送书籍。安装需要 npm registry 网络；安装完成后的 fake 测试不需要模型服务或账号。

原型的实际入口为 `dist/supervisor --node /绝对路径/node --helper /绝对路径/dist/helper.js [--bindings FILE]`，stdin 为 §8.1.1 JSONL。**无测试 preload 时它会运行真实 CLI，可能产生模型费用和服务端传输；本轮没有做这项联调。** 不要用用户书籍来替代 fixtures。

## 锁定的是 SDK、CLI 与运行时组合

| 组件 | 固定值与证据 |
|---|---|
| SDK | `0.1.0-20260918210405-g81edbf0`，安装包仍声明 `@ampcode/cli: latest` |
| CLI | `0.0.1791146283-g5c3f72`，直接精确依赖 + npm `overrides`，不是仅 pin SDK |
| 原生 CLI 包 | lock 中包含同版本 `cli-linux-x64/arm64`、`cli-darwin-arm64/x64`、`cli-win32-x64` 的 tarball URL/SHA512 integrity |
| Node / npm | `26.10.0` / `10.9.9`，本轮 Linux x64；Node 不嵌入 Go 二进制 |
| TypeScript / Node types | `7.0.2` / `26.6.4`，独立锁文件 |
| Go | 独立实验 module，Go `1.27.1`，只使用标准库，根依赖无变化 |

`npm run versions` 实际输出 `sdkLoaded:true`、`sdkResolvesPinnedCLI:true`。`npm ls @ampcode/sdk @ampcode/cli` 显示 SDK 使用 `deduped` 的上述 CLI；脚本从 SDK 模块位置解析 CLI，断言不是 PATH 上另一版本。Linux CLI 二进制 `--version` 为上述固定值，SHA256：

```text
f62bb5b17d10efbadd32552a0f5d41930da954f3724122ed6576d07c3f450314
```

CLI 安装脚本从固定 optional platform package 放置本地二进制，不应使用 `--omit=optional` 或 `--ignore-scripts`。辅助进程预检 SDK/CLI 元数据版本、Node 版本与本地可执行文件，避免 SDK 悄悄退回 `AMP_CLI_PATH`/PATH。`runtime-settings.json` 明确关闭 Amp 自动更新和全权限，测试验证 SDK 传入该路径。安装 integrity 与 `versions` 是组合复现证据，不是每次启动对任意恶意替换二进制做安全认证；配置发现/覆盖行为尚未做真实 CLI 集成验证。

## 协议与进程边界

```text
stdin start/cancel JSONL
        │
        ▼
Go：绑定复核、envelope、deadline、独立进程组、Linux subreaper
        │ 私有有界 IPC
        ▼
Node：execute({prompt: AsyncIterable, signal, options})
        │ 真实固定 @ampcode/sdk
        ▼
CLI：测试时仅在 spawn 边界替换为 fake CLI
        └── 受控延迟写者（同进程组）
```

- 一个监督程序/辅助进程只执行一个任务；保留 `requestId/workspaceId/taskId/generation`，sequence 从 1 严格递增，terminal 恰好一个，日志不进入 stdout。首行无法解析/超限、尚无可信任务标识时仅 stderr 报错并退出 2，不虚构任务 envelope。
- 顶层 `bookPath/fragment/progression/selectedText/generation` 可传入；原型不自行读出版文件，也不把这些字段自动拼入模型 prompt。只有明确 `prompt` 通过 SDK `createUserMessage` 发往流式 stdin。测试包含中文、换行、`$(touch injected)` 与形似选项文本，验证它们仍是数据。
- 显式 SDK 选项为 `visibility:'private'`、`executor:'local'`、`mode:'ultra'`；续接只有 `continue:明确threadId`。固定 SDK 对 `local` **不输出 `--executor local`**，而是依赖 CLI 的本机默认路径；真实 CLI 配置及服务端既有线程执行器仍待验证，不能据 fake 声称强制远端线程变成本地。
- `--bindings FILE` 是可信宿主输入，格式为 `{"schemaVersion":1,"threads":[{"threadId":"...","workspaceId":"...","taskId":"...","cwd":"绝对规范路径","baseRevision":"..."}]}`。Go 在启动前比对全部字段，拒绝缺失、重复、不同任务/版本/cwd 的续接。不是从候选内容中学习绑定，也不继续最近线程。
- 开始控制与 Node IPC 每行最多 64 KiB，非法 UTF-8/无换行 EOF 拒绝；辅助消息最多 4096 条/累计 8 MiB，Go 排空 Node stderr 并最多打印 16 KiB。**这些不是 SDK 内部 CLI stdout/stderr 的原始传输上限。** Node V8 堆设 128 MiB 只是故障 containment，不是整个进程/CLI 子树 RSS 限额。
- 正常成功需有效 success result、SDK 迭代完整正常结束、Node `Wait` 成功、进程组清空。`cliExitEvidence:'sdk-iterator-validated-zero'` 说明只依赖已核查 SDK 的 `waitForProcess/throwIfProcessFailed` 语义，未伪造原始 CLI exitCode；独立记录 `helperExitCode:0`。仍有子写者时先杀并回收，但结果是 `failed/WRITERS_AFTER_SDK`，不把强制停止写者后的候选升级为成功。
- 取消先通过 IPC 调用 `AbortController.abort()`，100ms 后组 TERM，再 100ms 后组 KILL；Node 提前退出则直接进入组清理。Linux `PR_SET_CHILD_SUBREAPER` 收养孤儿，Node `Wait` 后才 `wait4(-pgid)`，直到 `kill(-pgid,0)` 返回 ESRCH。无法确认则 `failed/CLEANUP_UNCONFIRMED`；失败/取消的 `reviewRequired:false`。
- 仅证明 `cleanup.scope:'process-group'`，不是操作系统沙箱。`setsid`、组外服务、外部写者、宿主自身 SIGKILL/机器断电不受此证明覆盖；未实现生产恢复 journal、写租约或冻结，不能据此接受/导出出版物。宿主必须持续消费 stdout/stderr。

`npm run demo` 的实际事件形状：

```jsonl
{"schemaVersion":1,"requestId":"demo-1","workspaceId":"ws-1","taskId":"task-1","sequence":1,"generation":null,"type":"started","data":{"executor":"local","visibility":"private","mode":"ultra"}}
{"schemaVersion":1,"requestId":"demo-1","workspaceId":"ws-1","taskId":"task-1","sequence":2,"generation":null,"type":"assistant","data":{"text":"中文🙂 café","threadId":"T-11111111-2222-4333-8444-555555555555"}}
{"schemaVersion":1,"requestId":"demo-1","workspaceId":"ws-1","taskId":"task-1","sequence":3,"generation":null,"type":"tool","data":{"id":"tool-7","input":{"bookPath":"Text/章.xhtml"},"name":"fixture_read","phase":"use"}}
{"schemaVersion":1,"requestId":"demo-1","workspaceId":"ws-1","taskId":"task-1","sequence":4,"generation":null,"type":"completed","data":{"reviewRequired":true,"code":null,"threadId":"T-11111111-2222-4333-8444-555555555555","cleanup":{"scope":"process-group","confirmed":true},"cliExitEvidence":"sdk-iterator-validated-zero","helperExitCode":0,"result":"fixture only"}}
```

## 同名故障矩阵与实包证据

`test/redirect-cli.mjs` 只替换 `node:child_process.spawn` 的目标，保留真实 ChildProcess/stdio/AbortSignal/退出行为，未知启动一律拒绝；不改 SDK execute、解析器、参数构造或等待逻辑。SDK 的 `AMP_CLI_PATH` 不能覆盖已安装本地 CLI，因此不能用它作为这里的 fake 入口。发行包公开的 `__testing` 只有输入序列化，并无可注入执行器；本实验没有把私有 test API 当生产能力。

| 场景（测试名） | 观测 | 门槛判断 |
|---|---|---|
| 分块 UTF-8 `chunked_utf8` | 每字节分块，中英文/astral emoji 准确还原 | 通过受控测试 |
| 流式输入 `stream_input` | 真 SDK 将消息变成 `--stream-json-input` 的 JSONL，`request_id` 正确，prompt 不进入 argv | 通过；不等于出版事务幂等 |
| 未知字段 `unknown_fields` | 额外字段/未知事件不影响有效 result，未给 generation 时 null | 通过 |
| stderr 并行输出 `stderr_parallel` | fake 尝试写 2,883,584 bytes 并等待 drain；stdout 未结束时 SDK 没有读取 stderr，未达到完成标记，宿主 1.8s TIMEOUT 并清理 | **失败门槛**，不是正常排空成功 |
| 消息超限 `message_limit` | 完整 70,000 字符消息经 SDK 解析后，被包装层 MESSAGE_LIMIT 拒绝 | 后置上限通过；原始读取上限未通过 |
| 未结束超限 `unterminated_oversize` | 1 MiB 无换行输出已送入管道，未触发原始 64 KiB 限制，只靠 TIMEOUT | **失败门槛** |
| 无 result EOF `no_result` | CLI 退出 0，包装层 MISSING_RESULT | 拒绝正确 |
| 异常 EOF `truncated_json` | JSON 截断，SDK_EXECUTION | 拒绝正确，SDK 原始错误串不泄露到 terminal |
| success 后非零 `success_nonzero` | result 后退出 23，SDK_EXECUTION，没有 completed | 正常退出时序通过 |
| 先退出后 EOF `exit_before_eof` | CLI exit 23 已被测试观察者记录，继承 stdout 的子进程晚 250ms 关闭；SDK 等不到已发出的 exit，TIMEOUT | **失败门槛**；不能可靠取得所有退出顺序的结果 |
| 取消前/后及重复 `cancel_before/cancel_after/repeat_cancel` | start 紧跟 cancel、收到响应后 cancel、重复 cancel 均只有一个 cancelled | 通过受控组测试；取消不保证此前零写入 |
| result 后取消 `cancel_after_result` | 已收到 success，CLI 尚未退出时取消，不发 completed | 通过 |
| 延迟子写者 `delayed_writer` | CLI/写者忽略 TERM，Go 升级 KILL；PID ESRCH，terminal 后 750ms 心跳不变且无迟延写入 | 通过受控 Linux 组测试 |
| 成功后残留写者 `success_writer` | SDK 已正常结束但子写者仍在，Go 杀/回收并报 WRITERS_AFTER_SDK | 不冒充可冻结成功 |
| AbortSignal 独立边界 `AbortSignal boundary` | CLI 因 SIGTERM 退出，组 TERM 前写者心跳仍增长；随后 Go 回收 | **SDK 单独不足**。测试仅用 400ms Node event-loop hold 暴露观察窗口，未修改信号/SDK行为 |
| 明确 threadId `explicit threadId` | 正确 binding 映射 `threads continue T-...`；缺失/四字段不匹配均在启动前拒绝 | 通过受控测试；真实续接未测 |

另有独立 Node helper 从不读取 stdin 的监督测试：60 KiB start 堵塞写入时，Go 仍按 deadline 杀/回收并返回 TIMEOUT。输入写入在单独 goroutine 保序执行，不阻塞取消/超时循环。

源码定位以锁定 npm 发行包 `node_modules/@ampcode/sdk/dist/index.js` 为准：`execute` 在 `yield* processOutputStream(...)` 后调用 `waitForProcess`；后者此时才添加 stderr data/exit listener，未先检查 `exitCode`；`spawnAmpCli` 把 signal 给单个 spawn；`killProcess` 只 kill 直接子进程；`processOutputStream` 使用 readline/JSON.parse，无原始行大小限制。上述位置由发行包而不是旧文档或自写 mock 推导，再用实包运行复现。没有宣称真实 Amp 服务已经出现这些故障。

## 验证状态与 Apple Silicon 缺口

本轮 `npm ci`、`npm test`（25 个 Node 测试；Go race 单测）、`npm run check`（TypeScript noEmit + Go vet）通过。另将监督程序用 `go build -race` 构建后重跑完整 Node 集成矩阵，并通过根模块 `go test -count=1 ./...` / `go vet ./...` 回归。安装审计报告 0 vulnerabilities，但不代表安全认证。`GOOS=darwin GOARCH=arm64 go build` 通过，仅证明编译；非 Linux 运行分支明确拒绝建立未经验证的回收保证，不假装 macOS 已支持。

| 已测 | 未测/未实现 |
|---|---|
| Linux x64、固定真实 SDK 加载/映射、fake CLI 协议与进程组/孤儿回收 | 真实模型调用、真实 Amp 工具树、权限提示/断网/登录恢复、真实新建/继续线程 |
| CLI 原生 Linux 二进制 `--version/--help`，安装链与锁文件 | Apple Silicon 干净机器安装、Node arm64 分发、CLI arm64 实际启动 |
| Go → Node → SDK → CLI 对照路径 | macOS 进程组退出/孤儿回收、app bundle 路径发现、无 shell PATH 启动、签名/公证/隔离属性与 SDK/CLI 商业许可分发审查 |

开发安装的 `node_modules` 约 166 MiB、`dist` 约 4.7 MiB（Linux 观测，包含开发依赖、未计外置 Node），不是 Apple Silicon 发布体积或运行内存指标。干净机器不应要求 Go/TypeScript 编译器，但必须提供已构建 Go 监督程序、合适架构的 Node、helper、SDK/CLI 运行依赖、package/version 元数据与固定 settings 文件；本原型尚未完成此发布包。

参考：官方 [TypeScript SDK API](https://ampcode.com/docs/sdk/typescript)、精确版本 [SDK npm 包](https://www.npmjs.com/package/@ampcode/sdk/v/0.1.0-20260918210405-g81edbf0)、[CLI npm 包](https://www.npmjs.com/package/@ampcode/cli/v/0.0.1791146283-g5c3f72)。依赖升级须重跑同矩阵并重新评估，不能只更新 lock 后沿用本轮结论。
