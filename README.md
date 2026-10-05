# Kepub

面向 Amp 的 EPUB 阅读、制作预览与编辑工作台。

**当前状态：开发方案 v0.7，路线为 CLI + 外部 Amp 协作 → MyGo UI → UI 内集成 Amp，后续接口仍暂定。第一轮已发布只读 Go 核心与 CLI、工作区库及两种独立 Amp 接入实验。第二轮编辑／审核／导出闭环，以及 Medium 实现的 C1 命令框架、C2 正文查询、C3 单节点简单文本修改已本地集成，尚未发布。下一批 C4 验证外部 Amp 经 CLI 协作，真实模型调用另获授权；当前没有 GUI、生产 Agent、安装包或已完成的 Mac 实机测试。**

## 已实现：M1-A / M1-B1 只读 CLI

需要 Go **1.27.1**。纯核心只依赖标准库、`golang.org/x/text v0.29.0`（Unicode 碰撞检查）和 `golang.org/x/sys v0.36.0`（原子不覆盖发布），不依赖 MyGo、Amp、Calibre、Java 或前端工具链。

```sh
go build -o kepub ./cmd/kepub
./kepub capabilities --json
./kepub info 'book.epub' --json
./kepub inspect 'book.epub' --section manifest --json
./kepub toc 'book.epub' --json
./kepub inspect 'book.epub' --section references --resource 'EPUB/chapter.xhtml' --direction incoming --json
./kepub unpack 'book.epub' --output 'new directory' --json
./kepub info --json -- '-book.epub'
```

- `inspect` 支持 `metadata`、`manifest`、`spine`、`navigation`、`references`、`capabilities`。`toc` 与 navigation 共用读取用例，按声明选择 EPUB3 nav / EPUB2 NCX，不从 spine 合成目录。多 rootfile 必须传 `--rootfile '书/Deep/package.opf'`，值是精确 BookPath，不是 URL。
- 引用边保留源位置、原 href、精确目标路径、query/fragment 和解析器版本；`--resource` / `--direction incoming|outgoing` 只过滤边，保留全局 coverage 与诊断。CSS 仅提取 literal url/import 子集，完整 CSS grammar 为 partial；脚本、SMIL、srcset 等明确不完整。**complete 只表示相应语法的提取覆盖，不是 EPUB 合规或编辑授权。**
- 仅支持 EPUB2/3 ZIP 与 UTF-8 XML（含 BOM、内建/数字实体）；目录输入、UTF-16、DTD、自定义实体、`xml:base`、远程 manifest href 不支持，明确失败而非容错改写。
- 拒绝穿越、绝对路径、特殊/符号链接/加密 ZIP 条目、重复和大小写/Unicode 碰撞（包括隐式父目录）。实际解压限制：20,000 条目、256MiB/文件、2GiB 总量；路径 4096 字节/128 层；解析 XML 8MiB/128 层/200,000 tokens、累计文本/位置索引 32MiB。这些是初始产品策略，不是 EPUB 标准。
- 原书只读，所有资源（含非 manifest 文件与空目录）保留；unpack 完整 staging 后原子发布，不覆盖已有路径。输出父目录必须已存在。Linux 和 macOS 支持原子不覆盖发布；macOS **只做交叉编译，尚未实机验证**。突然终止可能留下私有临时 staging，但不会发布半成品目录；不宣称断电耐久性。
- `--json` 成功/失败 stdout 都是一个统一 envelope；失败用稳定 code，退出码 1 内容/安全问题、2 参数、3 未实现能力、6 I/O。`--output/-o`、`--rootfile`、`--section` 可置于 BOOK 前后，`--` 结束选项，JSON 和 `--no-input` 不问答。其他契约中的选项尚未实现，会拒绝，不忽略。
- 固定版式、脚本标记、SMIL/音视频、签名、加密/字体混淆声明会记录限制；不执行、不解密、不验证签名、不删除内容。缺失 manifest 资源显式报告。成功只表示只读请求完成，**不是 EPUB 合规验证**。

workspace list/注册表、资源改名、GUI、预览、生产 Agent、Mac 发布仍未实现。capabilities 中这些项为 `planned`，不能据此执行。当前 schema 描述已实现命令的参数和结果基本形状，不代表所有操作的细粒度结果 schema 均已完成。

验证：`go test -count=1 ./...`、`go test -race -count=1 ./...`、`go vet ./...`；真实 CLI smoke 含 EPUB2/3、双向引用与错误 envelope。小型 EPUB 与恶意输入由测试生成，无第三方书籍。证据见 [M1-A](docs/verification/M1_A.md) 和 [M1-B1](docs/verification/M1_B1.md)。

## 本地已集成：C1 命令发现与诊断

```sh
go install ./cmd/kepub  # 安装到 GOBIN，未设置时为 $(go env GOPATH)/bin
./kepub --help
./kepub inspect --help --json
./kepub version --json
./kepub doctor --json
```

上面的 `./kepub` 使用前文构建的本地二进制；`go install` 后也可将安装目录加入 PATH，以 `kepub` 调用。构建需要 Go，已构建的 CLI 不要求用户安装 Go、Node、MyGo 或 Amp。正式 EPUB 校验另需 Java 17+ 与固定版 EPUBCheck。

- 命令帮助、capabilities 的增量 `commandSchemas`、必填及允许参数使用同一描述来源；保留原 capabilities 数组和 operation ID。未知选项、重复参数或非法值不会因加 `--help` 被忽略。
- `version` 返回实际 Go 构建／VCS 信息；没有发行标签的构建明确为 development，不联网推测版本。
- `doctor` 区分核心可运行与正式检查器可用。缺依赖仍可成功收集诊断，但 `formalValidationAvailable:false`；外部探测故障明确失败。探测有时间和输出上限，只发现 Amp 路径，不启动 Amp、不验证登录、不安装依赖。

证据见 [C1 CLI 验证](docs/verification/C1_CLI.md)。此批为本地源码交付，不是已发布的安装包或 Mac 实机验收。

## 本地已集成：C2 已接受版本的正文查询

```sh
./kepub workspace open 'book.epub' --output 'work' --json
./kepub content --workspace 'work' --resource 'EPUB/Text/chapter.xhtml' --json
./kepub content --workspace 'work' --resource 'EPUB/Text/chapter.xhtml' --query '目标短语' --limit 20 --json
```

`--resource` 必须是所选 manifest 中 XHTML 的精确 BookPath，不能传 href、fragment 或本机路径。返回 `workspaceId`、`revisionId`、`rootfile`、原资源 SHA-256、结构 locator 和解码文本；始终查询 accepted，即使存在被修改的活动候选。持锁查询，并发占用返回 `WORKSPACE_BUSY`（exit 4）。不需要 Java、Amp、Node 或 GUI。

query 区分大小写、按单个结果元素的文本做字面子串匹配；省略表示全部，显式值须为 1～4096 UTF-8 字节。limit 默认为 50，范围 1～200，另返回完整匹配数和 `truncated`，不会默选第一个匹配。XML 输入上限 8 MiB、返回文本累计上限 1 MiB；超限明确失败，不截断节点文本。

结果排除 script/style/head、外来命名空间子树及含这些内容的祖先。混合内容可读，但不代表允许修改；这不是浏览器可见文本、全书搜索或 EPUB 合规检查，locator 也不是可写偏移。证据见 [C2 内容查询验证](docs/verification/C2_CONTENT.md)。C3 在此基础上增加受限正文操作；这些 CLI 能力本身不等于真实 Amp 联调已通过。

## 本地已集成：C3 受限正文修改

正文请求使用 `schemaVersion:2`，恰好一个 `content.text.set` v1 操作。先运行 `content`，从同一次结果复制 `bookPath`、`revisionId`、`resourceSha256`、`locatorVersion` 和所选节点的 `locator`、`text`；将 `text` 作为 `expectedOldValue`，另填 `newValue`。不能只凭相似文本选择首个匹配。字段与兼容规则见 [CLI 契约 §2.2](docs/CLI_CONTRACT.md#22-c3-受限正文修改实施契约)。

请求仍经下文的 `plan → apply → task diff → task accept/reject → workspace export`，没有直接正文写入命令。只修改 manifest XHTML 的单个 body 后代简单文本元素，支持显式空元素，不支持混合内容、子元素、注释、CDATA、处理指令、自闭合或脚本／样式目标。新值按 XML 文本转义，不解释成标签；目标外字节和 OPF 时间戳保持，no-op 不改字节。读取绑定过期明确拒绝，不自动重新找相似目标。

正文 plan 和 execution 为 v2，已有 `metadata.set` 请求、计划、执行记录与摘要仍保持 v1。diff 的 `content.newValue` 是实际候选文本，不是计划新值；候选漂移可审阅／拒绝，不能接受。`content` 仍只读 accepted，accept 与正式 export 仍须真实 EPUBCheck。正文之外的结构、CSS、批量修改和真实 Amp 联调不在 C3 范围。

父 orb 独立执行 67 次真实 CLI 调用：旧二进制生成的 v1 计划／待审任务兼容、v2 操作／策略摘要、同文第二节点的精确修改、正文接受／正式导出／拒绝、空值／no-op、陈旧读取绑定和候选漂移均通过。导出仅替换预期节点文本，其他字节与原书 SHA-256 保持。证据见 [C3 正文修改验证](docs/verification/C3_CONTENT_EDIT.md)。已知检查器对部分百分号编码中文 href 的报告兼容问题仍会明确失败，C3 未绕过或修复该问题。

## 本地已集成：M1-B2 校验与打包

```sh
./kepub validate 'book.epub' --json
./kepub validate 'publication directory' --strict --timeout 30 --json
./kepub pack 'publication directory' --output 'new-book.epub' --json
./kepub pack 'publication directory' --output 'draft.epub' --draft --json
```

正式检查需要 Java 17+ 和官方 EPUBCheck **5.3.0**，通过 `KEPUB_EPUBCHECK_JAR` 指定主 JAR；orb setup 已准备这些依赖。命令不会自动下载，也不会在缺依赖时降级成草稿。`--strict` 使 warning 也阻止通过；显式 `--draft` 产物始终 `verified:false`。

pack 从明确的出版根冻结完整资源清单，不递归归档工作区；正式产物经过最终 ZIP 校验及前后哈希核对，再原子发布，不覆盖已有输出。输出须位于出版根之外。发布前观察到取消不会生成文件；发布成功后到达的取消不撤销或误报已完成的产物。

报告分别保留必需检查和引用提取 coverage；CSS partial 不自行阻止无结构修改的打包，实际 error/fatal 仍阻止。工具缺失、超时、残留受管进程或不完整报告不能记为 pass。进程组回收不是同用户 OS 沙箱，不覆盖脱组进程；合规通过也不代表阅读器渲染或人工可访问性审核。接口、真实 EPUB2/3 fixtures 和边界验证见 [M1-B2](docs/verification/M1_B2.md)。

父 orb 在干净非交互 login shell 中配置真实 EPUBCheck 5.3.0，最终闭环合并后 `go test -count=1 ./...`、`go test -race -count=1 ./...`、`go vet ./...` 全通过；普通／race 的 CLI 为 87.583s / 81.906s，workspace 为 108.225s / 108.602s，validation 为 151.116s / 142.217s。Darwin arm64 全仓交叉编译通过，没有执行。普通未配置 checker 的测试会跳过外部检查用例，不能代替上述验证。独立 SDK 模块另跑 `npm run check` 与 `npm test`：31/31 Node 场景及 Go race 通过，既有 SDK 失败门槛结论不变。

## 本地已集成：M2-B 确定性编辑与审核导出

`internal/workspace` 提供 Create/Open/Close、唯一独立候选、Checkpoint/Checkpoints/Restore 和精确内容树 SHA-256。原书、初始版本、候选与检查点均为独立副本；flock 保证协作进程单写者，恢复 journal 处理已记录的中断边界。候选普通写入不会污染基线；已知不支持内容和书内 Agent 配置保留但限制候选创建。

元数据 v1 计划仅修改一个唯一选中的现有 `dc:title` 或 `dc:creator` 简单文本，匹配预期旧值；C3 正文 v2 计划见上节，两类均每计划一个操作。目标外 OPF 与资源字节不变，no-op 不重排 XML 或更新时间。计划绑定工作区身份、绝对路径、当前 accepted revision、完整树和策略；apply 重新计算，失败回滚，成功仍是 `review_required` / `conformance:not_run`，不是自动接受。

将下面请求保存为工作区外的 `operations.json`，并把旧值替换为书内的实际标题。若有多个标题，须提供明确的可选 `id`，不能自动选择第一个。

```json
{
  "schemaVersion": 1,
  "operations": [{
    "operationId": "metadata.set",
    "operationVersion": 1,
    "params": {
      "namespace": "http://purl.org/dc/elements/1.1/",
      "localName": "title",
      "expectedOldValue": "原书标题",
      "newValue": "修订标题"
    }
  }]
}
```

```sh
./kepub workspace open 'book.epub' --output 'work' --json
./kepub plan --workspace 'work' --operations 'operations.json' --output 'plan.json' --json
./kepub apply --workspace 'work' --plan 'plan.json' --json
# 将 TASK_ID 替换为 apply 返回的 data.taskId，先审阅再接受
./kepub task diff TASK_ID --workspace 'work' --json
./kepub task accept TASK_ID --workspace 'work' --json
./kepub workspace export 'work' --output 'edited.epub' --json
```

- 不接受修改时，在 accept 前执行 `task reject TASK_ID --workspace work`；记录与检查点保留，随后可生成新计划。已消费计划不能重跑，旧 task ID 不能操作新的候选。
- accept 对独立冻结树运行正式检查，通过后生成独立 revision；下一次编辑使用新基线。检查尝试与接受决策分开记录；最终取消检查位于持久 journal 发布之前，意图提交后重开只向前完成。没有草稿接受或既有错误豁免。
- export 只读取当前 accepted，而不是 active 候选；初始导入不带合规通过状态。正式导出重新检查最终 ZIP，显式 `--draft` 才允许未正式验证的草稿。候选被普通外部写者改动后可以重开 diff/reject，但不能沿用旧执行状态接受。
- 工作区用明确目录定位，不使用全局注册表或 latest。open 只新建，不覆盖；plan 报告和 export 输出必须在整个工作区根之外、父目录已存在且输出不存在。操作／计划文件拒绝链接和特殊文件，FIFO 无 writer 不阻塞。

父 orb 另用含 BOM/CRLF/注释、特殊字符及空目录的自制 EPUB3 执行 27 次真实 CLI 调用：两次接受推进基线、旧计划拒绝、未接受内容隔离、缺 checker 拒绝、候选新增文件的 diff/reject、no-op、FIFO 与输出保护均通过。导出仅含预期两处 OPF 文本替换，其他文件字节完全一致；原书 SHA-256 未变。外部写者必须先停止；flock 与 cwd 不是同用户 OS 沙箱，断电和 ENOSPC 未证明。接口、恢复边界与 EPUB2/3 验证见 [M2-A](docs/verification/M2_A.md) 和 [M2-B](docs/verification/M2_B.md)。

## Orb 启动

`.agents/setup` 仅支持 Linux amd64，固定 Go 1.27.1，从根及 SDK 实验的锁文件副本预热/验证 Go 缓存。存在 SDK 实验 npm 锁时，复用精确 Node 26.10.0 / npm 10.9.9，否则校验官方归档后局部安装；`npm ci` 从固定锁和完整性校验缓存准备实验依赖。没有 SDK 锁时不安装 Node/npm。所有工具链通过非交互 login shell 可用，不改系统 Node 或仓库锁文件。

第二轮 setup 另准备 Java 17+ 与官方 EPUBCheck 5.3.0：验证完整下载包，保留全部依赖 JAR，暖运行核对精确文件集合和内容，并持久提供 `KEPUB_EPUBCHECK_JAR`。这只是开发/正式检查依赖，不使只读 CLI 依赖 Java。缺失时才安装 Debian JRE；其安全更新版本不伪称为固定 patch。

`.agents/resume` 只修复 Go 链接与快速检查工具/依赖，不安装、不认证、不启动服务。已有 Go/SDK 的父 orb 补装 Java/checker 为 10.92s；最新暖 setup 3.14s、resume 0.44s，锁文件未变。额外 JAR 被 resume 拒绝，setup 重装修复 4.73s，干净 login shell 验证通过。证据见 [EPUBCheck 环境验证](docs/verification/ORB_EPUBCHECK.md)，此前 Go/SDK 的空 HOME 测试见 [原 setup 验证](docs/verification/M2_A.md#独立后续sdk-实验的-orb-setup-验证)。不预装 Calibre/GUI，不是 Mac 安装器；第二轮 setup 仍待推送默认分支后才影响未来 orbs，尚未证明新服务端快照。

## 文档

- [开发方案 v0.7](docs/DEVELOPMENT_PLAN.md)：CLI／UI／Amp 三阶段、具体开发批次、正文编辑边界、Medium 分工与验收。
- [CLI 与操作契约](docs/CLI_CONTRACT.md)：拟定命令、OperationRegistry、plan/apply、机器输出、退出码与验收要求。
- [Calibre CLI 与编辑内核研究](docs/research/CALIBRE_CLI_REVIEW.md)：官方命令全景、关键源码调用链、证据及采用/不采用的设计。
- [开发方案 v0.2 历史原文](docs/history/DEVELOPMENT_PLAN_V0_2.md)：Calibre 研究后形成的上一版设计。
- [开发方案 v0.1 历史原文](docs/history/DEVELOPMENT_PLAN_V0_1.md)：最初的完整方案。

## 产品方向

首发只适配 Apple Silicon Mac，后期再考虑跨平台。**已确认采用 MyGo 与系统 WebView**，当前接口基线为 MyGo 0.2.0，macOS 使用 WKWebView，不捆绑 Chromium。主编辑窗口计划使用 Go + TypeScript，MyGo 网页控制通过 `Window.Page()` API；React + Vite 仍是前端计划，尚未实现。MyGo 纯 Go 原生 UI 只作为独立设置、诊断、检查器等辅助窗口的候选。独立 `kepub` CLI 与 GUI 共用 Go EPUB 核心。

Bridge 是系统 WebView 的脚本通信适配，与系统 WebView 本身不冲突；框架选型确认不代表预览安全已经通过。MyGo 0.2.0 公共网页窗口的顶层页面会注入 bridge，另开窗口不等于无桥或独立存储；导航拦截也不等于禁止所有网络请求。[边界实验](experiments/mygo-boundary/README.md)已在真实 Linux WebKitGTK 发布模式复现同源 iframe 经 `parent.mygo` 调用 Go，并验证受限 sandbox 对照；这不是 Mac 或完整 EPUB 预览验收。出版物隔离仍须在 MyGo／WKWebView 适配内解决并完成 Apple Silicon release 实测，详见 [预览安全边界](docs/DEVELOPMENT_PLAN.md#62-可信壳与不可信出版内容)。

Amp 负责内容理解与编辑；Kepub 提供不依赖模型的确定性 EPUB 操作，并管理工作区、预览、差异、检查、审核和导出。Calibre 是参考和可选适配，不是运行核心功能的必需依赖。

```text
打开 EPUB → 阅读并定位
                ↓
     确定性操作 或 Amp 编辑
                ↓
       候选结果 ↔ 实时预览
                ↓
 差异 + 检查覆盖 → 审核接受 → 导出
```

## 历史：v0.4 Amp 接入对照实验

Go 仍负责出版物、工作区、写租约、候选冻结、审核和导出。此前已并行比较 **A：Go 直接管理 CLI/JSONL** 与 **B：Go 监督 Node/TypeScript SDK 辅助进程，再调用 CLI**，采用相同输入/事件约定和取消、异常退出等故障场景，后续择一进入生产适配。SDK 不放入 WebView，也不成为只读 CLI 的依赖。

第一轮四条 Ultra 工作线已经完成：M1-B1 目录/引用/覆盖、M2-A 工作区快照与单写者、M0-A CLI 原型、M0-B SDK 原型。实现目录与依赖边界见 [第一轮分工](docs/DEVELOPMENT_PLAN.md#111-第一轮并行工作与文件所有权)。第二轮改用 Medium，M1-B2 校验/打包和 M2-B 元数据计划/候选/diff、审核与导出也已本地集成；范围见 [第二轮记录](docs/DEVELOPMENT_PLAN.md#112-第二轮medium-并行实现随后集成编辑闭环)。原型成功不等于生产 Agent 已接入；真实 Amp 与 Mac 实机验证单独列为门槛。

实验结论为 **A 优先进入后续受管 Agent 验证，B 暂不晋级**。固定 SDK 实包复现 stderr 排空、退出早于 EOF、原始未结束消息无大小上限的缺口；其测试通过代表成功复现并收敛失败，不是协议门槛通过。A 无需新增运行时，已通过 Linux 受控协议与进程测试，但两者都未验证真实 Amp、配置/权限发现、任意脱组写者或 Mac 回收。证据与离线命令见 [CLI 原型](docs/verification/AMP_CLI_SPIKE.md) / [SDK 原型](docs/verification/AMP_SDK_SPIKE.md)；后者为独立模块，根目录 `go test ./...` 不运行其 Node/Go 测试，须在 `experiments/amp-sdk` 执行 `npm test` 和 `npm run check`。

编辑器集成只借鉴文档/选区/诊断与 Agent 分离的职责，不复刻已停用 VS Code 侧栏或其私有协议。后续如需实时工具交接，再在 OperationRegistry 稳定后评估公开 MCP/插件 API。

## 历史：v0.3 的关键变化

MyGo 0.2.0 已正式发布。Kepub 因此固定新的桌面接口假设：网页能力从 `Window` 迁移到 `Page`；Preview 的加载、刷新、导航和崩溃恢复统一使用 `Window.Page()`。页面就绪不能只检查 `document.readyState`，而要绑定 workspace/task/generation/bookPath 并完成目标视图握手。

MyGo 新增的纯 Go native UI 在 macOS 使用 Metal/Core Text，但它不是 EPUB 阅读引擎，也不能从“同一应用可混用两种窗口”推导出“同一窗口可任意混排 native UI 与 WKWebView”。首发主窗口继续使用 Web UI + WKWebView；native UI 只作为独立辅助窗口候选。

CEF 没有随 MyGo 0.2.0 正式发布；公开 CEF PR 当前面向 Linux 且仍未合并，因此不进入 Apple Silicon 首发基线。

v0.2 的 Calibre 优化继续保留：转换、整理、结构编辑和只读查询分开；GUI、CLI 与 Agent 共用 OperationRegistry；原始 EPUB、accepted revision 与候选任务分离；`apply`、`accept`、`export` 保持不同语义。

## 基本约束

- 原书默认不覆盖；未修改资源尽量保持原字节，结构修改必须维护引用。
- 书籍页面不可信，不能拥有应用的文件或进程权限。
- 配置、聊天记录、检查点和工具文件不进入 EPUB。
- 正式导出以冻结归档及明确版本的 EPUBCheck 报告为依据。
- 项目名 Kepub 不表示默认输出 Kobo KEPUB，默认仍是普通 EPUB。

## 开发路线：CLI 与外部 Amp → UI → UI 内 Amp

| 阶段 | 开发批次与终点 |
|---|---|
| 1．CLI + 外部 Amp | C1 完善命令／能力契约和 doctor；C2 有界内容读取／定位；C3 单个 XHTML 简单文本的确定性编辑；C4 真实 Amp 经 CLI 生成候选、展示差异，用户决定接受／拒绝和导出 |
| 2．MyGo UI | U1 在 Apple Silicon 验证 WKWebView 隔离／生命周期；U2 复用 Go 用例，实现导航、阅读、受限编辑、候选预览和可视审核，无需 Amp 登录 |
| 3．UI 内集成 Amp | A1 由 Go 管理 Amp CLI，加入任务／线程绑定、进度、取消回收、冻结与失败恢复；仍不自动接受 |

先验证 **Amp → Kepub CLI**，不必先引入 SDK 或 MCP；**Kepub → Amp** 的受管执行不再作为 UI 前置。正文操作必须扩展已有计划、执行来源与恢复校验，不是直接写候选文件；CSS 写入、结构改名、批量修改另行验收。CLI 与桌面可分别发布，但均须通过对应的 Mac 安装与依赖检查，签名公证和发布另获授权。

当前 `task accept` 不能鉴别人类和 Agent，“先询问用户”是合作约定，不是权限隔离。GUI 不通过拼接命令或解析终端文本复用业务逻辑，而与 CLI 共用 Go 用例。

参考 Obsidian CLI 的命令发现、目标选择、查询与诊断，但 Kepub 保持无 GUI 可运行，不照搬当前活动文件或任意 eval；具体见 [CLI 构建取舍](docs/DEVELOPMENT_PLAN.md#91-参考-obsidian-cli但保持真正-headless)。[Amp 接口边界](docs/DEVELOPMENT_PLAN.md#813-external-apicli-与-typescript-sdk-的适用边界)另区分 SDK／CLI 的 Agent 执行与 External API 的工作区数据管理，后者不是发送编辑 prompt 的入口。

依赖、范围和验收见 [v0.7 开发批次](docs/DEVELOPMENT_PLAN.md#113-v07-开发批次与依赖)。**Medium** 已先并行完成 C1 与 C2 读核心，再单一写者接入 C2 CLI 和 C3 编辑闭环；父线程复核并独立验收。本批接口约束见 [CLI 契约 §2.1／§2.2](docs/CLI_CONTRACT.md#21-c1c2-本批实施契约)。M0～M6 仅保留为技术工作包编号；本地实现不等于真实模型联调或远端发布。

除上面明确列出的命令、库与独立实验外，设计文档中的命令和接口仍待实现；不能将其他设计示例当作当前安装使用说明。
