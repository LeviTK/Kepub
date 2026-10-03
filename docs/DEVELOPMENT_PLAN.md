# Kepub 开发方案

> 文档版本：0.1　日期：2026-10-03
>
> 状态：架构与实施计划，尚未实现或完成端到端验证。
>
> 第一阶段平台：Apple Silicon Mac。桌面端采用 MyGo + Go + TypeScript + WKWebView；同时提供独立、无界面的 `kepub` CLI。Amp 负责内容编辑，Kepub 负责 EPUB 工程、预览、审核、校验和导出。
>
> 本文中的 `kepub` 命令、目录、接口、性能指标与里程碑均为拟实现的契约，不代表仓库中已经存在这些能力。外部产品的能力依据见文末来源；生产版本必须通过 M0 的实机验证并锁定依赖。

## 1. 产品目标与范围

Kepub 是面向 Agent 的 EPUB 阅读、制作预览与编辑工作台，不是重写 Sigil，也不是把通用代码编辑器完整搬进阅读器。核心使用过程为：

```text
打开 EPUB → 建立工作区 → 阅读并定位章节
                         ↓
                   向 Amp 下达任务
                         ↓
              编辑任务副本 ↔ 实时预览
                         ↓
                  查看变更与校验结果
                         ↓
                接受或撤销 → 导出 EPUB
```

用户可以只使用阅读和预览功能，不登录 Amp；也可以只使用 CLI 解包、检查、打包，不启动窗口。GUI 与 CLI 必须操作同一套 EPUB 核心，不能各自实现一套解析和保存逻辑。

### 1.1 已确定的方向

- 首发只提供 `darwin/arm64`，不做 Intel、Universal Binary、Windows 或 Linux 安装包。后续跨平台通过适配层扩展，而非当前实现要求。
- MyGo 为首选桌面壳，但不成为 EPUB 核心的依赖。MyGo 目前为 v0.1 系列，必须设置可替换边界。[S1]
- Amp 是第一且唯一需要实现的 Agent。MVP 用本地 CLI 的结构化流，不强制安装 Python SDK 或 Node SDK。[S4][S5]
- EPUB 工作文件是内容事实来源；OPF、目录、引用图等内存对象是可重建索引，不拥有另一份独立可写内容。
- 实时预览不等于自动接受修改。Agent 可以持续修改任务副本，原始 EPUB 和已接受版本必须受到保护。
- CLI 与 GUI 平级，CLI 的无界面命令不能初始化 MyGo、AppKit 或 WKWebView。

### 1.2 第一阶段功能边界

| 功能 | MVP 范围 | 明确不承诺 |
|---|---|---|
| EPUB 输入 | 未加密、以可重排 XHTML 为主的 EPUB 2 / EPUB 3 | 任意损坏、任意加密或任意复杂出版物都能编辑 |
| 阅读预览 | 章节、目录、内部链接、原书样式、阅读位置、滚动阅读 | 完整分页阅读系统、与所有阅读器完全一致 |
| 内容渲染 | XHTML、CSS、常用图片、SVG、普通嵌入字体；竖排、ruby、RTL 列入样本测试 | 浏览器能够显示的格式必然是 EPUB 核心媒体类型 |
| Amp | 本地任务、多轮衔接、取消、变更列表、错误反馈 | 云端 runner/orb、自动发布、无限循环修复 |
| 编辑 | 现有 XHTML / CSS，受控目录修改 | MVP 中任意资源新增、删除、改名和结构重构 |
| 导出 | 独立输出文件、最终归档校验、可追溯报告 | 默认覆盖原书、仅凭预览正常就认定合规 |
| CLI | 查看、解包、打包、校验、工作区、任务、预览入口 | 第一版实现庞大命令全集 |

固定版式、Media Overlays、书籍脚本、复杂音视频和不支持的加密资源要被识别并提示。不能悄悄丢弃后再宣称支持。第一版不提供这些类型的完整编辑保证，可进入受限只读或拒绝编辑状态。

最低 macOS 版本暂以 **macOS 15** 为实施建议，M0 根据实机、MyGo 与渲染测试冻结。只支持 Apple Silicon 并不消除不同 macOS/WebKit 版本之间的差异。

## 2. 技术决策与替代方案

### 2.1 默认选型

| 层 | 选择 | 理由及边界 |
|---|---|---|
| EPUB 核心 | Go 独立 package | ZIP、XML、文件、进程、服务接口集中实现；不引用桌面框架 |
| 桌面壳 | MyGo | macOS 使用 WKWebView，并提供类型化调用、事件和流式 channel；先验证安全和生命周期边界。[S1][S2][S3] |
| 前端 | TypeScript + Vite + React | 实施默认值，不要求引入完整编辑器、终端模拟器或大型组件系统 |
| 预览 | WKWebView 内隔离的出版物视图 | 预览原始出版内容；UI 样式不能污染书籍样式 |
| Amp 接入 | Go 启动 Amp CLI，解析 JSONL | 避免额外语言运行时；不能把 TUI 的 ANSI 输出当稳定协议。[S4][S5] |
| 规范校验 | 独立 EPUBCheck 进程 | 保存官方报告，转换为 Kepub 诊断格式；记录实际检查规则。[S8][S9] |
| 历史版本 | 内容快照 + 文件哈希 | MVP 不依赖用户安装 Git；后续可增加 Git 导出 |
| 本机协调 | 锁 + Unix domain socket | GUI 与 CLI 共用会话所有者，防止两个写入者互相覆盖 |

MyGo 当前仓库 `go.mod` 声明 Go 1.27.1。开发工具链应以选定 MyGo revision 的要求为准，不照搬过时的最低版本。[S10] Go、MyGo、前端依赖、Amp、EPUBCheck、Java 必须分别记录版本；不得以一个“最新版本”概括全部依赖。

### 2.2 为什么不从 Sigil 插件开始

Sigil External Editor 桥接适合继续以 Sigil 为主编辑器；Python 插件适合任务完成后回写。Kepub 的目标已经是独立阅读预览和 Agent 编辑，不再需要承担 Sigil 内部资源模型与外部工作区同步的问题。

参考 Sigil 的工作区、资源解析、缓存失效和校验思路，但不在 MVP 中依赖其运行目录、不要求用户安装 Sigil，也不复制完整编辑器实现。任何直接复用源码的决定，应先检查相应文件的许可证和项目分发策略，不在本文替项目选定许可证。

### 2.3 Amp CLI 与 Python/TypeScript SDK 的比较

| 方式 | 能力 | 成本 | 本项目决定 |
|---|---|---|---|
| CLI + JSONL | 本地执行、流式消息、线程上下文 | 自己维护进程、协议适配与测试 | MVP 默认 |
| Python SDK helper | 提供类型化流和程序接口，仍要求 Amp CLI | 增加 Python 环境及 SDK/CLI 兼容管理 | 有明确收益时再引入 |
| TypeScript SDK helper | 程序化接口；CLI 是 SDK 依赖 | 增加独立 JS 运行时或 helper 分发 | 可选替换实现 |
| WebView 直接调用 SDK | 浏览器前端不具备本机 CLI 子进程环境 | 会混淆权限与运行时边界 | 不采用 |

官方 CLI 已支持 `--execute --stream-json`，所以“想要结构化 Amp 面板就必须加 SDK”不是成立的前提。[S5] Python SDK 和 TypeScript SDK 都是可用的后端适配选项，不是 EPUB 核心，也不负责阅读器隔离。[S6][S7]

如果 CLI 协议频繁变化或 SDK 提供不可替代的任务控制能力，只替换 `AgentAdapter`；其余层继续消费 Kepub 自己定义的事件。

## 3. 总体架构

```text
                         ┌─────────────────────┐
                         │ Kepub GUI           │
                         │ MyGo + TypeScript   │
                         └─────────┬───────────┘
                                   │ 窄接口 / 事件
┌──────────────────┐     ┌─────────▼───────────┐
│ kepub CLI        ├────►│ 应用服务 / 会话协调 │
│ 无界面独立入口   │     │ 锁、任务、状态机    │
└──────────────────┘     └─────────┬───────────┘
                                   │
                         ┌─────────▼───────────┐
                         │ Go EPUB Core        │
                         │ 解析 / 快照 / 打包  │
                         └──────┬─────┬─────┬──┘
                                │     │     │
                   ┌────────────┘     │     └───────────┐
                   ▼                  ▼                 ▼
              Amp Adapter       Publication Handler   Validator
              本地 CLI          只读资源映射          EPUBCheck
                   │                  │                 │
                   ▼                  ▼                 ▼
              任务可写副本      隔离的 WKWebView      结构化诊断
```

### 3.1 建议仓库布局

以下是未来实现布局，本次方案提交不创建这些代码文件：

```text
Kepub/
├── README.md
├── docs/
│   ├── DEVELOPMENT_PLAN.md
│   ├── adr/                    # 后续记录关键决策
│   └── verification/           # M0 实测记录与兼容矩阵
├── cmd/
│   ├── kepub/                 # headless CLI
│   └── kepub-desktop/         # MyGo 入口
├── internal/
│   ├── epub/                  # container / OPF / spine / nav / URL
│   ├── archive/               # 安全解包、清单式打包
│   ├── workspace/             # 快照、版本、日志、恢复
│   ├── app/                   # 应用用例与状态机
│   ├── session/               # 锁、注册表、本机 IPC
│   ├── preview/               # 资源访问、依赖、失效事件
│   ├── agent/                 # 统一任务接口
│   │   └── ampcli/            # CLI 版本与协议适配
│   ├── validation/            # 快检 + EPUBCheck
│   ├── desktop/               # MyGo 适配，仅 GUI 引用
│   └── platform/              # macOS 路径、进程组等差异
├── frontend/
├── testdata/                   # 自制或明确授权的样本
├── scripts/                    # 构建和验证脚本，不保存令牌
├── go.mod
└── go.sum
```

先用一个 Go module，避免过早拆包。通过依赖检查保证 `cmd/kepub` 不可传递依赖 MyGo；需要独立发布核心时再拆 module。MyGo 的 `Channel`、窗口对象和 TypeScript 生成类型不得渗入 `epub`、`archive` 和 `workspace`。

### 3.2 核心概念

- `Publication`：选定的 package document 及其 manifest、spine、导航、布局信息和能力警告。
- `Workspace`：一本书的持久编辑会话，以随机 ID 标识，不能只依赖书名、ISBN 或 OPF identifier。
- `Revision`：已接受内容的不可变版本，关联完整文件清单和哈希。
- `Task`：基于一个 revision 创建的 Amp 编辑任务，拥有独立候选副本。
- `PreviewTarget`：当前显示 accepted revision 还是 task candidate，以及具体 book path 和定位信息。
- `ValidationReport`：针对确定输入哈希、工具版本和规则集生成的结果，而非一个可随意沿用的“绿色勾”。

“磁盘是事实来源”不等于“完全不维护 EPUB 模型”。结构索引必须存在，但索引失效后能够从磁盘重新构建，且不能悄悄覆盖更新的文件。

## 4. 工作区、任务与数据保护

### 4.1 出版内容与工具状态分离

建议状态根目录为 `~/Library/Application Support/Kepub/`，通过平台路径适配器获得，不散落硬编码。

```text
workspaces/<workspace-id>/
├── state.json                   # 来源路径、哈希、revision、能力等
├── original/book.epub           # 导入时保留的原始字节
├── revisions/<revision-id>/pub/ # 已接受的不可变出版内容
├── tasks/<task-id>/
│   ├── task.json
│   ├── work/
│   │   ├── AGENTS.md           # Kepub 生成的任务说明
│   │   └── pub/                # 任务候选 EPUB 根目录
│   ├── reports/
│   └── changes.json
├── preview/                     # 可重建的稳定预览代际
└── journal/                     # 提交与恢复日志
```

`pub/` 才是 EPUB 的容器根目录。Agent 的说明、日志、线程、快照、`.git`、配置以及预览脚本不能混进导出归档。

打包不能简单递归压缩 workspace。必须使用经过校验的出版文件清单：必要容器文件、manifest 资源，以及显式登记需要保留的其他输入文件。保留合法但未改动的输入资源；遇到不支持的文件类型或配置冲突应提示，不应按文件名静默删除。

原书携带的 `AGENTS.md`、`.amp`、MCP 或其他 Agent 配置不可信，不能直接成为运行配置。导入时检测并隔离；若与合法出版资源冲突且无法安全保留，降级只读并报告，不覆盖用户内容。`--settings-file` 不是关闭全部其他配置来源的保证，必须结合已选 Amp 版本验证。[S11]

### 4.2 编辑事务

每个工作区同一时刻只有一个可写任务，其他窗口和 CLI 作为读者或经会话服务发起操作。

```text
读取 accepted revision
        ↓
创建独立候选副本与写入租约
        ↓
启动 Amp → 文件变化 → 稳定预览
        ↓
Amp 退出 / 用户停止
        ↓
冻结候选内容 → 计算 diff → 快检与完整校验
        ↓
等待审核
   ┌────┴────┐
 接受       拒绝
   ↓          ↓
创建新版本  保留任务记录，恢复 accepted 预览
```

不能用普通硬链接构造可写副本，否则修改可能污染基线。第一版用复制；文件系统克隆属于后续经过测试的优化。

接受时比较 `baseRevision` 与当前版本，同时重算候选哈希。任一不一致就返回冲突，禁止盲目覆盖。接受需要停止该任务的写入进程，并针对冻结结果检查；UI 上已看过的旧报告不能替代这一步。

初版只做整任务接受或拒绝，不做逐 hunk 接受。后者容易使跨 XHTML、CSS、OPF 的修改只应用一半，留给后续事务合并能力。

### 4.3 失败和恢复

工作区状态与 revision 指针使用临时文件、同步落盘和原子替换。启动时检查未完成 journal，能够区分运行中、异常退出、等待审核和已提交状态。恢复不能仅凭旧 PID 判断进程仍存活，还要核对会话标识和锁。

取消、断网、Amp 崩溃、Java 缺失、磁盘满或预览崩溃，都不能损坏 `original/book.epub` 和已接受版本。候选结果保留为可检查状态，绝不自动当作成功。

## 5. EPUB 核心与规范边界

### 5.1 解析管线

```text
安全读取 ZIP
 → 校验容器与文件清单
 → 读取 META-INF/container.xml
 → 选择 rootfile
 → 解析 OPF / manifest / spine
 → EPUB 3 nav 或 EPUB 2 NCX
 → 建立资源引用与渲染能力信息
```

不要假定 OPF 位于 `OEBPS/content.opf`，也不要假定章节、CSS、图片固定放在某些文件夹。处理命名空间、相对 URL、percent encoding、fragment 和非 ASCII 路径，分别保存原始 URL 与解析后的 book path。

多个 rootfile 时，界面提示用户选择；CLI 要求 `--rootfile` 消除歧义，不静默选错。`linear="no"` 的内容保留且可通过链接访问，但默认不混入连续阅读顺序。OPF 中出现未知属性或元数据时，尽量保持原字节；不能用只有几个字段的 struct 重序列化整份文件而丢失信息。

UTF-8 / UTF-16 输入要经过明确解码；没有修改的文件保持原字节。对旧 XHTML 的合法实体采用受控解析策略，禁止联网下载 DTD、外部实体或启用无限实体展开。[S12]

### 5.2 “符合 EPUB 规范”的定义

本项目分开记录三件事：

1. 出版物结构和内容是否通过指定版本的 EPUBCheck。
2. Kepub 已实现的制作预览能力及未实现能力。
3. 在目标阅读器中的实测表现。

三者不能互相替代。EPUBCheck 通过不能证明所有阅读器表现相同；WKWebView 渲染正常也不能证明归档有效。完整 Reading System 一致性另有要求，本项目 MVP 不作完整一致性声明。[S8][S13]

EPUB 3 的 OPF `package@version` 仍使用 `3.0`；不能因为目标规范为 EPUB 3.3 而改成 `3.3`。[S12] API 应分开返回：

```json
{
  "packageVersion": "3.0",
  "authoringTarget": "EPUB 3.3",
  "validation": {
    "status": "not_run",
    "toolVersion": null,
    "ruleset": null
  }
}
```

内容制作基线暂定 EPUB 3.3，兼容打开 EPUB 2，不自动升级原书格式。实际校验规则必须与锁定工具对应；当前 EPUBCheck 官网示例已显示 3.4 规则，不能假定任意最新版仍只验证 3.3，也不能靠填一个 CLI 版本参数伪装规则选择。[S8][S9]

### 5.3 合法保存

`mimetype` 是 ZIP 第一项，内容精确为 `application/epub+zip`，无 BOM、无尾部换行、不压缩、不加密，ZIP header 不带 extra field。[S12] 实现时用二进制测试检查 header，尤其注意 ZIP 库可能自动加入时间戳扩展，不能只检查文件名顺序。

导出流程：

```text
冻结 accepted revision
 → 生成明确的输出文件清单
 → 必要的受控元数据更新
 → 在目标目录创建临时归档
 → 对最终归档执行 EPUBCheck
 → 记录归档 SHA-256 与报告
 → 原子生成目标文件
```

只有出版内容确实改变时才按既定策略更新 EPUB 3 的修改时间，并把该变更纳入最终校验。禁止把 workspace 的每次打开时间都写进原书。

默认禁止覆盖已有目标和输入原书。覆盖须显式请求，并确认源文件自导入后未改变；保留备份。需要保留未通过检查的研究结果时，提供明确的“导出草稿”路径，不能标成验证通过。

### 5.4 加密、字体与特殊格式

区分书籍 DRM、ZIP 加密、字体混淆和普通嵌入字体。MVP 仅保证未加密、未混淆字体的目标子集。发现 `encryption.xml` 后识别算法并报告，不将所有存在该文件的书一概判为 DRM，也不擅自移除。

不支持的混淆字体可在只读模式提示并回退字体，但不能据此确认出版物原始排版。存在签名、复杂固定版式或不支持资源时，不允许编辑流程无声破坏其语义。

## 6. 预览实现与安全隔离

### 6.1 先做制作预览，不做完整阅读引擎

首版使用章节级滚动视图；提供目录、上一章/下一章、内部锚点、返回历史、缩放与当前位置。保留原书 CSS 的“制作模式”默认不注入字号、行距或夜间样式；可选“阅读模式”的用户样式必须标注，且不写回 EPUB。

章节 DOM、CSS 和媒体可见性测试通过后，再考虑分页、双页、CFI、标注、全文搜索。不要为获得一个预览窗先实现完整阅读器生态。

### 6.2 资源访问层

GUI 自定义协议与 CLI HTTP 服务复用同一个只读 `PublicationHandler`，但不必使用相同传输：

```text
kepub-book://localhost/<workspace>/<generation>/<book-path>
http://127.0.0.1:<port>/<token>/<workspace>/<generation>/<book-path>
```

这些是 Kepub 的拟定 URL 设计，不是 MyGo 自带 EPUB 功能。MyGo 已提供 `mygo.Protocol.Handle` 和 `http.Handler` 接入点。[S2]

Handler 只服务当前被授权 publication generation 的文件；必须显式处理 MIME、大小、错误与路径。XHTML 以正确媒体类型加载，不能普遍改为 `text/html` 掩盖 XML 错误。不存在的出版物资源必须返回真实错误，不能回退到应用 `index.html`；因此不能直接沿用带 SPA fallback 的默认 FileServer。[S2]

每次请求按解码和规范化后的路径进行 containment 检查。禁止目录穿越、符号链接逃逸、直接读取工作区状态或任意本机文件。资源 URL 不暴露原书绝对路径。字体和跨源资源问题需在实际 WKWebView 上验证，不能随意开放所有 CORS。

### 6.3 MyGo 的关键风险

MyGo 文档明确：自定义协议页面属于应用自有页面，可调用 Go；框架也会向页面注入运行时。文档另外说明嵌入 frame 不得调用 Go。[S2][S3]

因此，**不能把不可信 EPUB XHTML 直接加载为有完整 IPC 权限的顶层应用页面。** `epub://` 或自定义 scheme 本身不是安全沙箱。

首选实现：可信应用壳 + 与其隔离的出版物 frame；禁用书籍脚本、表单提交、弹窗、顶层跳转和远程资源。所有 Go 服务仍需验证调用窗口、会话和操作权限，不能仅因为请求来自 MyGo 就执行文件操作或启动程序。

必须通过 M0 验证以下攻击面：frame 直接调用 runtime、导航到受信 URL、`file:` / `about:` 跳转、SVG 与嵌套文档、frame 消息伪造、开发模式 localhost 信任规则，以及 release 构建的差异。若框架无法提供可验证的隔离，改为独立、没有 MyGo 原生桥的 WKWebView 预览适配器；在解决前不得发布可打开任意外部 EPUB 的版本。

选区或点击位置回传使用应用自有、权限受限的通道。不能为了拿到选中文字而打开所有书籍脚本。MVP 最低上下文为章节路径和阅读位置；原生隔离脚本未验证前，不承诺任意 DOM 选区同步。

### 6.4 实时刷新

监听器覆盖候选 `pub/` 的目录树，处理目录新增、文件原子替换、删除、重命名、事件合并和漏报后的重扫。监听事件只是变化线索，内容哈希才是判断依据。

初始建议采用 300–500 ms 合并窗口，并检查文件稳定性。该数值是调优起点，不是既有性能保证。

| 变化 | 处理 |
|---|---|
| 当前 XHTML | 快检后刷新对应章节，尽量恢复 fragment / 阅读位置 |
| 非当前 XHTML | 更新索引，不强制跳走当前章节 |
| CSS / 字体 / 图片 | 使关联章节失效；依赖不确定时保守刷新当前视图 |
| OPF | 重新解析结构，成功后发布新 generation |
| nav / NCX | 重新解析目录并更新 UI |
| 非法中间状态 | 标注错误，保留上一份可渲染预览，不伪装为最新有效结果 |

预览使用稳定的只读 generation，避免 XHTML 来自新版本而 CSS 来自旧版本。任务可写副本仍是候选内容来源，预览 generation 只是派生缓存。刷新只消费当前 `workspaceId + taskId + generation` 的事件，丢弃旧任务晚到消息。

### 6.5 CLI 预览服务

`kepub serve` 默认只监听 loopback，使用不可预测会话令牌、Host/Origin 校验和访问日志脱敏。不开放目录列表，不绑定 `0.0.0.0`，不提供任意路径参数。GET 请求不能触发编辑；控制操作走另一个经过鉴权的本机 IPC 通道。

浏览器预览是调试和替代入口，不与 WKWebView 做像素一致承诺。令牌 URL 不写入公共日志，页面不加载远程追踪资源。

## 7. Amp 集成契约

### 7.1 程序化任务

已核实的上游命令形态为：[S4][S5]

```sh
amp --execute --stream-json
```

提示词通过 stdin 输入；Go 使用参数数组启动进程，设置工作目录为该任务的 `work/`。提示词明确出版内容位于 `pub/`，当前章节为对应 book path。避免把任意用户输入拼接成 `sh -c` 命令，也不要把整本书正文塞入 argv。

从官方流中处理 `system/init`、`assistant`、工具调用及结果、最终 `result`。最终成功必须同时满足正常进程退出、完整终止事件、任务文件快检和规定校验，而不能只匹配文字“已完成”。

读取 stdout / stderr 要并发进行，避免管道堵塞；日志与协议分离。设置可配置的消息大小上限，不能直接使用过小的逐行扫描默认缓冲；测试长行、分块 UTF-8、JSON 错误、未知字段、重复消息、意外 EOF 和错误退出。

### 7.2 多轮与线程

记录明确的 Amp thread/session ID，并绑定 workspace、task、基线 revision 与 cwd。不得使用“继续最近一个线程”来猜测，防止多本书串线。

官方支持继续线程与 JSON 流，但具体带 ID 的调用参数应由 M0 对锁定 CLI 的 `--help` 和实测结果确认。[S5] 继续线程可能保留原工作目录，不能把同一线程随意搬到新的任务目录；路径或基线改变时，新建线程或经验证后显式迁移。[S6]

第一版可以采用“一次用户提交对应一次本地执行，显式线程继续”的模型。长期 stdin JSON 多轮会话作为后续优化，不必一开始自造终端。

### 7.3 交互式 CLI

`kepub amp` 给终端用户保留原生 Amp TUI。继承终端输入输出并正确传递信号；无 TTY 时明确拒绝交互模式并提示使用 `kepub task run`。不把真实终端画面嵌入 GUI，也不把 ANSI 转义序列当成消息协议。

GUI 第一版的 Amp 面板由结构化消息构建；以后确有需要再加 PTY + 终端组件，不让终端模拟成为 MVP 前置条件。

### 7.4 权限、隐私和进程控制

`cwd`、任务副本和 `AGENTS.md` 都不是操作系统沙箱。Amp CLI 及其启动的 shell 可能拥有当前用户权限。任务隔离主要防止误覆盖原书，不能据此承诺无法访问其他目录。[S6]

MVP 面向用户明确启动的本地任务。首次启用 Amp 时说明模型处理可能发送书籍片段和提示词到服务端；“本地执行器”不等于“离线模型”。默认不开启 orb/runner、公共线程、自动上传整本书或任何自动发布功能。

优先使用已安装并授权的 Amp，记录可执行路径及版本；不复制凭证进 EPUB，不输出令牌或完整环境。禁止默认使用跳过权限检查的参数。对上游支持的权限控制做能力探测，不能虚构一个配置名就认为已获得文件系统隔离。

取消时停止整个受管进程组，而不只杀父进程；设置宽限期、强制终止和回收流程。任务超时、子进程残留、用户关闭窗口都需要确定行为。窗口关闭默认询问正在运行的任务，不默默删除其目录。

自动修复校验错误默认最多两轮，并有总时间上限；相同错误集合无进展则停止。次数和时间是 Kepub 的控制，不把无证据的价格预算说成已被 Amp 强制执行。

## 8. GUI 与 CLI 共用会话

### 8.1 单写者模型

同一本工作区的修改由一个会话所有者串行协调。所有者可以是 GUI 进程，也可以是 CLI 创建的会话进程。其他入口通过 Unix socket 附着，拿到当前状态和事件。

实现本机注册表，记录 workspace ID、socket、协议版本和会话 nonce。socket 放在受限且路径长度可控的运行目录，目录权限 0700，socket 权限 0600；验证客户端和会话，而非把路径存在等同于授权。

两个进程都尝试写时，第二个得到明确的 `WORKSPACE_BUSY` 或通过现有会话提交任务。不创建隐式竞争副本。只读 `info`、`toc` 和对独立文件的 `validate` 不依赖常驻服务。

### 8.2 操作语义

- `open`：解析文件并显示 GUI，不自动启动 Amp。
- `workspace open`：创建或显式恢复持久工作区，返回 ID，不默认弹窗。
- `preview --workspace ID`：显示当前 accepted 内容，或用户明确指定的 task candidate。
- `task run --workspace ID`：创建候选任务，不直接改 accepted revision。
- `task accept`：冻结、复核哈希、校验并产生新 revision。
- `workspace export`：从已接受版本生成独立 EPUB。

GUI 与 CLI 共享内容与任务状态，但每个视图可保留自己的阅读位置。“当前章节”必须指定 view 或使用明确的 active view，不能依赖全局字符串覆盖其他窗口。

## 9. CLI 设计

### 9.1 首批命令

以下均为计划中的 Kepub 命令，不是当前可运行安装说明。

| 命令 | 作用 | 默认是否写入 |
|---|---|---|
| `kepub doctor` | 检查工具、架构、协议与路径 | 否 |
| `kepub info BOOK --json` | metadata、OPF、资源、能力警告 | 否 |
| `kepub toc BOOK --json` | 读取目录 | 否 |
| `kepub unpack BOOK -o DIR` | 安全解包到新目录 | 是，不覆盖 |
| `kepub pack DIR -o OUT.epub` | 清单式打包并校验 | 是，不覆盖 |
| `kepub validate BOOK_OR_DIR --json` | 返回快检和 EPUBCheck 诊断 | 否；可写临时报告 |
| `kepub workspace open BOOK --json` | 创建或恢复工作区 | 是 |
| `kepub workspace list --json` | 列出可访问工作区 | 否 |
| `kepub preview --workspace ID` | 打开制作预览 | 仅视图状态 |
| `kepub amp --workspace ID` | 在候选任务内启动原生 Amp TUI | 仅任务副本 |
| `kepub task run --workspace ID --prompt-file FILE` | 程序化编辑任务 | 仅任务副本 |
| `kepub task diff TASK --json` | 查看完整变更清单 | 否 |
| `kepub task accept TASK` | 校验并接受任务 | 是 |
| `kepub task reject TASK` | 放弃候选修改，保留记录 | 状态变更 |
| `kepub workspace export ID -o OUT.epub` | 导出已接受版本 | 是，不覆盖 |
| `kepub serve --workspace ID` | loopback 浏览器预览 | 仅会话状态 |

CLI 先实现文件级命令，再接工作区和任务；不要求第一批提交同时实现整张表。`links`、`references`、`unused`、安全资源改名、自动 `fix`、MCP 和命令补全属于后续阶段。

### 9.2 机器输出

`--json` 的 stdout 只有一个 JSON 对象，日志和进度进入 stderr；流式任务使用 `--jsonl`，与 `--json` 互斥。错误也使用同一 envelope。CLI 不通过本地化错误文字让脚本判断状态。

```json
{
  "schemaVersion": 1,
  "ok": false,
  "command": "validate",
  "data": {
    "status": "failed",
    "inputSha256": "<hash>",
    "validator": {
      "name": "EPUBCheck",
      "version": "<detected-version>",
      "ruleset": "<reported-or-unknown>"
    },
    "diagnostics": [
      {
        "source": "kepub",
        "code": "LINK_TARGET_MISSING",
        "severity": "error",
        "bookPath": "EPUB/Text/chapter01.xhtml",
        "line": 18,
        "column": 7,
        "message": "内部链接指向不存在的资源"
      }
    ]
  },
  "error": {"code": "VALIDATION_FAILED", "message": "出版物未通过校验"}
}
```

位置不可得时使用 `null`，不猜测行号。EPUBCheck 原始错误码保留；Kepub 自定义错误码必须标明来源。`schemaVersion` 与软件版本分离。

建议退出码：`0` 成功，`1` 内容或校验失败，`2` 参数错误，`3` 依赖或环境不可用，`4` 锁或版本冲突，`5` Agent 执行失败，`130` 用户取消。所有命令按同一表测试。

长任务事件至少包括 `requestId`、`workspaceId`、`taskId`、`sequence`、`generation`、`type`。序号在会话内单调递增；重连先取快照，再补事件或全量刷新，不能仅靠丢失后无法恢复的广播。

### 9.3 端到端验收用法

```sh
# 以下示例需要后续实现。
kepub doctor --json
kepub info ./book.epub --json
kepub workspace open ./book.epub --json

# 用上一步返回的真实 workspace ID 替换占位符。
kepub preview --workspace WORKSPACE_ID
kepub task run --workspace WORKSPACE_ID --prompt-file ./task.txt --jsonl

# 任务结束后，用真实 task ID 审核和接受。
kepub task diff TASK_ID --json
kepub task accept TASK_ID
kepub workspace export WORKSPACE_ID -o ./book-edited.epub
kepub validate ./book-edited.epub --json
```

`task.txt` 示例：仅调整当前章节段落样式，不改正文，不新增或删除资源；完成后列出修改文件，并运行可用的 EPUB 检查。

## 10. 校验与故障处理

### 10.1 分层检查

L1 为高频快检：XML 是否良构、manifest/spine 是否能解析、必需文件是否存在、关键链接与 ID 是否有效。它不是 EPUBCheck 的替代实现。

L2 为任务结束后的变更审查：越界修改、保护文件变化、正文变化提示、资源增删、任务范围违背情况。正文变化不能仅靠简单去标签字符串保证语义不变，应展示差异并由用户审核。

L3 为冻结输入上的完整 EPUBCheck；L4 为最终输出归档上的再次检查。没有报告、报告失败、工具缺失或结果与哈希不符，状态都不能是“通过”。

EPUBCheck 官方支持 expanded 目录模式与 JSON 报告。[S9]

```sh
java -jar /absolute/path/epubcheck.jar --mode exp /path/to/pub --json /path/to/report.json
java -jar /absolute/path/epubcheck.jar /path/to/output.epub --json /path/to/report.json
```

真实调用必须按已安装版本验证。报告写到独立文件后解析，避免日志污染 stdout；保留原始报告以便定位适配错误。Java 和 EPUBCheck 的完整分发依赖都需要被发现和检查，不假定复制单个 JAR 就足够。[S14]

### 10.2 原书已有问题

导入时保存 baseline 诊断，区分原有问题与新增问题，但不能仅因为“错误没有增加”就说输出有效。可以在工作区保留无效候选、继续修复或导出明确标记的草稿；正式已验证导出必须满足相应门槛。

默认 error/fatal 阻止正式导出，warning 可导出但显示；`--strict` 将 warning 也设为门槛。不将所有无障碍缺陷当成可以由工具自动证明已修复的问题。

### 10.3 依赖缺失时

没有 Amp：仍可阅读、预览、解析、打包和校验。没有 Java/EPUBCheck：仍可阅读和执行部分快检，但完整校验返回 unavailable，禁止宣称规范合规。没有 GUI 会话：文件级 CLI 正常运行，`preview` 返回可解释的环境问题或使用 `serve`。

## 11. 安全工程要求

安全是打开外部 EPUB 的前提，不是发布后的加分项。

| 风险 | 必须实现的控制 |
|---|---|
| ZIP 路径穿越 | 拒绝绝对路径、越界路径、非法分隔符和标准化后逃逸；基于最终路径检查 |
| ZIP bomb | 对实际解压字节、文件数、单项大小和总量设置上限，边读取边计数 |
| 重复与碰撞 | 检测重复 ZIP 条目、大小写与 Unicode 标准化碰撞；不悄悄覆盖 |
| 符号链接与特殊文件 | 不解压为可逃逸链接或设备文件；导出与预览再次检查 |
| XML/DTD | 禁用网络解析与外部实体；限制深度和展开量 |
| 不可信书籍脚本 | 与应用 IPC 分离；默认禁用脚本和远程请求 |
| Prompt injection | 书籍内容始终视为数据；不从书中自动启用工具、插件或配置 |
| Agent 越权 | 显式启用、能力探测、任务副本、修改审查；不将 cwd 伪称沙箱 |
| 输出泄漏 | 导出清单排除应用状态、凭证、聊天记录、工具配置 |
| 本机 HTTP / IPC | loopback、鉴权、受限 socket、原点校验、无任意文件服务 |
| 发布依赖 | 固定版本、校验下载、许可清单、签名，不在运行时盲目执行远程安装脚本 |

初始可配置安全阈值建议：20,000 个文件、单文件 256 MiB、总展开量 2 GiB。它们是产品防护默认值，不是 EPUB 规范限制。超限要有明确错误，不在用户不知情时解除保护；阈值在样本库和内存测试后调整。

对不受信书籍启用 Agent 仍有本机权限和内容发送风险。高保证隔离需要独立执行环境或经过验证的 OS 级限制，应作为单独里程碑，不在 MVP 中虚假承诺。

## 12. 桌面交互设计

主窗口保持精简：顶部“打开、导出、校验”，侧栏显示目录/资源，主体为 Amp 对话与预览，底部显示任务和校验状态。资源树首版是定位工具，不是全功能代码编辑器。

必须清楚显示正在看哪份内容：`已接受版本`、`任务实时结果（待审核）`、`上一份有效预览（当前候选有错误）`。用户点击接受前可查看变更文件、文本差异和诊断；不能用绿色状态暗示结果已经保存。

打开书籍不自动调用模型；启动任务不默认修改正文；拒绝任务后恢复旧预览。任务运行时切换到其他书籍，不把新选择错误传给旧任务。导出采用原生保存对话框并显示独立输出路径。

读者偏好与出版 CSS 分离。缩放、面板布局、阅读位置保存到工作区状态，不以注入 CSS 的方式永久修改原书。

## 13. macOS 构建与分发

首发产物为 `Kepub.app` 和独立 `kepub` arm64 二进制。CLI 命令安装到用户选择的 PATH 位置，或提供 `~/.local/bin` 的显式安装方案；不要擅自写入 `/opt/homebrew/bin`，也不要声称已有 Homebrew formula。

GUI 从 Finder 启动时不能依赖用户 shell 的 PATH。通过设置与 `doctor` 发现 Amp、Java、EPUBCheck 的绝对路径，检测版本与架构；不把 Ghostty 当运行依赖。原生终端桥接是可选功能，程序化任务不需要外部终端。

开发版允许本机构建与测试。对外分发前验证 app bundle、代码签名、公证、文件关联和隔离属性处理；helper 与运行时也要纳入检查。MyGo 的分发机制需要用本项目实际产物验证，不能由其“单 Go 程序”宣传推导整套 Kepub 只有几 MB。

MVP 优先使用用户已安装的 Amp 与 Java，降低打包与许可复杂度。后续是否捆绑 Java runtime、是否分发 Amp helper，分别经过许可、更新和 arm64 测试。普通用户运行已打包应用不应需要 Go 或 Bun。

## 14. 实施里程碑与验收门槛

各阶段以下一阶段依赖已通过为前提，不给未经验证的固定工期承诺。当前所有条目均待实施。

### M0：风险验证与版本冻结

交付 `docs/verification/` 中的实测记录、最小样本和依赖矩阵。

- [ ] MyGo 在 Apple Silicon release 构建中正常启动；记录最低 macOS 和实际 Go/MyGo revision。
- [ ] 正确加载 XHTML、外链 CSS、图片、字体、竖排/ruby 样本，验证资源 MIME 和缓存失效。
- [ ] 证明书籍 frame 无法调用 Go、不能逃逸到可信顶层、不能读取任意本机资源。
- [ ] Amp JSONL 本地执行、显式线程继续、失败与取消可重复复现；记录权限和配置加载行为。
- [ ] EPUBCheck 和 Java 的版本、校验规则、JSON 格式和退出码得到实测记录。
- [ ] 得到最小端到端证据：Amp 改一份测试 XHTML，预览更新，导出再校验。

**退出条件：** 隔离与协议全部通过；否则修补/替换适配层，不开展依赖这些假设的大量 UI 工作。不以开发模式能跑替代 release 验证。

### M1：EPUB 核心与文件级 CLI

- [ ] 安全导入、container/rootfile、OPF、spine、nav/NCX。
- [ ] `doctor`、`info`、`toc`、`unpack`、`pack`、`validate`。
- [ ] 正确 ZIP header、出版文件清单、JSON schema 和退出码。
- [ ] 最小 fixture 库及 round-trip 测试。

**退出条件：** 不启动 GUI 即可完成“打开 → 检查 → 解包 → 打包 → 校验”；未修改资源字节不变，原书不被覆盖，危险样本被拒绝。

### M2：工作区、快照和共享会话

- [ ] workspace/revision/task 状态机、日志恢复和单写者锁。
- [ ] task candidate、冻结、diff、接受/拒绝与导出。
- [ ] GUI/CLI 会话发现与附着的 IPC 原型。
- [ ] `workspace open/list/export`、`task diff/accept/reject`。

**退出条件：** 两个进程不能同时破坏同一工作区；模拟崩溃和磁盘失败后保留原书与 accepted revision；历史内容可恢复。

### M3：MyGo 制作预览与 CLI serve

- [ ] 三栏基础界面、目录、章节与链接导航。
- [ ] 隔离 PublicationHandler、稳定 generation、CSS 及媒体失效刷新。
- [ ] 阅读位置恢复、原书样式与阅读模式分离。
- [ ] `preview`、`serve` 及共享工作区预览。

**退出条件：** 外部修改候选文件后无需重开书；非法中间状态可解释；安全攻击样本不触达 Go 控制接口。

### M4：Amp 编辑闭环

- [ ] `AgentAdapter`、CLI JSONL、结构化面板、取消与错误展示。
- [ ] `kepub amp` 原生 TUI 与 `task run` 非交互入口。
- [ ] 当前章节上下文、显式 thread 绑定、任务范围和隐私提示。
- [ ] 有限修复循环、任务冻结、diff 审核、最终归档验证。

**退出条件：** GUI 与 CLI 都完成完整工作流；取消、断网、非法输出不会写坏 accepted 内容；没有未退出的受管写进程。

### M5：可分发的 macOS 版本

- [ ] arm64 产物、Finder 打开、`.epub` 文件关联与 CLI 安装说明。
- [ ] 干净机器的依赖发现、签名、公证和升级迁移测试。
- [ ] 性能、内存、残留进程与长期会话测试。
- [ ] 发布说明列明支持子集、依赖和已知限制。

**退出条件：** 非开发机器能够按文档使用；没有未知的 P0 数据损坏/越权问题；发布版通过完整验收矩阵。

### 后续阶段

先补经过事务验证的资源新增/删除/改名、OPF/spine 修改与引用修复，再考虑全文搜索、分页、固定版式、字体混淆、注释、MCP 和其他 Agent。跨平台最后进入独立适配与渲染测试阶段，不把 MyGo 可交叉编译等同于已完成支持。

## 15. 测试计划

### 15.1 样本矩阵

| 类别 | 至少覆盖 |
|---|---|
| 基本出版物 | EPUB 2 + NCX、EPUB 3 + nav、nav 不在 spine、非线性章节 |
| 路径 | 根目录/深层 OPF、多 rootfile、中文、空格、percent encoding、同名资源、Unicode 碰撞 |
| 排版 | 中英文段落、嵌入字体、SVG、表格、ruby、竖排、RTL、长章节 |
| 链接 | 跨章节锚点、脚注往返、丢失目标、重复 ID、CSS `url()` 与 `@import` |
| 不支持特性 | 固定版式、混合布局、脚本、音视频、加密/混淆、签名 |
| 异常输入 | 损坏 ZIP、重复 entry、路径穿越、解压超限、非法 XML、错误 namespace |
| 生命周期 | 原子替换文件、连续写入、取消、崩溃、磁盘满、多进程冲突、源文件被外部修改 |
| 权限 | 不可信 frame、导航逃逸、外链资源、伪造消息、书内 Agent 配置 |

样本优先自制；使用公开测试书时保存来源和许可，不把用户私有书籍上传至公共仓库。字体测试使用可再分发的授权素材或测试机本地资源，不提交未经授权的字体。

### 15.2 分层自动化

Go 单元测试覆盖路径、归档、XML、URL、事务和状态机；fuzz 测试覆盖不可信解析入口。fake Amp 进程模拟协议、取消和超长消息，普通 CI 不调用真实付费模型。EPUBCheck 集成测试锁版本并保存预期错误码。

前端测试覆盖状态与事件顺序；浏览器测试不能替代真实 MyGo/WKWebView 测试。release 构建在 Apple Silicon 实机进行安全、菜单、文件关联、渲染和恢复验收。

仅文档变更不运行模型或发布程序。未来 CI 区分文档检查、核心测试和需要凭证的签名发布；不得把公共 PR 置于含发布密钥或 Amp 凭证的执行环境。

### 15.3 性能目标（待 M0 基线验证）

以明确记录配置的 Apple Silicon 测试机和固定样本为基准，首轮目标为：普通 20 MiB、约 200 章样本打开到首章显示不超过 3 秒；单章稳定修改到预览完成的 P95 不超过 1 秒；常见本机取消在 3 秒内进入确定终态。

这些是目标而非现有效果，不能与模型响应延迟混算。内存和空闲 CPU 先测量后冻结阈值，分别报告 GUI、WebKit、Amp 和 Java 进程；不引用 Hello World 的内存作为产品性能。

## 16. 首批开发任务清单

| 编号 | 任务 | 完成证据 |
|---|---|---|
| K-001 | 冻结依赖与架构测试矩阵 | 版本、平台、行为验证记录 |
| K-002 | MyGo 隔离预览 spike | release 安全样本全部通过 |
| K-003 | Amp 协议 spike | fake/真实本地执行、继续、取消日志 |
| K-004 | 安全归档与路径模型 | 单测、fuzz、ZIP header 断言 |
| K-005 | Publication parser | EPUB2/3、嵌套 OPF、导航样本 |
| K-006 | EPUBCheck adapter | 缺失依赖、失败、JSON、规则记录 |
| K-007 | 文件级 CLI | 稳定 JSON、退出码、无 GUI 依赖 |
| K-008 | Workspace 与 revision | 冲突、崩溃恢复、数据保全测试 |
| K-009 | Session IPC | 多入口同一工作区、单写者验证 |
| K-010 | Watcher 与 generation | 原子替换、CSS 更新、中间错误测试 |
| K-011 | GUI 阅读与审核 | 完整交互录像及状态测试 |
| K-012 | Agent 任务闭环 | diff → 校验 → 接受 → 导出 |
| K-013 | 打包分发 | 干净 arm64 Mac 安装验收 |

这些是待实施的任务标识，不代表已经创建 GitHub Issues。建议每个任务使用小范围分支和可核验验收记录，不让 Agent 一次性生成整个应用后再补测试。

## 17. 风险、决策门与范围控制

| 风险 | 处理决策 |
|---|---|
| MyGo API 或隔离不足 | M0 阻断；限制在 desktop 适配层，必要时替换桌面预览实现 |
| Amp 协议变动 | 冻结版本、契约测试、能力探测；不支持时拒绝自动任务而不是猜参数 |
| 书籍资源模型复杂 | 有边界地支持、明确只读降级；不重写无法保真的元数据 |
| 候选预览与 accepted 混淆 | 明确目标标识、generation 和状态条；导出只取冻结内容 |
| GUI/CLI 并发覆盖 | 单写者租约、版本检查、会话协议 |
| Java 部署带来体积 | MVP 发现已有依赖，后续独立决定 runtime 分发 |
| 预览被误当完整规范实现 | 发布说明列功能矩阵，结果展示绑定工具版本和规则 |
| 功能膨胀 | 首先完成一个“打开—编辑—预览—审核—验证—导出”闭环 |

项目最优先的是“不丢书、不误写、不越权、可验证”。文件树美化、完整代码编辑器、内嵌终端、云同步、阅读统计和跨平台发行都不能挤掉这些门槛。

## 18. 来源与核验记录

核验日期为 2026-10-03。MyGo 官网本次直接访问未成功，使用官方 GitHub 仓库文档；检索时 `main` 指向 `933b79b17d9b4f7e0cf53d24a10bf5ff21351920`。这些是方案依据，不代表已在目标 Mac 上编译或运行。

- **[S1] MyGo 官方 README**：系统 WebView、项目阶段、平台与许可说明。https://github.com/egoist/mygo/blob/main/README.md
- **[S2] MyGo 前端与协议文档**：`Protocol.Handle`、文件服务、custom protocol 信任与 runtime。https://github.com/egoist/mygo/blob/main/docs/frontend.md
- **[S3] MyGo bindings 文档**：类型化接口、channel、上下文和调用来源限制。https://github.com/egoist/mygo/blob/main/docs/bindings.md
- **[S4] Amp Execute Mode**：本地执行、stdin、非交互工作方式。https://ampcode.com/docs/cli/execute-mode
- **[S5] Amp Streaming JSON**：JSONL、消息格式、线程继续与流式输入。https://ampcode.com/docs/cli/streaming-json
- **[S6] Amp Python SDK**：CLI 依赖、cwd、线程和执行器选项。https://ampcode.com/docs/sdk/python
- **[S7] Amp TypeScript SDK**：CLI 依赖、execute 和程序化接口。https://ampcode.com/docs/sdk/typescript
- **[S8] EPUBCheck 官方入口**：出版物检查范围与分发形式。https://www.w3.org/publishing/epubcheck/
- **[S9] EPUBCheck CLI**：expanded 模式、JSON、退出行为、单文件与整书检查边界。https://www.w3.org/publishing/epubcheck/docs/cli/
- **[S10] MyGo go.mod**：本次读取声明 `go 1.27.1`。https://github.com/egoist/mygo/blob/main/go.mod
- **[S11] Amp Configuration**：配置来源、工作区优先级与 settings file。https://ampcode.com/docs/cli/settings
- **[S12] W3C EPUB 3.3**：容器、URL、XML、package version、ZIP 与出版物要求。https://www.w3.org/TR/epub-33/
- **[S13] W3C EPUB Reading Systems 3.3**：阅读系统一致性边界。https://www.w3.org/TR/epub-rs-33/
- **[S14] EPUBCheck 安装说明**：Java 与完整分发目录要求。https://www.w3.org/publishing/epubcheck/docs/installation/

M0 应进一步保存锁定依赖的 commit/tag、校验和、CLI help、诊断样例和测试机器信息。官网、安装说明、发布页或实际工具输出发生不一致时，以完成验证的工具版本及其可追溯源码为实施基线，保留差异记录，不擅自补全。

---

**实施原则：先证明安全预览和可控 Agent，再实现无界面 EPUB 核心，接入共享工作区和 GUI，最后完成可分发的 Apple Silicon 版本。**
