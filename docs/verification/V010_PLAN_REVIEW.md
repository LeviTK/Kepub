# v0.10 开发计划的真实 Droid 审核

2026-10-05，Linux orb。用户授权先审核计划，解决问题后按 S0 → T1～T6 开发并循环 review。本记录只证明实际计划审核及修订，不证明新增功能、S0 资产或发行已完成。父 Orb 维护三份主文档并核验结果；[既有 Medium 编码 Orb](https://ampcode.com/threads/T-01a10d92-bcf1-7410-abf3-603c2849c8fd) 在隔离快照调用真实 Factory Droid，保留原未提交 T1 初稿。

## 固定产品与第一轮输入

父基线为 [已发布 v0.9 文档提交](https://github.com/LeviTK/Kepub/commit/a8920d3d69fb15ebcc1d33bb37c0a41ed7eb3bee)。子 Orb 使用其可用的旧基线，父独立核对两者 `cmd`、`internal`、`go.mod`、`go.sum`、`.agents/setup` 完全一致；未混入子 Orb 初稿。计划审核的产品树固定为：

- `cmd`：`e5a43c345c8c8bb6332da013dd6b718623d71ee3`
- `internal`：`dad94faf92109b25e6225a9dffca3a21a7553616`

R1 docs-only 快照为 `16e9d68258139f4fbc4c7ca9b0c502ea8d72eaf9`，实际输入 SHA-256：

| 文档 | 字节数 | SHA-256 |
| --- | ---: | --- |
| README.md | 31049 | `7de43a94da0364396bf978b4dd5d0c7161b6ff83a34af2cd09210cd195344b2d` |
| docs/DEVELOPMENT_PLAN.md | 144599 | `25e314572cf062a128ac7c8f42260c98fdd132c1bcca8b1a70be4a04d0065de7` |
| docs/CLI_CONTRACT.md | 50775 | `fbee0cc1a9ee1ed95982e040be799a3b22ebb1185b2985e9d2264ece29c715b6` |

## R1 实际完成，但未通过

实际 Droid CLI 为 0.233.0；调用如下，不是 Amp 子代理冒充 Droid：

```sh
droid exec --auto medium -m claude-opus-5-5 -r medium -o stream-json \
  --cwd /tmp/kepub-v010-plan-review \
  -f /tmp/kepub-v010-plan-review-prompt.md
```

真实 init 确认 `claude-opus-5-5`／`reasoning_effort: medium`，session `036842ab-bb8a-4bc8-a210-357eaff2b212`。实际 completion：29 turns、907835 ms；外层进程 exit 0，99 个合法事件，0 error／tool-error。快照审核后保持 clean，产品树与文档 hash 不变。父已下载并核对报告、证据 JSON 与 stream；进程 exit 0 表示审核执行完成，不表示计划通过。

审核全文读取三份文档、Issue #3／#4 正文（当时无评论），抽查解析／持久只读原因／编辑来源相关源码，并核对官方 XML 约束、允许标识符、阅读系统 XML 处理、2026 change log、checker releases 和 epub-tests 来源。没有运行 Go 测试、真实书籍 demo、EPUBCheck 或 Mac 测试。

| 问题 | 等级 | 父核验后的修订方向，尚须复审 |
| --- | --- | --- |
| F1 S0 退出范围与测试提交选取不确定 | High | 有限全文清单、直接依赖盘点、三份 REC 条款矩阵、固定 epub33 源码／工件映射 |
| F2 T1b 必需语法与 T2 入口不确定 | High | 必需内部声明／实体／默认属性语义及来源矩阵，不能以 unsupported 代替；T1a、T1b 都通过才进入 T2 |
| F3 EPUB3 外部 DTD 读取语义冲突 | Medium | EPUB3 外部子集零读取；仅显式 EPUB2 迁移使用冻结的受信离线资产；区分阅读系统与作者工具要求 |
| F4 字体混淆编辑资格与旧只读状态无人负责 | Medium | T2 标准 IDPF 混淆资格和显式版本化重评，字体／密钥不变；未知加密及签名仍只读 |
| F5 T1a 无独立通过条件 | Medium | UTF-16 全链路、search／status／diff、授权原书及合成样本分别验收 |
| F6 缺 EPUB2 规范及 DTD 资产归属 | Low | S0 登记，T3 编码前冻结完整有限迁移输入资产及许可 |
| F7 checker 对 2026 修订覆盖未分配阶段 | Low | S0 核对三项变更，未知已证规则基线明确 unknown，不冒用目标 URI |
| F8 拆章 CSS 扩展缺验收归属 | Low | T5 CSS／片段／导入／字体引用反例，T6 正式 ZIP 闭环 |
| F9 UTF-16 预算字节口径不清 | Low | raw 8 MiB 含 BOM，另设解码／展开 16 MiB 及 DTD 预算，读写／重算统一 |

R1 另记录但未证实为缺陷的观察：新增输入对旧二进制的安全拒绝、诊断兼容说明、T2 依赖撤销形式待冻结、status 经 Open 恢复的既有语义、旧设计的 ID 与现行 DIR 区别。这些观察不计作已复现产品缺陷，也不因后续修订删除原结论。

## R2 实际完成，仍未通过

三份修订文档已实际传输，父 `git diff --check`、代码块配对及新增本地链接检查通过。R2 使用全新审核请求，不复用 R1 完成事件，输入如下。

| 文档 | 字节数 | SHA-256 |
| --- | ---: | --- |
| README.md | 32063 | `f08177eb7e1a36f4476128795b82b7c62f69b863c10cac60e54dbfcb0b20dd0a` |
| docs/DEVELOPMENT_PLAN.md | 153855 | `91c09024cc64851d5fcfbb40f106bb40c7a2f5dde51b38852ba71b98f68ecdd5` |
| docs/CLI_CONTRACT.md | 53107 | `9eb2bf089a5325b3ce616f4e734471dbab966e1f89aef2f04993322301cc2dd6` |

父对两轮任务各发生过一次消息工具解析失败，均先查明目标线程仍 idle、无新请求后才重新发送成功，没有据此重复启动 Droid。R1 的临时规范抓取不是 S0 原文资产交付；未推送、发行或关闭 issue。

R2 仍为真实 CLI 0.233.0／`claude-opus-5-5`／medium，session `8f404989-4a8b-4fcc-b2b8-365643613f94`；实际 completion 为 41 turns、1761745 ms，进程 exit 0，132 个合法事件，0 会话 error、1 个工具 error。该工具错误为 Python 提取规范时空匹配后访问 `None.start()`，已补读；不伪称全部工具成功。Droid 报告称无法查看自身 effort，外层 stream init 已直接证实 medium。

固定快照 `0f1d967e23f1747aafd027c79c9132488f32232b` 前后完全一致，cmd／internal、三文档 hash 和 clean 状态均复核。父已下载、读取并核对最终报告和证据，报告 SHA-256 `7a6da948f450254624921ed6db9e072bb7c67c6f3d74fa9b2565acc92c35305a`，stream SHA-256 `98f96106966bb633425ebaa794c5c0a52cf83f87177366d4a3acf32c2de7bd09`。

R2 全文读取三份文档、两份 issue 正文，先独立分析再对照 R1；抽查源码及官方 XML／EPUB 规范和测试仓库历史，未运行产品测试、demo、EPUBCheck 或私有书。F2／F3／F4／F6／F7／F8／F9 解决，F1／F5 主体解决但有残留；仍确认以下问题。

| 问题 | 等级 | 父核验后的修订方向，须 R3 复审 |
| --- | --- | --- |
| N1 规范条款的计数单位不明确 | Medium | BCP 14 完整实例、定义及无关键字约束；原文位置／hash；机械清点加逐节复核，对账排除理由 |
| N2 良构但无法展开实体的处理缺失 | Medium | 保留原文与未知来源，专用诊断／partial；content/search 不提供残缺成功文本，禁止依赖未知内容的局部写 |
| N3 epub33 报告与共享测试源码不是同一版本 | Low | 报告提交与每个用例源码提交分别固定，追溯改名／拆分／删除的用例 |
| N4 T3 需 CSS 变换但解析器在 T5 | Low | T3 首批保留 CSS，需变换则阻断；T5 交付迁移扩展及正反例，T6 闭环 |
| N5 混淆字体路径变化遗漏 CipherReference | Low | T2 纳入依赖核验；T4a 改名原子同步，不能同步则拒绝；本阶段拒绝删除被混淆字体 |
| N6 EPUB2 合法外部 DOCTYPE 可能误报非法 | Low | 显式版本 profile；EPUB2 非迁移路径能力不足与 EPUB3 规范非法分开 |

同时收紧 T1a 不承担 T1b 专属预算、T5 开工与完成依赖的区别、16 MiB 每资源且计入标记的口径。未证实观察仍保留：NOTATION 的 EPUB 解释需在 T1b 冻结前裁定；checker 的外部子集处理与 Kepub 可能不同；修复撤销的具体版本化入口留 T2 冻结，不能就地篡改已执行任务或抹去来源。

父直接读取 XML 1.0 §4.1／§5.1，确认 Entity Declared 的 WFC／VC 区别；直接核对 epub-tests 历史，确认 3.4 OPF 修改、历史报告改名和 cnt-css-fonts 拆分。父修改文档时一次补丁因“与／與”上下文不符失败，重新读取确认未应用后修正，没有丢弃原文或跳过核验。官方入口 HTTP 200 和这些静态核对均不算 S0 归档或实现测试。

## R3 完整审核完成，保留 1 个 Low

R3 实际 completion 为 37 turns、988054 ms，原进程 exit 0；Droid 确认 R2 问题已解决或归入有门槛的阶段裁定，无 High／Medium，但报告 1 个 Low。父继续核查并修订该项，未将其省略为“零问题”；产品实现仍暂停。

三主文档及本记录的当时版本已实际传入既有编码 Orb，父的 `git diff --check`、代码块配对、15 个增量本地链接检查通过。隔离审核输入如下；后续向本记录追加结果不改变已冻结审核输入。

| 文档 | 字节数 | SHA-256 |
| --- | ---: | --- |
| README.md | 32528 | `a3d91b3609cb2eb5f39b942e4a1eff2f2e662c1391fd04dc8ec8e0cc77074edb` |
| docs/DEVELOPMENT_PLAN.md | 160193 | `dd780c53e41b460ae428e410421f367a58f22917aec07a99214d5ccf78049a56` |
| docs/CLI_CONTRACT.md | 55479 | `d21a9ad0a68a697e29b2d421be69163777ff4e2cce9993e9bbce077172b2bdfa` |
| docs/verification/V010_PLAN_REVIEW.md（启动时） | 8203 | `484e2e3f6ff7c67ac145bb7cffe7dcc641c5a402485be3b3b6d34c1f8625ccd7` |

固定快照为 `59b74b429257f77470afc6ad14bfb417ff98c259`。父已直接读取实际 stream init，确认 `claude-opus-5-5`／medium，session `6b26e581-3d71-4880-922b-f2ba5010ad93`；不能把启动、读取进度或无 error 当作最终通过。

父等待期间补核 EPUB 3.3 §3.9 与附录 B 原文。首次全页工具输出被截断，另用标准库按章节重新读取；临时 Python 检索首次因未安装 `bs4` 失败，未安装或修改项目依赖，改用标准库成功。该规范核对不是 S0 原文归档或产品测试。

父下载并核验最终报告 SHA-256 `c49dd584455a1b47d202943796642da8f89e97bc15bff824584e9c3d675d7020`，stream SHA-256 `58541304f23528b774ccab4ba2e54c15dd7eb5f468894823381f82e216880e2a`。114 个合法事件，0 terminal error、0 `isError` 工具事件；但两次组合命令中均有缺少父提交对象的 `git cat-file` 失败，不能因 shell 最终 exit 0 就说全部子命令成功。隔离快照前后 HEAD／产品树／四文档／clean 状态精确相同；主工作区只有结束时 digest，不能将此扩大为该工作区的逐文件前后证明。

本轮全文读三主文档、Issue #3/#4，再对照 R2；核对相关解析／工作区／CSS 源码及固定 EPUB／RS／XML 章节，没有运行测试、demo、EPUBCheck、私有书或 Mac 验收，也没有全文复核 CSS Snapshot／Accessibility。本轮不能替代任何实现验收。

F1（Low）指出现有 32 MiB 索引将字符数据计两次，并在每层祖先追加子树文本，另计 location；可能在 raw／decoded 限额之前拒绝正文。父读取两个现有解析器并独立执行基线探针：同为 7 MiB ASCII 文本，`html > body > p` 的 raw 为 7340065 B、文本计量 29360128 B，解析成功；再套 `span` 后 raw 为 7340078 B、文本计量 36700160 B，精确得到 `XML text/location limit`；命令 exit 0、输出 `EXISTING_INDEX_BUDGET_PROBE_PASS`。这是已有 UTF-8 行为的证据，不是尚未实现的 UTF-16 通过证据。

父选择保留既有安全预算，在计划 §3.4 与 CLI §2.4 明确独立索引计量、典型深度的有效上限，以及跨 8 MiB 正例使用 CJK 注释／小正文、另测大正文索引拒绝的区别。不新增优化或放宽上限，也不将上限称为必定可处理的容量。

四项非阻塞观察仍保留：UTF-16 CSS 在 S0 登记并归 T5；META-INF／OPF 的资源 profile 由 S0 分别映射；合法外部 DOCTYPE 的 EPUB2 非迁移路径仍有能力限制；T2 新状态须让旧二进制安全拒绝。这些不是已经实现或经过实测的能力。

## R4 定向复核完成，保留措辞问题

R4 只复核 R3 F1 与相关上下文，不是另一次全规范／全源码审计。真实组合仍为 CLI 0.233.0／`claude-opus-5-5`／medium，session `2a3478bc-343e-406f-b1a4-710c01cdd395`；实际 completion 为 17 turns、623770 ms，原进程 exit 0，76 个合法事件。

固定快照 `7690a4f15e58d57d0ce7f73a946a6a15edb70541` 的产品树未变；README 沿用 R3。计划 161064 B／SHA-256 `4168a3967d5acd952684aac481df31c8bd86aa916273616cd9d2112e6b915659`，契约 56114 B／`bc912518c8ebeb286b8903203112de8ed1ed240b9bb050df12b3a8a9e625e553`，本记录启动版 11987 B／`9095adb6919f03c2b85db4a3b1090cdf151cb2106771678688826701dd65960a`。隔离快照前后、主工作区已记录的文档与初稿文件 digest 均一致。

父已下载并核验最终报告 SHA-256 `df9f8df6f01e55ad60e0c98d61a090080c8278152bd73eb5b638de83c3070304`，stream `cbcf0201ab99dcc95b6e5f43f6acc1002c497a14c634b6e4864db564ee3c3b4c`。R4 确认 F1 关闭，索引计量与既有源码吻合；但新列 R4-L1（Low）：跨 8 MiB 的样本句须明确重复 LE／BE 和读取／定位／局部编辑／来源重算／历史重开一致的要求，不能仅验证解析。父在计划与契约各补同一句，未改其他技术语义。

Droid 在独立临时源码副本做了索引等式、精确边界和父探针复现，定向测试与 vet 最终通过；另用放开 raw 检查的 parser replica 和标准库 UTF-16 转码验证样本构成。后者只是模拟，不是产品 UTF-16 支持证据。没有全仓普通／race、EPUBCheck、私有书或 Mac 验收。

失败保留：1 个 `isError` 事件来自 `diff` 的预期差异 exit 1；另有探针失败／panic 被组合命令的最终状态掩盖，以及缺少父／R3 提交对象的查询失败。探针有四类自身错误：空白计数期望算错、UTF-16 样本原始大小算错、按字节截断多字节字符导致后续 panic、replica 原始大小开关未启用；修正后重跑通过。四类不等于只有四次失败执行，不能说本轮全部工具均成功。

## R5 完成，已知计划问题关闭，进入 S0

R5 输入为计划 161189 B／SHA-256 `f7747ebb8836340ac000678a6112fef6b7f67f2fb3afc6693916c5f52ce4cd14`，契约 56239 B／`0a93e146d82fa478239d3928696f8014772db1ebf7e9c4b099ed509361f2f80a`。相对 R4 各增加 125 B，明确两种字节序与完整链路；README、产品未改，审核记录仍用 R4 启动时历史版本。本轮只确认 R4-L1 及上下文一致性，不重复已完成的探针或冒称全量审核。

真实组合仍为 CLI 0.233.0／`claude-opus-5-5`／medium，session `dbeebdea-750e-4ae4-ae13-d7a69463d20c`；实际 completion 为 4 turns、72008 ms，原进程 exit 0，13 个有效事件，无 terminal error／`isError`，stderr 为空。隔离快照 `61dd64bb6ebb0146faddd37948d234ff2a3df386` 前后 HEAD、产品树、文档及 clean 状态精确不变，主工作区已记录文件也不变。

结论为 **R4-L1 关闭，未发现新的相关问题**。父已核对报告与实际 completion 逐字相同，报告 SHA-256 `2293a9ffe6e8834929e3e2cc4b39c26bd2034dd12f9878a81455131e969ea9a9`，stream `591477108ad8614f7f39f54342bfbb89f5620a062a1d20e001309cd1480a3016`。报告表格把 README hash 的缩写尾部误写为 `…cdb`，完整证据仍为 R3／R4 的 `a3d91b36…74edb`；保留原报告并在此更正，不把转录错误变成文件不一致。

本轮未重读规范、核验源码、构建或测试，也未做 EPUBCheck、私有书或平台验收。结合 R3 全文计划审查及 R4／R5 定向关闭，父认为已满足用户的“计划无已知阻塞问题后开发”条件，开始 S0 规范资产与差距核验。四项观察在对应阶段落实；T1 仍须等待 S0 完整通过，不能据此宣称实现无 bug 或所有规范能力已完成。

此后 README／计划只更新审核与实施状态，技术范围和 R5 的预算／验收语义不变。原书已在父 Orb 实际重新取得，3328634 B、SHA-256 `91b9d80c84258c89f47f6faac43eff6477b7c6649140ac848b3dc78924761e4b`，仅供获授权的 T1 验收，不提交书籍。尚未推送、发行或关闭 issue。
