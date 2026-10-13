# Kepub

**独立运行的 EPUB CLI：查询、受控编辑、审核与导出。**

以 Go 实现，不依赖桌面端或模型服务。原书、已接受版本与待审候选分离；修改经过计划、执行、差异审阅和正式校验。项目名不表示默认输出 Kobo KEPUB，默认仍是普通 EPUB。

[快速开始](#快速开始) · [编辑流程](#编辑流程) · [能力与边界](#能力与边界) · [开发与验证](#开发与验证) · [路线与待办](#路线与待办) · [文档索引](#文档索引)

## 当前状态

截至 **2026-10-13**，S0、T1a／T1b 与 T2 的五个有限增量已验收并集成主线；已有修复栈及后续审查修复通过 [PR #27](https://github.com/LeviTK/Kepub/pull/27) 合并到 `main`。

- **可用：** EPUB2/3 查询、安全解包、工作区、受控元数据／XHTML 编辑、多资源事务、搜索替换、修复提案、正式校验与导出。
- **尚未完成：** 完整 T2、EPUB2 → EPUB3 转换、完整新建/章节/图片维护、完整 CSS／资源维护、桌面预览及生产 Agent 集成。
- **交付形态：** 源码构建，暂无版本化安装包；Linux 已验证，Apple Silicon Mac 实机验收仍待完成。
- **本轮范围：** 本地新增原生 EPUB3 最小新建入口，已执行真实 CLI 新建、正文/标题编辑、拒绝/接受、正式导出与重开；独立固定树审查及全仓验证状态见[首增量收据](docs/verification/NATIVE_EPUB3_MINIMUM.md)。未推送、未发行，不代表完整 M1/F3。文档 v0.11 是方案版本，不是软件发行版本。

[合并提交 e102b21](https://github.com/LeviTK/Kepub/commit/e102b2198ffd10df1cb05da3d70f68c47dc694ce) 的 [Linux CI](https://github.com/LeviTK/Kepub/actions/runs/38364503620) 已通过：普通／race 各 1858 个测试及子测试 PASS、1 项私有原书 SKIP、0 FAIL，另通过 vet 和普通／race CLI 构建。该结果不代表 Mac、断电、真实渲染或完整 EPUB 3.3 支持；历史失败与有限批准保持原范围。

## 快速开始

### 从源码构建

需要 **Go 1.27.1**。已构建的核心 CLI 不要求 Go、Node、MyGo、Amp 或 Calibre 运行环境。

```sh
git clone https://github.com/LeviTK/Kepub.git
cd Kepub
go build -o kepub ./cmd/kepub
./kepub --help
./kepub version --json
./kepub capabilities --json
./kepub doctor --json
```

也可执行 `go install ./cmd/kepub`，将 `GOBIN`（默认 `$(go env GOPATH)/bin`）加入 PATH 后以 `kepub` 调用。可执行能力以 `--help` 与 `capabilities` 为准；`planned` 不代表已经实现。

### 查询与解包

以下书名和资源路径均为示例，须替换为实际输入。

```sh
./kepub info 'book.epub' --json
./kepub toc 'book.epub' --json
./kepub inspect 'book.epub' --section manifest --json
./kepub inspect 'book.epub' --section references \
  --resource 'EPUB/chapter.xhtml' --direction incoming --json
./kepub unpack 'book.epub' --output 'new directory' --json
```

`inspect` 支持 metadata、manifest、spine、navigation、references、capabilities。多 rootfile 输入必须显式传 `--rootfile`，值为精确书内路径；以 `-` 开头的书名使用 `--` 分隔，例如 `./kepub info --json -- '-book.epub'`。输出父目录须已存在，目标不能已存在。

### 配置正式检查器

正式 `validate`、`pack`、`task accept` 和非草稿 export 需要 **Java 17+ 与 EPUBCheck 5.3.0**。下载并解压 [官方完整发行包](https://github.com/w3c/epubcheck/releases/tag/v5.3.0)，保留主 JAR 与 `lib/`，不要只复制主 JAR。

```sh
export KEPUB_EPUBCHECK_JAR='/absolute/path/epubcheck-5.3.0/epubcheck.jar'
./kepub doctor --json
./kepub validate 'book.epub' --json
./kepub validate 'publication directory' --strict --timeout 30 --json
./kepub pack 'publication directory' --output 'new-book.epub' --json
```

`doctor` 成功收集诊断不等于检查器可用，须检查 `formalValidationAvailable`。命令不自动安装依赖，缺检查器不会隐式降级；`--strict` 使 warning 也阻止通过。显式 `pack --draft` 或 `workspace export --draft` 允许草稿，产物标记 `verified:false`；**没有草稿接受**。

### 原生新建 EPUB3（本地首增量）

使用公开的[新建定义示例](docs/examples/newbook-v1.json)，修改标题、作者、语言、标识符、UTC 修改时间及纯文本章节。

```sh
./kepub workspace create --definition docs/examples/newbook-v1.json --output 'new-work' --json
./kepub content --workspace 'new-work' --resource 'EPUB/chapter-002.xhtml' --json
./kepub workspace export 'new-work' --output 'new-book.epub' --json
```

新建必须先通过正式检查，不能用 `--draft` 跳过。新书记录生成定义、模板版本与完整来源，不伪造导入原书；已有工作区不覆盖。支持 1–64 章，重复章名允许，输入顺序就是正文/目录顺序，固定横排 CSS 不嵌入字体。后续编辑复用下面的 plan/apply/diff/accept/reject 流程。此首增量不含封面/插图、两层目录或创建后的资源新增/章节重排；详见 [冻结契约](docs/CLI_CONTRACT.md#201-原生-epub3-最小新建-v12026-10-13-冻结)。

## 编辑流程

**`apply` 只生成待审候选；`accept` 才推进已接受版本；export 只导出已接受版本。**

```text
原书 → 工作区 accepted → plan → apply → 候选 diff
                                         ├─ accept → 新 accepted → export
                                         └─ reject → 保留记录，随后重新计划
```

### 1. 建立工作区并查询

```sh
./kepub workspace open 'book.epub' --output 'work' --json
./kepub content --workspace 'work' --resource 'EPUB/chapter.xhtml' --json
./kepub search --workspace 'work' --query '目标短语' --limit 20 --json
```

`content` 与 `search` 只读 accepted，不读取活动候选。`--resource` 使用 manifest 中 XHTML 的精确 BookPath，不是 href、fragment 或本机路径。查询结果提供 revision、资源哈希与 locator；可读不等于可写，不能凭相似文本选择第一个匹配。

### 2. 生成计划并执行

将以下请求保存为工作区外的 `operations.json`，把旧值替换为书内实际标题。多个标题时须提供明确的可选 `id`。

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
./kepub plan --workspace 'work' --operations 'operations.json' --output 'plan.json' --json
./kepub apply --workspace 'work' --plan 'plan.json' --json
```

计划绑定工作区、accepted revision、完整树、策略与预期旧值；输入过期明确拒绝。正文和结构操作使用各自版本化请求，字段见 [CLI 契约](docs/CLI_CONTRACT.md)，不能把元数据示例改成任意 HTML／patch 写入。

### 3. 审阅、接受或拒绝

用 apply 返回的 `data.taskId` 替换 `TASK_ID`：

```sh
./kepub task status TASK_ID --workspace 'work' --json
./kepub task diff TASK_ID --workspace 'work'
./kepub task accept TASK_ID --workspace 'work' --json
./kepub workspace export 'work' --output 'edited.epub' --json
```

如不接受，在 accept 前改为执行 `./kepub task reject TASK_ID --workspace 'work' --json`。已消费计划不能重跑，旧 task ID 不能操作新候选。plan 报告和 export 输出须在整个工作区之外，父目录存在且输出不存在。pack 的输入是出版根目录，**不是工作区根目录**。

## 能力与边界

| 已交付能力 | 范围与限制 |
| --- | --- |
| 查询与安全解包 | EPUB2/3 ZIP、metadata／manifest／spine／nav／NCX、引用 coverage；不以 spine 合成目录 |
| XML 与来源 | UTF-8、UTF-16 LE/BE、允许的 DOCTYPE 和有界内部子集；不读取外部 DTD，不把生成节点或默认属性当可写原字节 |
| 单操作编辑 | schema 1 元数据；schema 2 简单正文。保留各自旧参数、摘要和权限 |
| 多资源事务 | schema 3，2–256 个既有操作，完整写集合、回滚与来源重算 |
| XHTML 结构 | schema 4，受控属性及插入／替换／删除／同资源移动；统一最终身份与引用门禁 |
| 字面／正则替换 | schema 5，显式 locator 范围、Go RE2、精确命中数和输出预算 |
| 跨资源移动 | schema 6，显式子树移动，三编码保真及证明范围内的链接同步 |
| 修复提案与差异 | schema 7，FixProposal／ValidationDelta；仅两条获准原生修复，不提供任意自动修书 |
| 校验与导出 | 固定 EPUBCheck、冻结输入与最终 ZIP 核验、原子不覆盖发布；草稿与正式产物分开 |

修复提案与诊断差异查询：

```sh
./kepub fix propose --workspace 'work' --json
./kepub fix delta --workspace 'work' --before initial --after-task TASK_ID --json
```

二者只读。执行修复须显式选择实际 repair ID，再走审核事务。提案可包含 unfixable 项，诊断数量减少不代表 EPUB 合规；接口见 [FixProposal/Delta v1](docs/contracts/T2_FIX_PROPOSAL_V1.md)。

### 使用前需要知道

- **原书保护：** 原书、revision、候选及检查点是独立副本。目标外字节与非 manifest 文件／空目录按受支持契约保留；无操作不重排 XML。当前不是内容寻址去重存储。
- **覆盖不等于合规：** 引用 `complete` 只表示对应语法提取完整，不是编辑授权。CSS 仅支持 literal url/import 子集；脚本、SMIL、srcset 等覆盖不完整。校验通过不等于渲染或人工无障碍通过。
- **解析不等于检查器接受：** EPUBCheck 5.3.0 拒绝 UTF-16 XHTML（`HTM_058`）；UTF-16 OPF/container 与 UTF-8 XHTML 的非 strict 正例另有验收。不会为通过检查而隐式转码。
- **预算是产品策略：** 书内路径上限 4096 字节／128 层；展开库存 20,000 个唯一文件／目录（含隐式父目录）、累计路径 32 MiB、256 MiB/文件、2 GiB 实际总量；原始 XML 8 MiB。解析、解码、引用图、输出另有独立预算，详见契约，不因过滤查询或缓存命中豁免。
- **工作区不是 OS 沙箱：** 协作进程锁不防御任意同用户写者。安全候选漂移可审阅／拒绝，来源损坏、链接及特殊文件仍拒绝；不能删 journal、任务或检查点强行恢复。已证明的恢复边界不等于断电耐久性。
- **限制保持显式：** 不解密、不执行书内脚本、不联网读取外部 DTD；未知加密／签名书及字体混淆的编辑资格受限。资源改名、全局工作区注册表、GUI、预览和生产 Agent 尚未实现。

JSON 成功／失败均输出统一 envelope。脚本应核对退出码与稳定错误 code，不解析终端摘要；命令发现、预算、取消和恢复细节以 [CLI 契约](docs/CLI_CONTRACT.md)为准。

## 开发与验证

核心依赖为 Go 标准库、`golang.org/x/text` 和 `golang.org/x/sys`，版本见 [go.mod](go.mod)。先配置真实 EPUBCheck，再运行：

```sh
go test -count=1 -timeout=30m ./...
go test -race -count=1 -timeout=30m ./...
go vet ./...
```

[GitHub Actions](.github/workflows/ci.yml) 保留 PR 自动检查；main push／合并不自动跑全套，按[开发流程](docs/DEVELOPMENT_WORKFLOW.md)评估后，必要时手动触发并填写理由。CI 验证精确提交，依序执行普通／race／vet及普通／race CLI 构建，保存 14 天日志。**评估无需重复不等于新跑 PASS；没有 checks、SKIP 或工具错误均不算通过。** Linux CI 不执行私有原书；缺 checker 的结果不能替代正式验证。现有 Actions 的 Node／setup-java 弃用警告仍待维护，不影响上述固定运行的成功结论。

Orb 的 [.agents/setup](.agents/setup) 准备 Linux amd64 Go、Java 和固定 EPUBCheck；[.agents/resume](.agents/resume) 只做快速恢复检查。SDK 实验有独立锁文件／运行时，根目录 Go 套件不覆盖它；须在 `experiments/amp-sdk` 单独执行 `npm run check` 和 `npm test`。

贡献前阅读 [当前开发流程](docs/DEVELOPMENT_WORKFLOW.md)、[开发约束](AGENTS.md)和[开发注意事项](docs/DEVELOPMENT_NOTES.md)。按风险选择验证，以[交接收据](docs/templates/CHANGE_RECEIPT.md)区分作者、独立审查、父验收和 CI 证据；回归保留旧树反例、合法正控和新树证据。源码交付、平台验收与发行分别记录，不提交私有书籍、凭据或原始敏感日志。

## 路线与待办

最新排期见 [最小 EPUB3 制书闭环总项 #31](https://github.com/LeviTK/Kepub/issues/31) 和 [开发方案 §0.2](docs/DEVELOPMENT_PLAN.md#02-最小制书闭环优先八域三级拆分与交付顺序)：先做正常 Unicode 新书的章节、基础样式、图片、目录、审核与正式导出，再扩展会评、复杂 CSS、旧书迁移和 GUI。八域拆成 35 个模块、128 条三级任务。以下 T/S 名称保留原范围，不再表示必须先做 T3 才能做 T4。

| 阶段 | 当前状态 |
| --- | --- |
| S0：规范资产与差距矩阵 | 有限门槛已验收；归档不等于全规范实现 |
| T1a／T1b：编码、终端与 XML 来源 | 已验收并集成 |
| T2：事务、编辑与修复 | 五个有限增量已集成；字体混淆资格、旧工作区重评、OPF/spine、资源维护及 NCX/SVG/CSS 等仍待完成 |
| T3：EPUB2 → EPUB3 | 待实现；显式转换，不静默升级原书 |
| T4：新建与维护 | 待实现；含章节／目录／资源及按标题拆章 |
| T5：样式、系统字体与基础图片 | 待实现；不新增嵌入字体，系统字体不保证跨设备外观一致 |
| T6：CLI 验收与发行 | 待实现；Linux 与 Apple Silicon Mac 分别验收 |
| S2–S4：预览、交互与完整符合性 | 后置；MyGo／WKWebView 选型及边界实验不等于产品已交付 |

完整依赖与关闭门槛见 [开发方案 §11.7–§11.9](docs/DEVELOPMENT_PLAN.md)。T6 完成也不自动满足全规范／渲染／无障碍总目标。

### 现有 Issue 的归属

| 跟踪项 | 剩余范围 |
| --- | --- |
| [#1 Diff UI](https://github.com/LeviTK/Kepub/issues/1) | Pierre 差异展示与审核 UI；现有终端 diff 不是该 UI |
| [#2 内容寻址版本库](https://github.com/LeviTK/Kepub/issues/2) | 不可变 Blob/Tree/Revision、去重与迁移；现有哈希核验不等于实现 |
| [#3 EPUB 3.3 总目标](https://github.com/LeviTK/Kepub/issues/3) | 规范、编辑、真实渲染与无障碍分维度验收 |
| [#4 生态借鉴](https://github.com/LeviTK/Kepub/issues/4) | 来源、许可、对照测试和可选适配边界，不要求集成所有上游 |
| [#14 路径预算](https://github.com/LeviTK/Kepub/issues/14) → [#28 Mac 验收](https://github.com/LeviTK/Kepub/issues/28) | 产品修复与 Linux 回归已集成；保留 Mac 实机门槛 |
| [#15 查询成本](https://github.com/LeviTK/Kepub/issues/15) → [#29 剩余 A](https://github.com/LeviTK/Kepub/issues/29) | 部分调用级复用已交付；整书快照与安全等价验证仍在，B 依赖 #2 |

2026-10-13 整理结果：#5–#13 已按原范围关闭；#16、#27 已合并；#17–#26 由 #27 集成后关闭，原审查和失败记录保留。关闭修复项不表示整个产品完成。

Issue／PR 使用类型标签（如 `bug`、`enhancement`、`type:performance`）、`area:*` 模块标签及必要的 `status:*` 标签；优先级仅沿用已确认的 P1/P2，不替未来工作猜排期。新标题建议 `类型(模块): 具体目标`，原 R/N 审查编号保留供追溯。新 Issue 写明当前行为、目标／非目标、验收与依赖；PR 写明关联 Issue、精确输入、验证结果和未测项。标签不代替 GitHub 的 open／closed／merged 状态，也不代表开发授权。

## 文档索引

| 阅读目的 | 入口 |
| --- | --- |
| 参数、schema、预算与错误语义 | [CLI 与操作契约](docs/CLI_CONTRACT.md) |
| 路线、架构、阶段和关闭条件 | [开发方案 v0.11](docs/DEVELOPMENT_PLAN.md) |
| 规范覆盖与 S0 证据 | [EPUB 3.3 支持矩阵](docs/EPUB33_SUPPORT_MATRIX.md) · [S0 父验收](docs/verification/S0_PARENT_ACCEPTANCE.md) · [2026 checker 核验](docs/verification/S0_EPUBCHECK_2026.md) |
| 查询、诊断与正文编辑 | [M1-A](docs/verification/M1_A.md) · [M1-B1](docs/verification/M1_B1.md) · [C1](docs/verification/C1_CLI.md) · [C2](docs/verification/C2_CONTENT.md) · [C3](docs/verification/C3_CONTENT_EDIT.md) |
| 工作区、审核、校验与导出 | [M2-A](docs/verification/M2_A.md) · [M2-B](docs/verification/M2_B.md) · [M1-B2](docs/verification/M1_B2.md) |
| 编码、来源与事务增量 | [T1a](docs/verification/T1A_ENCODING_TERMINAL.md) · [T1b](docs/verification/T1B_XML_SUBSET.md) · [T2 五批验收](docs/verification/T2_MULTI_OPERATION.md) |
| 历史审查与开发环境 | [Droid 审查记录](docs/verification/DROID_REVIEW.md) · [Orb EPUBCheck](docs/verification/ORB_EPUBCHECK.md) |
| 后置方案与实验 | [Calibre 研究](docs/research/CALIBRE_CLI_REVIEW.md) · [MyGo 边界](experiments/mygo-boundary/README.md) · [Amp CLI 原型](docs/verification/AMP_CLI_SPIKE.md) · [SDK 原型](docs/verification/AMP_SDK_SPIKE.md) |

历史文档保留当时的未推送、拒绝和未测结论；不能把旧树批准迁移到新树，也不能将设计示例当成当前可执行命令。
