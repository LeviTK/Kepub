# Kepub CLI 与操作契约

> 设计版本：0.2 · 日期：2026-10-03 · 状态：待实现。
>
> 本文定义 Kepub 自己的命令和协议，不是 Calibre 或 Amp 的使用手册，也不表示命令已经能运行。架构见 [开发方案](DEVELOPMENT_PLAN.md)，设计依据见 [Calibre 研究](research/CALIBRE_CLI_REVIEW.md)。

## 1. 设计目标

CLI 是与 GUI 平级的产品入口，也是 Agent 的确定性工具层。不要求用户安装桌面端来查询或打包 EPUB；不要求运行模型来改一个已明确指定的字段。

一个子命令对应一个应用用例。GUI 不拼 shell 字符串复用 CLI；GUI/CLI 都调用 Go 服务。Agent 使用 CLI 时仍面对相同任务、校验与审核语义。

## 2. 命令分组与实施次序

| 命令形态 | 语义 | 阶段 |
|---|---|---|
| `kepub doctor --json` | 环境/架构/外部工具发现，不调用模型、不修改书籍 | M1 |
| `kepub capabilities --json` | 版本化能力和可用性；区别 planned/available/unavailable/unsupported | M1 |
| `kepub info BOOK --json` | 出版信息摘要，含规范/渲染能力警告 | M1 |
| `kepub toc BOOK --json` | 读取导航，不改变 spine | M1 |
| `kepub inspect BOOK --section manifest --json` | 指定范围的只读结构查询 | M1 |
| `kepub unpack BOOK --output DIR` | 解到新目录，安全检查，不覆盖 | M1 |
| `kepub pack DIR --output OUT.epub` | 清单式归档及正式检查，不是convert | M1 |
| `kepub validate BOOK_OR_DIR --json` | 分层诊断及覆盖报告 | M1 |
| `kepub workspace open BOOK --json` | 创建/显式恢复工作区，返回ID，不弹窗 | M2 |
| `kepub workspace list --json` | 列出可访问工作区与状态 | M2 |
| `kepub plan --workspace ID --operations FILE --output PLAN.json` | 从accepted基线生成操作计划，不写出版内容 | M2 |
| `kepub apply PLAN.json --json` | 核对计划，创建并处理候选任务；不接受/不导出 | M2 |
| `kepub task diff TASK --json` | 真实文件增删改、路径变化和内容差异 | M2 |
| `kepub task accept TASK` | 审核后的显式接受，冻结检查并创建新revision | M2 |
| `kepub task reject TASK` | 拒绝候选并记录状态，不删除原书 | M2 |
| `kepub workspace export ID --output OUT.epub` | 仅从accepted导出、检查最终归档 | M2 |
| `kepub preview --workspace ID --at 'EPUB/Text/ch01.xhtml#note1'` | 启动GUI预览并定位；不启动Amp | M3 |
| `kepub serve --workspace ID` | 受限本机只读预览服务 | M3 |
| `kepub amp --workspace ID` | 建候选后启动原生Amp TUI，要求TTY | M4 |
| `kepub task run --workspace ID --prompt-file FILE --jsonl` | 程序化Amp任务，结束后仍需审核 | M4 |

`inspect --section` 首批枚举 `metadata`、`manifest`、`spine`、`navigation`、`references`、`capabilities`。引用查询可加 `--resource BOOK_PATH` 和 `--direction incoming|outgoing`；查询不完整时在数据中返回 coverage，不能把空列表当全书无引用。

`info` 和 `toc` 是同一读用例的便捷入口。暂不增加一串同义顶层命令；改名、metadata和未来polish由操作注册表表达。`convert`、任意Calibre透传、MCP、书库、邮件、批量删除不属于首批命令。

上述命令按阶段逐步实现，`capabilities` 不能把表中所有设计都提前报告为 available。

## 3. 目标选择与全局约定

### 3.1 明确目标

只读命令接受一个 BOOK/目录，或者 `--workspace ID` 绑定accepted revision，或 `--task ID` 读取稳定候选代际；三者互斥。`--revision REV` 只能与对应工作区联合使用。

对目录、文件、工作区的格式检测与能力限制来自相同核心，不根据扩展名自动启用转换。多rootfile要求 `--rootfile BOOK_PATH`；出现歧义返回参数错误，不能选第一个后继续写。

任务读取包含实际generation/输入hash；当前候选不稳定时返回上一冻结视图的标识和stale状态，或按请求拒绝读取，不伪装最新。

### 3.2 输出和交互

- `--json`：stdout 恰好一个 UTF-8 JSON 对象；包括失败情况。日志/进度去stderr。
- `--jsonl`：仅长任务支持，逐行完整事件，最后一个terminal事件；与 `--json` 互斥。
- `--output/-o`：仅产物命令使用，值为显式输出位置；Kepub 自己保持一致，不能直接映射到 Calibre `-o`。
- `--strict`：正式检查门槛提高至warning；不意味着全世界阅读器一致。
- `--timeout`：正数秒，作用于当前请求及受管子进程；超时回收进程组并保留失败记录。
- `--`：结束选项，用于以短横线开头的路径。调用外部工具时先转为合法绝对路径并用参数数组。
- `--no-input`：禁止交互，缺少目标、授权或必要选择时返回明确错误。JSON模式默认不做终端问答。

不提供可以跳过所有安全、版本和校验门槛的全局 `--force`。草稿导出是显式模式 `--draft`，输出和报告均标记未获正式验证；不能因缺Java而自动切草稿。

serve默认在stderr显示本机访问说明，结构化使用场景用受控事件接口；不要把会话令牌写到公开诊断包。原生 `amp` 不支持混合TUI和JSON stdout。

## 4. OperationRegistry

### 4.1 注册条目

每项操作至少包含以下字段：

| 字段 | 含义 |
|---|---|
| operationId / operationVersion | 操作语义及独立版本 |
| implementationStatus | planned / available / unavailable / unsupported |
| reason | 不可用原因，不猜测已安装工具能力 |
| inputSchema / outputSchema | 严格参数与结果约束 |
| mutatesPublication | 是否写出版内容 |
| risk | read_only / bounded_edit / structural / lossy / external |
| requiresModel / requiresNetwork / requiresGUI | 执行依赖，默认不擅自开启 |
| supportedFeatures | EPUB版本、语法、资源与布局子集 |
| preconditions | 输入版本、哈希、覆盖、权限等 |
| postChecks | 执行后必须进行的检查 |
| idempotency | 重试与无变化行为 |

阶段状态与运行时可用性要有明确表达，例如实现尚不存在是planned，代码存在但工具缺失是unavailable。不能将文档中的 planned 状态生成可执行工具调用。

### 4.2 首批操作

| operationId | 输入边界 | 行为 |
|---|---|---|
| `publication.inspect` | 已选择的publication和section | 只读结构与coverage |
| `references.inspect` | BookPath与方向 | 返回引用边及未知/阻断范围 |
| `metadata.set` | 明确且唯一命中的metadata元素、预期旧值、新值 | 保留无关字段/namespace/refinement；不能唯一选择则拒绝 |
| `resource.rename` | 一个现有非OPF资源、未占用目标BookPath | 更新已证明覆盖的入站/出站引用，不修改内容语义 |

初版 `resource.rename` 不支持循环交换、批量图改名、OPF改名、多rootfile重构、未知引用。只变大小写的改名必须有专门的平台安全流程和fixture，通过前显示unsupported。

`metadata.set` 首期不改变package unique identifier、语言体系、复杂关联或升级EPUB版本。无变化返回 `changed:false`，不借机重新格式化整份OPF或更新时间。

未来操作如 `css.prune`、`image.optimize`、`font.subset`、`toc.rebuild`、`publication.convert` 分别设计参数与风险，不通过一个任意字符串的 `run_command` 逃离注册表。

## 5. 声明式操作请求

示例是设计契约，不能直接执行。请求文件只描述操作，不含workspace凭证或任意可执行代码：

```json
{
  "schemaVersion": 1,
  "operations": [
    {
      "operationId": "resource.rename",
      "operationVersion": 1,
      "params": {
        "from": "EPUB/Text/ch01.xhtml",
        "to": "EPUB/Chapters/ch01.xhtml"
      }
    }
  ]
}
```

未知字段、未知操作版本、不合法路径、空操作集、超出步骤上限都必须失败。批量步骤不是任意循环/条件脚本；每一步只能调用注册的确定性操作。未来带外部资源的输入通过应用登记为输入artifact并绑定hash，不让书籍内容提供任意本机读取路径。

## 6. 计划契约

### 6.1 plan不改书

plan从指定accepted revision读取，完成参数、能力和引用覆盖检查，计算修改影响与风险，写出计划文件。文件报告本身是允许的副作用，出版内容、accepted pointer与原始EPUB不得改变。

计划示例中的HASH/ID是说明用占位值；真实实现必须输出合法ID和完整摘要：

```json
{
  "schemaVersion": 1,
  "planId": "PLAN_ID",
  "workspaceId": "WORKSPACE_ID",
  "baseRevision": "REVISION_ID",
  "inputTreeSha256": "SOURCE_TREE_HASH",
  "operationSetSha256": "OPERATIONS_HASH",
  "policySha256": "POLICY_HASH",
  "operations": [
    {
      "operationId": "resource.rename",
      "operationVersion": 1,
      "params": {
        "from": "EPUB/Text/ch01.xhtml",
        "to": "EPUB/Chapters/ch01.xhtml"
      }
    }
  ],
  "impact": {
    "exactWriteSet": false,
    "changedCandidates": [
      "EPUB/package.opf",
      "EPUB/nav.xhtml",
      "EPUB/Text/ch01.xhtml",
      "EPUB/Chapters/ch01.xhtml"
    ],
    "referenceCoverage": "partial"
  },
  "applicable": false,
  "blockers": [
    {
      "code": "REFERENCE_COVERAGE_INCOMPLETE",
      "message": "尚未证明所有受影响引用均已覆盖"
    }
  ]
}
```

示例故意为被阻断计划，说明“生成了文件”不等于允许执行。partial coverage下的潜在文件列表不能伪装成完整write set；可执行结构计划必须完成对应支持范围的必要覆盖检查。

### 6.2 摘要规则

`inputTreeSha256`来自受控publication清单中的精确BookPath、文件类型、大小与内容hash，按明确排序和编码方式组合；不能取目录mtime。operation/policy摘要用已冻结的规范化JSON算法，字段顺序不应改变语义hash。计划自身hash不包含自指hash字段。

计划文件内容不可信：apply重新验证schema、操作版本、路径、预期输入与授权，从实际输入重算影响；不能相信用户或Agent写入的 `applicable:true` 或任意writeSet。

### 6.3 apply只进入候选

执行顺序：

```text
取得工作区写租约
 → 校验baseRevision/输入树/配置/实现版本
 → 重算计划与影响
 → 创建task及独立候选
 → 操作前checkpoint
 → 执行操作并验证真实writeSet
 → 快检/完整检查
 → 冻结结果供审核
```

计划读取后输入变化返回STALE_PLAN；越界改动、未知语法、阶段失败均不接受。必须验证创建/删除/改名与内容写入的完整文件集合，不仅检查被写文件的后缀。

### 6.4 与Amp候选的衔接

v0.2首期的plan以accepted revision为基线，不直接对运行中Amp候选作结构变更。需要接着处理Amp结果时：先结束任务、审核并接受内容结果，再对新revision生成结构计划；不能把旧revision计划偷偷套到新候选上。

同一候选内暂停Agent、获取写入租约、应用操作再继续对话，属于后续受控工具交接协议。没有该协议前，不能仅凭“模型说已暂停”判断没有子进程在写。

## 7. 机器结果和退出码

### 7.1 统一envelope

```json
{
  "schemaVersion": 1,
  "ok": false,
  "command": "validate",
  "requestId": "REQUEST_ID",
  "data": {
    "status": "incomplete",
    "inputTreeSha256": "SOURCE_TREE_HASH",
    "checks": [
      {"id": "parse.xml", "status": "failed", "source": "kepub"},
      {"id": "references", "status": "blocked", "blockedBy": ["parse.xml"], "source": "kepub"},
      {"id": "epubcheck", "status": "unavailable", "source": "epubcheck"}
    ],
    "diagnostics": [
      {
        "source": "kepub",
        "code": "XML_NOT_WELL_FORMED",
        "severity": "error",
        "bookPath": "EPUB/Text/ch01.xhtml",
        "line": null,
        "column": null,
        "message": "XML解析失败"
      }
    ]
  },
  "error": {"code": "DEPENDENCY_UNAVAILABLE", "message": "必要的完整检查器不可用"}
}
```

`ok`表示请求是否达到其契约，不直接等于出版物合规。`inspect`成功返回问题列表仍可ok=true；`validate`仅在请求的必需检查完整且门槛通过时ok=true。`task run/apply`执行成功不代表accepted，输出必须带 `reviewRequired` 和校验状态。

检查报告可同时有内容错误与依赖缺失，全部保留。顶层error/退出码选择优先级：参数/授权错误 → 锁或版本冲突 → 必需依赖缺失 → 执行故障 → 内容/策略失败。示例因此返回依赖不可用，而没有丢弃XML错误。

位置不可得时null；上游原始错误码与Kepub码分开；checker版本、规则、配置、输入hash与原始报告路径保留。输出路径若含私有信息，在可分享报告中脱敏。

### 7.2 退出码

| 码 | 含义 |
|---|---|
| 0 | 请求按契约完成；不隐含书籍已接受/已验证 |
| 1 | 内容、校验或计划执行政策不满足 |
| 2 | 参数/schema/操作版本/授权输入不正确 |
| 3 | 必需依赖或能力不可用 |
| 4 | 工作区忙、版本冲突或过期计划 |
| 5 | Agent/外部执行器失败或超时 |
| 6 | 文件I/O或内部执行故障 |
| 130 | 用户取消 |

`--json`即使失败也不打印Python/Go堆栈到stdout；内部信息放stderr或脱敏报告。对外部工具不透传其退出码，而保留 `backendExitCode` 后映射到上述契约。

### 7.3 事件

JSONL事件至少含 `schemaVersion`、`requestId`、`workspaceId`、可空taskId、sequence、可空generation、type、data。sequence在请求内严格递增；重新连接通过状态快照和恢复游标，不能假定事件永不丢失。

task进程正常退出、协议terminal事件与文件冻结检查分别验证；有一项缺失就不能报告completed。进度文本、用户生成内容和终端控制字符不参与状态判定。

## 8. 接受、导出和权限

`task accept`在用户审核后显式执行，复核冻结hash/baseRevision并停止受管写进程。默认拒绝新error/fatal；原书已有错误需针对当前检查报告显式确认，结果仍标未验证有效。检查缺失/结构不可安全解析不能靠确认绕过。

`workspace export`仅针对accepted；默认最终归档完整检查通过才生成正式产物。`--draft`是用户明确请求的例外，报告不能标pass。原书保护、已有输出冲突和校验政策不能被一个通用 `--yes` 取消。

GUI对Agent默认只暴露查询、提案和候选任务入口，不自动授予accept/export。原生Amp若可运行当前用户任意shell，仍可能调用用户权限下的CLI；这种限制是应用流程，不是OS安全隔离。高保证授权隔离必须另做执行环境，不能靠命令名字或AGENTS文件宣称完成。

## 9. 外部适配契约

不实现 `kepub calibre <任意参数>`。适配器从明确operation参数生成argv，并记录版本、支持状态和副作用。公开CLI优先；内部API helper仅在独立、受控、固定版本的适配层出现。

例如未来使用ebook-polish时，Kepub的 `--output` 必须映射成Calibre第二位置参数，而非 `-o`。可选内部CSS检查的Qt/WebEngine依赖单独探测，不因一个 `calibre-debug --version` 成功就宣称可用。

外部命令输入/输出都在task artifacts中，不能给原书作为输出目标；实际输出重新安全导入，检查条目、hash、未知资源和格式变化。模型和外部工具都不能跳过同一候选审核。

## 10. 文档生成与验收

实现后由注册表生成操作schema、help、capabilities结构和示例骨架；本文保留行为语义，生成物变更需契约测试。支持的CLI语法不能仅存在于README。

必要用例包括：

- 无GUI、无Amp、无Calibre的info/unpack核心流程；缺EPUBCheck时明确unavailable。
- JSON成功/失败各只有一个对象，JSONL断流不能成功；带空格/中文/短横线路径。
- partial引用计划不能apply；修改plan的applicable/writeSet不能绕过重算。
- 基线变化、操作版本变化、并发写、运行中Agent均拒绝危险apply。
- metadata no-op不改字节；改名更新入站/出站引用，不支持语法明确阻断。
- 接受并非导出；应用计划并非接受；工具退出0并非校验通过。
- 进程取消后无遗留写入者；阶段失败可回滚，输入原书未被覆盖。

本次只制定以上契约，没有把示例注册成已实现命令，也没有执行真实Calibre/Amp任务。
