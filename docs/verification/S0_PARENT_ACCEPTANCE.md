# S0 父 Orb 独立验收记录

## 当前结论：阶段资产未通过总验收

**最新用户指令（2026-10-06）：当前审计改用 DeepSeek V4.1 Flash，Droid 待用量恢复后再使用。** 使用 Amp 的实际 `deepseek-v4.1-flash` 模式开展独立完整 S0 审计，不续写或冒充中止的 Droid R6。原快照 D1 已实际结束并给出 approved；父另核出三项报告／阅读记录更正，且该批准不覆盖随后新增的 Amp 验收协议。协议适配固定树已通过父 **124/124** 完整回归与重放，用户随后要求新建独立审计 Orb，D2 已创建，尚无终局决定。固定输入、完整阅读、实际测试、反例和父验收要求不变；当前组合仍未放行，T1 暂停，不自动充值、监控额度或重启 Droid。下述各轮的 Droid 要求及额度阻塞保留为当时历史。

首次检查于 2026-10-05；2026-10-06 已完成下述 P1～P8、Droid R1 F1～F6 的阶段修复复验。后续 R2／P9 及 R3 的来源修复已独立复验并集成本地 `main`，固定修复树与父最终组合均 115 项通过。R4 被子 Orb 重启中断，没有 completion、实际退出码或决定；恢复后完整 R5 实际 completion／exit 0，但决定 rejected。其 contributor 继承及七处普通建议缺口现已修复、父独立复验并集成本地；最后集成树完整 **121/121 PASS，589.402s**。随后 **R6 因 Factory 周额度耗尽（HTTP 402）实际 child exit 1，没有 completion 或批准／拒绝决定**，详见下文。旧失败、阅读限制和错误验收记录保留，不充当当前批准。计划本身复审已完成，S0 仍待最新指定审计者的新完整审查和实际门禁通过；T1 继续暂停。父 Orb 的 [EPUBCheck 增量核验](S0_EPUBCHECK_2026.md) 已通过定向及全仓普通／race／vet、Darwin 交叉编译，但不替代本记录的资产与矩阵验收。

首次受审阶段提交为编码 Orb 的 `b92e1903d831d48ae9df13c8592024e69dac0ed2`；父在独立 worktree `/tmp/kepub-s0-parent-review` 检查，当时没有把阶段资产合入父本地 `main`。以下首次失败记录予以保留；后续修复结果单独记录，不覆盖原结果。检查未修改产品或三份冻结主文档。

## 实际交付与传输核验

- bundle 要求基线 `e8e8b2a539fa61e46cc95d5940b36960e3462e8f`，包含上述受审提交及已移交的父 checker 测试／记录。
- 初始包为 182,927,969 bytes、SHA-256 `c543749ffa0ee8a2f1e70cca9b90ba75f5cc87dd077ba291360767e7ade2fdc5`。编码 Orb 随后实际执行 Git repack 并重新生成包；父下载的是最终包，不能拿旧大小或旧 hash 核对它。
- 最终下载为 60,133,631 bytes、SHA-256 `26f6cf48b0443b2fd3cc624c8f85779084256eb7e182ba9136b3c9cb15a9a824`，与编码 Orb 最终日志吻合。
- `git bundle verify`、实际 `git fetch` 读取对象、`git fsck --full --no-reflogs` 全部成功。不是只以 bundle 头或文件名证明包完整。
- 比对冻结基线：`cmd/`、产品 `internal/`、`go.mod`、`go.sum` 及三主文档未改；唯一新增 Go 文件为父已交付的 `internal/validation/rec2026_test.go`。

## 既有检查实际通过，但语义门槛仍阻断

在隔离受审树运行：

```sh
python3 -m unittest discover -s scripts -p 'test_*.py' -v
python3 scripts/epub33_assets.py verify
python3 scripts/epub33_tests.py verify
python3 scripts/epub33_tests.py reproduce
python3 scripts/epub33_upstreams.py verify
python3 scripts/epub33_assets.py gate
```

- 既有 Python 测试 18/18 通过，0.061s。
- `assets verify`：104 文件，749 个候选全为 pending，`semanticComplete:false`。
- `tests verify`：169 用例，报告 `sourceGaps:0`，`executed:false`、`semanticReviewComplete:false`；离线重建成功。它验证的是库存／工件，不是执行这些阅读系统用例。
- `upstreams verify`：14 个研究项目，1 个已披露许可缺口，`behaviorTested:false`。后置 adapter 许可不明继续阻止采用，不等于阻止研究登记。
- `gate` 实际 exit 1：`S0 semantic gate blocked: pending clause/section review (not an asset PASS)`。749 条候选、500 个核心章节，以及未标记约束、直接依赖适用性、官方用例条款映射和配套资料盘点仍待完成。
- 离线重建未造成已跟踪文件漂移。Python 仅产生可删除的 `__pycache__`。

## 父新增三个反例全部真实失败

父独立阅读归档和脚本后构造反例，不改变原资产来伪造正常结果。在受审树运行上传的 `kepub-s0-parent-negative-tests.py`：3 tests，2.300s，**FAIL 3/3**。

| 编号 | 观察与复现 | 应满足的约束 |
|---|---|---|
| P1：来源缺失仍报零缺口 | 在临时副本将 `pub-data-urls_browsing-context` 的 source commit/archive/hash、generated path/hash、website path/hash 置 null，并令 `failures=[]`。`epub33_tests.verify` 跳过 `if sourceCommit` 分支，仍返回 `sourceGaps:0`。原包该条目未缺失，这是验证器反例。 | 缺少完整来源证据必须验证失败；缺口根据实际必需字段／工件计算，不能只相信独立的失败列表。 |
| P2：真实 schema 依赖遗漏 | 受审包 `mapping.rnc` 明确 `include "../mod/datatypes.rnc"`，`ocf-metadata-30.rnc` 明确 `include "./package-30.rnc"`。两目标均不在 manifest；`assets verify` 仍通过。 | 归档 schema 所需的 include/external 及传递依赖属于本阶段必要资产，不是可后置的参考文献清单。缺失必须被检查器发现。 |
| P3：重定向后版本身份错绑 | 模拟固定 `2026/REC-demo-20260113/` 请求返回 `2027/REC-demo-20270113/`，正文 This version 也为 2027。`fetch`、`index`、`verify` 均未拒绝，manifest baseline 仍为 2026。原包未观察到该重定向，这是捕获／校验器反例。 | 固定版本身份必须绑定请求的规范版本，不能只验证 final URL 出现在返回正文。合法原版本是正例，新版本重定向是反例。 |

P2 两个遗漏 URL 均位于 [EPUBCheck 固定源码](https://github.com/w3c/epubcheck/tree/029831b8f477e4519e9734c984ee24357547a698) 的 `src/main/resources/com/adobe/epubcheck/schema/30/` 下，分别为 `mod/datatypes.rnc`、`package-30.rnc`。后续须继续闭合它们的必要依赖，不能仅补两个文件后停止检查。

父反例脚本 SHA-256：`ec975736bfec2e23369d44c40ba49eda450b4c1bb96417000c834eddb71e7dfe`；原失败日志 SHA-256：`d59b019115416cb68d2bdfc3e2e78abce7bff397d1b19795197b0886f0f5a152`。两文件均已实际传给编码 Orb，在 `.agents/` 保留为交接证据，不是产品运行依赖。首次直接上传 `/tmp` 日志被工具拒绝，复制到工作区后上传成功；没有据失败上传声称交付。

## 三项修复已独立复验并集成本地，语义门槛仍未通过

编码 Orb 随后交付 `dc7d603` 与 `1c81f5b4740d64890a74784a2354e42ce169f46d` 修复。父实际下载 `.agents/kepub-s0-assets-fixes.bundle`，16,991 bytes，SHA-256 `f3f0fc15b019703c31af4b2427b02927d68aa2f01defdcfe657eef8a7218da58`，核对 bundle 前置、fetch 对象并逐项检查差异；不是只根据子线程的通过声明验收。

- 原三条独立反例现在作为 `scripts/test_epub33_parent_acceptance.py` 保存，字节与上述原 SHA 完全一致。在同一隔离 worktree 的固定修复树执行：**3/3 PASS，0.044s**；没有为适配实现而改写反例预期。
- 完整 Python 集合 **27/27 PASS，0.122s**；`assets verify`、`tests verify`、`tests reproduce`、`upstreams verify` 全部成功。
- 归档增至 **107 个资产／12,565,293 原始字节**。仅增加三个必要 schema；原 104 项的字节、hash、路径、下载时间与 final URL 均未变。正式用例工件、上游记录、待审矩阵、产品 Go 代码和三份冻结主文档未因这次修复改变。具体闭合和反例见 [修复记录](S0_ASSETS_FIXES.md)。
- 父将原资产及两次修复集成本地 `main`，集成树与受审树的资产、脚本、产品和主文档逐项比对无差异。集成后完整 Python 集合再次 **27/27 PASS，0.113s**；三个 verify 均通过。未 push、发布或部署。
- 全增量 `git diff --check` 因原样归档的上游 RNC 空白返回 **2**；脚本和记录范围的检查通过。不修改上游原始字节来消除该提示，也不声称全增量空白检查通过。

集成后再次实际执行 `python3 scripts/epub33_assets.py gate`，仍 **exit 1**：`S0 semantic gate blocked: pending clause/section review (not an asset PASS)`。749 个候选、500 个核心章节仍待语义审查，能力五维仍为 `not-tested`；336 条直接引用／227 个不同 URL 的适用性、阶段归属及 169 个官方用例的固定条款映射尚未完成。官方用例仍为 `executed:false`，不把工件重建当作功能执行。

P1／P2／P3 已关闭，但完整语义矩阵、父语义复验及固定全 S0 树的真实 Droid 审查尚未完成；不得把本阶段记录当作 S0 PASS、T1 实现或发行声明。

## P4：报告 ID 去重遗漏同一用例必需的第二本书

2026-10-06 后续独立抽查发现，`pkg-unique-id` 不只是报告中同一行意外重复。已归档第一本书的 XHTML 要求阅读系统把 *Unique identifier not unique* 与 *Unique identifier reused* 分别显示；这是两本书共同完成的用例。当时索引只有 `tests/pkg-unique-id` 的一套源码／工件，却仍由 `tests verify` 返回 `169 / sourceGaps:0`。该输出不能证明完整配对工件已经归档。

父直接读取固定报告提交的 Git tree，确认还存在 [tests/pkg-unique-id_duplicate](https://github.com/w3c/epub-tests/tree/54092b4233253e9aac80e93ec4782b380b4b3403/tests/pkg-unique-id_duplicate)，包含五个普通文件，tree 为 `1bb88991c92b28e1cbfc49e85eb91070ff864965`。它的正文给出相反方向的配对显示要求，`dc:identifier` 同为 `pkg-unique-id`，而 `dc:title` 为 `pkg-unique-id_duplicate`；不能根据相同出版物标识符去掉第二份工件。

父实际下载 [官网第二本 EPUB](https://w3c.github.io/epub-tests/tests/pkg-unique-id_duplicate.epub)，**1,976 bytes**，SHA-256 `2f1b1967d196d94ed6e330bfe037e7eff44447d5802fbe2158de30c829cb09b7`；逐一下载固定提交中的五个源码文件，与 ZIP 的完整普通文件集合逐字节比对，全部一致。另保留一次查证失败：外部研究最初误称该源码提交中也有二进制，父直接请求该固定提交下的 `.epub` 得到 HTTP 404，Git tree 也确认没有二进制。之后确认的是官网成品与固定源码的内容一致，不是凭构造的 GitHub 链接认定二进制存在。

新增独立验收脚本 `.agents/kepub-s0-parent-paired-fixture-test.py`（SHA-256 `fd4eb99969063dc293cf7973f29425ac75d2a1107428ec88ecfa082fdfcb1379`），要求第二本的源码、官方成品和本地生成物均匹配父独立取得的五个文件 hash。在修复前父树运行 **1 test／FAIL，0.006s，exit 1**，原因是索引没有该配对来源。失败日志 SHA-256 `4bc92781eeae6e59dd438b1f184b7bd0f0c73b93ce42a9aa22238b3a01c1ad12`。这不是更改原资产后制造的反例。

P4 已交编码 Orb 补齐配对工件并增加缺失／伪造配对来源的验证反例。仍分别统计 169 个用例 ID、170 个报告行和实际工件数，不把第二本伪称新的规范用例；修复与父复验未完成前，此项继续阻断 S0。没有执行该阅读系统行为测试，也没有发现或宣称对应 CLI 产品缺陷。

### P4 独立复验通过并集成

父实际下载修复 bundle，9,228 bytes、SHA-256 `4dc415b61a16e1cb8796a2fe7c955909dccb503d0b571e1185b27c2f574f1f01`，verify／fetch 成功；受审提交为 `adc835c4b4008e4b964e4ca5288910d55b8e2001`。在上述独立 worktree 固定该提交，原样父配对测试 **1/1 PASS，0.013s**，完整 Python 集合 **32/32 PASS，0.370s**，含四个新增缺失／伪造配对证据反例。`tests verify`、`tests reproduce`、`assets verify`、`upstreams verify` 全部通过；重建后无跟踪文件变化，增量 `git diff --check` 通过。

父逐项比较原 169 条 case 数据，去掉新增 `pairedFixtures` 后与修复前完全相同；原源码／成品／生成物均未替换。验证结果现在区分 **169 cases／170 reportRows／170 publications**。父集成本地 `main` 后再跑完整集合，**32/32 PASS，0.425s**，verify 同样通过；受审树与集成树的脚本、资产、产品和三主文档无差异。

[子 Orb 的 38 项记录](S0_ASSETS_PAIRS.md) 属于其当时还叠有未提交语义测试的工作树；这里固定 P4 提交实际只有 32 项，不能把 38 项归给此提交。此次修复不改变产品 Go 代码，无新增 Go 执行结论。P4 关闭，S0 的完整语义整合、父语义验收与真实 Droid 审查仍未通过；未 push／发布，T1 保持暂停。

## P5：非规范性参考资料被抽取成规范性引用

父将 227 个直接引用的阶段归属作为独立只读研究交给有界 subagent，自己复核输入身份、全部排除项、版本／条件差异及 XML／URL／Unicode／字体／旧版样本。该研究不是 Droid，也不是全部引用已独立通过父语义验收。

研究发现 CSS Snapshot 的 33 条非规范性引用被误标为 normative。父实际复读 `original/css.html` 第 7584 行起的 `<h3 id="informative">Non-Normative References` 及其书目，确认该观察；`source_inventory` 先做 `"normative references" in heading` 子串匹配，因此把 `Non-Normative References` 也归为规范性引用。原文不需要修改，需修复抽取和重新生成派生清单。

父新增 `.agents/kepub-s0-parent-reference-tests.py`（SHA-256 `f4f42e2899aa246b7db97e0e45cc70d88ff7873f6b3fdc6b9bf0209d55ca67ab`）：一个规范→非规范→规范的合成控制，以及真实 CSS 八条书目断言。在修复前父树实际运行 **2 tests／FAIL，9 个含 subtest 失败，0.012s，exit 1**；日志 SHA-256 `fed8ce678b76ac71adc2727a52021ae519ddaa7b80bcb5b88fb3eb355bf6026d`。先将代码和测试交脚本所有者修复，再做下述固定树复验，不改写这次失败。

研究草案实际输出 `.agents/kepub-s0-parent-dependencies-review.json`，412,716 bytes，SHA-256 `75897ba88561782c39bc93600b57d7d900844157377bd1ff45e5106ac6a012e7`，绑定原外依赖输入 SHA-256 `6a2f061df2e22df6d9b14caba93534a4d38c7dec62547a3c80b6a6f4aa966090`。父程序逐项检查 227/227 的 URL、citations、下载状态、hash 和版本字段保持原样；197 项 mapped、30 项有理由 excluded，后者均为资料性引用。草案保留 P5 错误标记，未升级任何下载状态。修复后须显式修订并重新绑定新输入，不能篡改原稿 hash 伪称早已基于正确清单。

另确认 336 条原文书目中只有 332 条附带 URL：`epub/bib-us-ascii` 及 `aria/bib-dpub-aria`、`aria/bib-epub-3`、`aria/bib-wai-aria` 原文确无 URL。父要求单独记录这些书目，按 document＋entry＋原文 hash／DOM 绑定，URL／未下载内容 hash 保持 null，不凭题名合并到可能不同的版本。引用页面的取得时间也不能被标成目标外文的下载时间。T3 的额外 DTD／NCX／DTBook 清单只登记后续资产前置；付费 ISO 继续披露后续缺口，不购买、不把目录页当全文。这些决定不放松 S0 对实际必需归档范围的要求。

### P5 独立复验通过并集成

父实际下载固定修复 bundle，4,248 bytes、SHA-256 `4ce5383eec28d263d99c4bd1e710efd8486fee5edabe0dbcba078644970c76c6`，verify／fetch 成功；受审提交为 `5d3e7e8b7c96f873ec09fe1afd5710e071a034e1`。父隔离固定树上原样 P5 测试 **2/2 PASS，0.011s**，全部 Python **34/34 PASS，0.378s**；三个 verify 及官方工件 reproduce 全部 exit 0，重建后无跟踪文件漂移，增量空白检查通过。子侧一次使用错误脚本名的失败保留在 [P5 修复记录](S0_ASSETS_REFERENCES.md)，不与正确命令的通过结果合并。

父逐项比较 JSON，确认仅 33 条 CSS 书目的 kind 从 normative 改为 informative，并同步相应依赖引用；336 条原文书目、227 个 URL 和其他库存字段均未改变。新外依赖基线 SHA-256 为 `764a400496929be751f1189018ce2f18780caedb7a2e89ea60b856b95d34ba95`。原规范文件、P4 的 170 本测试书和产品代码均未改。

集成本地 `main` 后全部 Python 再次 **34/34 PASS，0.425s**，资产 verify 成功；与受审树的脚本、规范资产、产品和三主文档比对相同。P5 关闭，但当前稳定树仍只有 749 条 pending 矩阵：后续完整语义增量尚未父验收，`semanticComplete:false`，不得据此开始 T1 或宣称完成 S0／Droid 审查。

## 完整语义增量首次验收：既有检查通过，独立反例仍阻断

父实际下载语义提交 `a330430703fcf35f0fa02de14c19cc99be32b465` 的 bundle，**1,115,832 bytes**、SHA-256 `7b648efb7445fbd4e12ef7d803844df1d6af08e535449d38ff431d1d69d7427f`，前置 P5 提交；verify／fetch 成功。在隔离 worktree 固定此树，确认产品、Go 依赖与三主文档无变化；没有先集成再把作者的通过声明当父验收。

- 全部既有 Python **59/59 PASS，114.516s**。原父 attributes／linear default／landmarks、P1～P5 控制均通过。
- 四个资产／用例／上游／语义 verify 与官方工件 reproduce 全部成功。107 资产，1474 条矩阵（1430 mapped／44 excluded），500 核心章节，169 cases／170 reportRows／170 publications，283 配套章节和 63 CSS 模块。五维能力仍 `not-tested`，官方用例仍未执行。
- 原父外依赖研究与原输入保留；重新绑定后的 33 条 P5 修订、227 个 URL／332 条带 URL 引用、4 条独立无 URL 书目都有记录，不声称下载或版本等价。
- `gate` 实际 exit 1。此时没有 pending 矩阵行；`semanticComplete:false` 仍要求父语义验收和随后固定树 Droid。通用错误中“pending clause/section review”不能解释为旧 749 条仍未整理。
- 首次按文档完整重建造成 `matrix.json`、`MATRIX.md`、`derived.json`、`semantic-index.json` 四文件漂移。父逐身份比较 1474 rows／725 manualConstraints／500 sectionReviews 全相同，其他 metadata 也相同，仅排列不同；仍不符合固定提交的字节可复现要求，不能用第二次幂等冒称首次无漂移。

### P6：明确的非规范性说明被登记为规范约束

固定 EPUB REC 原文中 `sec-container-abstract-intro`、`sec-docs-intro`、`sec-nav-def-types-intro` 均有 `class="informative"` 和 “This section is non-normative”。§1.5 明确排除这种来源的规范性。父从矩阵发现 **10 条 manual mapped** 仍来自这些章节：6 条容器文件角色、1 条 SMIL text 说明、3 条导航类型说明；SMIL 那条同时写着 mapped、`S0-excluded` 与“不适用”，自相矛盾。

父新增 `.agents/kepub-s0-parent-informative-tests.py`，源 hash 固定到上述 REC；要求保留这些来源记录但按理由排除，并以真正规范章节的 SMIL text REQUIRED／toc SHOULD 为不受影响的正控。实际运行 **2 tests／10 个 subtest FAIL，0.882s，exit 1**；正控通过。原脚本 SHA-256 `473ec3a68978c5be7ff56d5f13cd49bcbcf3a58877aedc21d9254d6d29e68c09`，失败日志 SHA-256 `79efea9edb9521c5156e1ebcb320f36571704cb9c564b9f8336a29b8449c8b8b`，均已实际传给编码 Orb。

此项要求修正来源适用范围的继承与 amendment 决定，不能直接删除观察或把对应正式章节的要求一并排除。不是 CLI 解析／渲染功能的失败。

### P7：手工条款的版本与章节可自洽错绑而不被拒绝

父在内存副本中同时修改 `manualConstraints` 与对应 row：把 manifest item 定义的 `specVersion` 换成虚构的 EPUB 3.4／2027 URI，或把 `specSection` 换成真实但无关的 `sec-nav-toc`。原始归档、source hash、DOM 与片段保持；`verify_mapping(matrix=...)` 两次均接受，因为 row 只与手工记录比较，没有将手工记录的版本／章节重新绑定原 manifest 与 DOM。

新增 `.agents/kepub-s0-parent-manual-provenance-tests.py` 实际 **1 test／2 个 subtest FAIL，10.763s，exit 1**；脚本 SHA-256 `cd3831d5fb127aeabd351e559d174472e93ad3eb23bbdc1c89673a12d06588dd`，红日志 SHA-256 `929cc410d50f953c50951e80d96f960177f016c4c493a12f4ff6c35a6e8a45ef`，两者均已传输。这是验证器反例，不声称收到的真实矩阵已经指向 3.4。

P6、P7 和首次重建漂移等待最小修复及原样父复验。完整语义增量尚未集成，S0 未通过，T1 仍暂停，尚未启动完整 S0 的 Droid 审查；本地新增记录没有 push／发布。

### P6／P7 及固定重建复验通过并集成

父下载相对 `a330430` 的修复包，**28,281 bytes**、SHA-256 `86be16ae6c6ae0f147cd6b332196713b635ea241f0c4350f045a288217cfb09d`，verify／fetch 成功；包含重建排序提交及受审终点 `dbf8263482754647a25ffdbdd681ee472d5191fc`。在隔离 worktree 固定终点后，原父 P6 测试 **2/2 PASS，1.187s**，P7 **1/1 PASS，10.336s**；仓库保存的两个原反例脚本与前述 SHA 完全一致。

- 完整 Python **66/66 PASS，139.675s**，包括有效矩阵正控和原样 P1～P7 反例；不能仅用另一处拒绝导致的偶然绿灯证明来源修复。
- 逐身份比较首次语义提交：1474 行身份全保留，恰好 10 行由 mapped 改为 excluded，仅 decision／reason／applicability／phase 等决定元数据变化；725 条手工来源和 500 条章节记录内容完全相同。真正规范章节的容器、导航、SMIL 要求未删除。
- 按文档完整执行 review／amendment 导入、两个 index、四类 verify、官方工件 reproduce，均 exit 0；随后 `git diff --exit-code` 与增量空白检查均通过。首次重建现在就与固定提交逐字节一致，不是只验证第二次幂等。107 资产／12,565,293 原字节未变。
- 当前计数为 **1474＝1420 mapped＋54 excluded**，无 pending 行。能力五维仍未测；169 个官方 reading-system 用例没有被执行。`gate` 仍按预期 **exit 1**，固定树检查命令最终输出 `S0_PARENT_P6P7_FIXED_TREE_REPLAY_PASS` 只表示上述复验通过，不是 S0 总门槛通过。
- 父将完整语义、排序与两项修复集成本地 `main`，集成终点 `5b36d89`。脚本、全部规范／用例资产、矩阵说明、产品与 Go 依赖与受审树无差异；集成树完整 Python 再次 **66/66 PASS，140.595s**。本次未重复无产品变化的 Go 大组合，不把旧 Go 结果标为新产品实现结果。

P6／P7 和固定重建问题关闭，继续进入固定组合树的真实 Factory Droid 审查。三主文档仅同步 S0 进度而未改变范围／接口，父记录保留所有历史失败。已有验证、源映射及抽查不宣称穷尽证明、完整规范支持、渲染／无障碍通过、S0 总验收或 T1 解锁；未 push／发布。

## S0 实现 Droid R1 已结束，但不批准进入 T1

R1 在固定 fixture `be198c651be1197ae62109dd76e6d4d083cb1b23` 上运行，其完整 tree `da66b3ce0e8c6be3ec1320531032dddc54c70be5` 与当时父本地 `main` 完全相同；fixture 仅承载父的精确文档，不是另一个产品实现。实际 CLI **0.233.0**，stream init 明确 **claude-opus-5-5／reasoning_effort medium**，会话 `6b0b6a6e-c262-4267-930e-9a062aa9dd34`；原 shell PID 83184／Droid PID 83193 正常 **completion、exit 0，94 turns、1,587,830ms**。退出成功表示审查进程结束，不表示审查批准。

父下载并核对原始报告／evidence／完整 stream／探测包，SHA-256 分别为 `452ef6fbef8e19608fec5df8c5f006c437f440edac475f5b9f9c3415fd217be7`、`56620d0594341fdcb32838b2c7622e30f8f388d5ecd062211895728bb8c34e9c`、`6820a9988ac04d922a02a449318234e076d815f2727f8ae7eacc6eab9cdc1b78`、`460e2dfc79f124809283d2bd7e7eab027692bc6871bef6358654d67de9bf6e99`。前后 HEAD、tree、工作树及四文档 hash 均相同。Droid 临时生成后删除的 Python cache 不计产品改动。

- 实际正控：Droid 独立完整 Python **66/66 PASS，104.003s**；真实 JAR 的 `TestRealREC2026` **12/12 PASS，54.9s**；文档流程重建无跟踪文件漂移，正常 gate 为 exit 1。
- 发现 F1：只修改 `semanticComplete` 并重建，门禁便可错误放行，未核对必需阶段证据和当前输入身份。F2：已有代码／测试关联未登记，任意非空但不存在的 evidence 路径可支撑 `supported`。F5：执行状态硬编码为 false，不能拒绝伪造的 PASS；删掉已知许可缺口后可报告零缺口。这些是验证器及交付数据问题，不能因正常样本测试通过而忽略。
- F3／F4 是继承列表条件和规范自身 `data-tests` 关联遗漏，父复核如下。F6 是三份直接链接 NVDL schema 未归档；固定 EPUBCheck 源码中均真实存在，须补有限依赖闭包和目录许可，不能标为无效链接规避。
- 阅读限制：R1 全读四份主要脚本；未覆盖块中的 RS／a11y 逐条读，EPUB 409 个未覆盖块仅约前 220 个细读、其余粗看，Notes／CSS／测试源码抽样。不能把本轮称为全部 S0 原文、代码和测试全文审查，也没有执行全部官方阅读系统用例或 Mac 验收。
- 工具失败保留：准备时 fetch bundle 的不存在 `HEAD` 引用失败，改用真实命名 ref 后成功；Droid 临时统计脚本把 list 用作 `Counter` 键，实际 `TypeError`／exit 1，非仓库测试失败。其最终文字说无法自证模型，与父保存的实际 init 可以并存，不据此声称更换了模型。

父随后在本地 `e0976e5` 的一次性资产副本中重跑原 `mutants.py` 的 `pos / gate / support / exec / upstream` 五组，仅重新绑定仓库路径，未改主树数据。正控四类 verify 均 exit 0、gate exit 1；翻转 `semanticComplete` 后 gate 错误 exit 0；不存在证据的 supported、伪造官方 executed/PASS 均未被 verify 拒绝；删除许可缺口并伪造已测试状态后 upstream verify 仍 exit 0、`gaps:0`。因此 F1／F2／F5 在父环境亦复现，不只是转述 Droid。探测器进程 PID 514333 实际 exit 0 仅表示五组探测运行结束，不是上述反例通过；原日志 SHA-256 `dbfd1d78ecc794819a9222adc8c64d457cf7d11d64bb3f8ae82b290b35b8201e`。

### F3／F4：父原文对照确认遗漏，也排除探测器误报

父只改 import 路径重跑 Droid 原 `lists.py`，确实检出 26 处引导句。但裸 `dt` 标签并不自动等于漏条款：Dublin Core 字段名、MathML 标题和 viewport 的 name／content 标题已有相应 `dd` 约束；RS 的 “Some uses … include” 是例示。真正缺失的允许／禁止列表须分别保留其 MUST、SHOULD、MAY、排除条件及 one-of／any-of 关系，不把每个允许值误写为必须同时使用。

父新增独立 `kepub-s0-parent-list-backlink-tests.py`，在原固定父树运行 **2 tests／58 subtest FAIL，3.159s，exit 1**：47 个具体 leaf 约束缺映射、11 处明确 REC 条款上下文未进入用例关联。47 项包括文件名 18 类中的 20 个叶项（非字符范围含三个子项）、data URL 四场景、禁加密八文件、manifest 五属性、toc 两顺序、自定义属性两保留域、远程资源四族及无障碍同步两顺序。不是以整段引导句中“有冒号”作为已覆盖证据。

规范三个固定原文含 **162 个片段 ID／237 次 `data-tests` 引用**，其中 158 个 ID 属于冻结的 169 个用例；另四个未匹配 ID 和 11 个无 REC 反向关联的既有用例需如实登记，不凭拼写相似自动改名，不替换为 3.4 用例。外部 `epub-structural-tests` 的 **456 次引用**与本轮 169 个报告用例分开。父将 `dt` 配到 `dd`、span／cell 配到所属条款后，仍确认 11 处遗漏；追加 REC 关联不得删除原报告的目标或把 `executed:false` 提升为通过。

脚本 SHA-256 `b4fafbb877d5aea0436946dd3c0c14a0fa05aff595b875aacb5f0aec6e190c3d`，原失败日志 SHA-256 `fb1b19bef831953589ead392d9840690349aafb660c22cdf697899af13552e8d`；两者已实际上传编码 Orb，后续应原样复验。这些定向覆盖不替代其他列表、全部上下文和外部依赖的审查。

### P8：示例伪代码已被错误提升为规范要求

父没有直接采纳 R1 的“17 个叶步骤只登记 9 个，所以再补 8 个”建议。固定 EPUB REC §1.5 明文 “All algorithm explanations are non-normative”；`obfus-algorithm` 又明确 “The following pseudo-code exemplifies the obfuscation algorithm.”。现有九条伪代码 `manual-normative` 记录仍为 mapped，应保留原身份并注明排除；不能因为补覆盖而把说明改成强制要求。该节真正的 MUST（压缩前混淆）及 key 推导的 MUST 仍须保留。对前文算法定义与解释须分别核对，不能整节粗暴排除。

父独立脚本 `kepub-s0-parent-algorithm-tests.py` 实际 **2 tests／10 failures，10.114s，exit 1**：九条状态错误，另一个验证器反例表明重新强制 mapped 也不被拒绝。原有效矩阵 verify、原身份完整性及两个真实 MUST 正控通过。脚本 SHA-256 `d29d51d3b109c1e4a33dd3a00d25dd5fbfe618c32f04df73d7800a2c60b9e7a2`，红日志 SHA-256 `481a11e826c6090cebd92ad2e4df12ecf400cc6d144ab5dd3bc90d382a0d80bc`，均已实际传输。父一次只读探查误用不存在的 section 名称导致 `StopIteration`，改读真实 `obfus-algorithm` 后获得上述原文；不将该工具错误当产品失败。

当前只完成发现复核与原始反例交接，修复包尚待父验收。三主文档仅同步事实状态，不收窄 S0 清单或产品规范承诺；R2 未启动，T1 暂停，没有 push／发布。

## F1／F2／F5 首批修复已独立复验并集成，其余问题仍阻断

父实际下载 `294cc8d7c342c99e36335412c6db6f48f0125d02` 的首批 bundle，**18,518 bytes**、SHA-256 `778735fe8976ead1bcd9c42df91abf5c914b34856b7120d1963a580e0e2b1564`，前置 `dbf8263`；verify／fetch 成功。在原独立 worktree 固定新提交，完整 Python **79/79 PASS，185.517s**；源码／测试与 [批次记录](S0_ASSETS_R1_GATE_EVIDENCE.md) 已逐项检查。不是根据子线程的 79 项通过声明直接合入。

- 按更新后的文档首次执行完整 review import、两个 index、四类 verify 和 official reproduce，全部 exit 0；`git diff --exit-code` 与增量空白检查通过。正常 `gate` 实际 exit 1，明确为 `independent acceptance records missing`，不是无效来源造成的偶然拒绝。组合最终输出 `S0_PARENT_R1_VALIDATION_FIXED_REPLAY_PASS`。
- 同样的五组原 `mutants.py` 探测在新固定树重跑：有效正控保持；bool 翻转不再放行；不存在 evidence 的支持声明、伪官方 executed/PASS、伪上游行为／采用均被所属 validator 和 aggregate gate 拒绝。其他无关 validator 的成功不能代替所属检查。固定树探测日志 SHA-256 `bd135c2e4c07d33b177ea6c384ed3526682e0edaab1a89e38154897679f48054`。
- 父另跑真实 aggregate 集成对照，**1/1 PASS，108.314s**：在一次性完整副本中，仅重新绑定仓库根，不 mock 任何底层 validator；缺少凭据先拒绝，明确标注 synthetic 的一致凭据可通过，再分别修改 Go 代码、验证脚本和契约，使旧凭据因输入身份变化拒绝，恢复原字节后又可通过。临时目录自动清除，真实主树没有 `acceptance.json`，也没有真实 Droid 或父批准。该测试不是 S0 语义通过，只防止“门禁永远失败”制造假绿。脚本／日志 SHA-256 分别为 `1e3bc4df0a6a1ff79aa09fee208f70b6e29082e2b735d09365c2c3f813a96b38`／`f91de9b41411771c453f0e516f337bbff5f21fcf0b302b57df11b28893634834`。
- 独立逐身份比较确认：1474 个 row、725 个手工来源、500 个章节记录身份保留；仅 13 行的 `evidence/testIds/gap` 改动，五维未提升。107 原始资产、170 必需测试书、上游原始文件、产品 Go、module 和本批三主文档没有改动。引用存在不等于执行通过，viewport／SVG 样本仍只代表既有窄范围。
- 父本地集成为 `de53b5704799ac4885cf8bafe10a88c19c864ae5`，资产／脚本／产品与受审树完全一致；集成后完整 Python 再次 **79/79 PASS，181.240s**。固定／集成日志 SHA-256 分别为 `0a8e2b3fa879ef76f3ff11d94c9d19ee995d416296427022fc198de84e64eaec`／`9e78eeeb2b0f0872628471b891856b2e36a23fb7b4a6e621ce1d55ff0423c5f4`。本批不重新标注旧 Go 大组合为新结果。

F3／F4／F6、P8 及 T3 前置机读登记仍由编码 Orb 修复，尚未父验收；新完整 Droid R2 未启动，S0 不通过，T1 仍暂停。以上仅是首批局部问题关闭，不是当前所有问题清零；未 push／发布。

## F3／F4／F6、P8 与 T3 前置登记已独立复验并集成

父实际下载相对 `294cc8d` 的第二批 bundle：**147,713 bytes**、SHA-256 `715fd53fb1c3a461c2264da2c6c071fb86d8c5a39b2dc7e3ca86fd565ace9e57`，verify／fetch 成功；受审提交为 `df57cddddae3653a6197d9f071957169ede1902f`。在独立 worktree 固定此树，完整 Python **92/92 PASS，261.055s**，其中原父列表／backlink 和算法反例脚本与原 SHA 逐字一致。测试日志 SHA-256 `571e532c06b6262150c82eb668ab8feb2c66c835996111de27c669b476a63b6a`。脚本及测试差异已实际阅读，不只依赖子侧通过报告。

- 首次按新文档完整执行三 review／三 amendment 导入、两个 index、四类 verify、official reproduce，全部 exit 0；`git diff --exit-code` 和 staged diff 都为零。正常 gate 实际 **exit 1：independent acceptance records missing**。组合输出 `S0_PARENT_R1_SOURCE_FIXED_REPLAY_PASS`，日志 SHA-256 `71e211291886802bb5f8e21c6de5891d7573daf58d00485f1bd4f762b674be56`；不是 S0 总门槛通过。
- 父逐组读取新增 59 个叶项及其真实引导句，核对 filename 禁止条件、data URL iframe 例外、八项禁加密文件、manifest 条件属性、remote MAY、namespace 排除域、fallback one-of、toc／sync 两顺序、skip／escape 非穷尽可选项和 a11y any-of 条件。原 1474 身份无删除；11 个算法解释观察保留为排除，真正的 key／压缩顺序／算法标识 MUST 不受影响；28 条旧引导句仅改正“已含完整列表”的误述，5 个旧叶项补上下文，44 个旧 `dd` 补相邻 `dt` 绑定。500 section records 原样保留。
- **1533＝1468 mapped＋65 excluded**，784 manual，五维仍全部 `not-tested`。来源匹配反例覆盖同时篡改 manual／row 的真实但不相邻上下文和 `dt`、整组删除绑定及算法说明重新升级；不能只以 hash 自洽接受错误语义关联。父原定向覆盖不声称穷尽证明所有剩余语义。
- 固定 REC 的 237 处片段引用／162 ID、158 个已知用例、4 个未匹配 ID、11 个无 REC backlink 的既有用例，以及 456 个外部结构测试链接分开保存。父原 11 条遗漏反例现通过，原报告的出版物侧条件保持；169 个官方 case／170 report rows／170 本书仍未执行阅读系统行为测试。
- 父直接从固定 EPUBCheck 源提交独立取得三 NVDL 的 **14 schema＋目录 LICENSE**，15 项 SHA-256／字节数全部与交付 raw bytes 相同。来源哈希清单 SHA-256 `bdc4cda83a5c1bf7fcf083c047f0f21cef41728c3b1bee478ca8eea5b8ff6899`。共 **118 文件／12,599,439 原始字节**；原 107 个资产的 URL、取得时间、路径、长度及 hash 均保留。目录 LICENSE 的 IDPF MIT-form 不以根 BSD 替代，不宣称法律意见或执行了这些 schema。
- T3 八家族登记逐项复读：前三保持固定 bibliography 的原版本／URL（包括 draft 命名和 `.doc`），其他精确 legacy edition 仍待确定；全部 pending，取得 hash／path 为 null，禁运行时下载。此登记不完成 T3 的有限离线依赖归档，也不新增迁移功能。
- 父本地集成提交为 `cdf8725`，脚本、全部资产／矩阵、产品、Go/module/setup 与受审树相同。父拥有的主文档仅改为引用实际验收记录和固定输入门禁，不改范围或接口；避免为了写下审查结论而再次改变其已审输入。真实批准证据将独立保存，不把此阶段记录写成已批准。

子侧首次 91-test 运行的一个新增测试失败保留：测试按相同 XPath 误选了 RS 记录，不是其意图的 EPUB 同节相邻绑定反例；修正选择条件后原异常预期不变。最终子侧 92/92 PASS 229.677s 与父独立结果分开。初次 NVDL 捕获因 namespace 不完整只得 111 文件的错误成功也保留，最终以精确 15-file 闭包和独立字节对照验收；原样上游文件的 whitespace 检查 exit 2 不伪称通过。详细记录见 [第二批修复](S0_ASSETS_R1_SOURCE_CORRECTIONS.md)。

当前已复现的 R1/P8 修复已关闭，但新固定树仍未获 Droid 批准。下一步是冻结最终组合输入并执行真实完整 R2；不拿 R1 的局部阅读、正常进程退出或这批 92 个通过测试替代。T1 暂停，未 push／发布。

### 最终组合已冻结，R2 已真实启动但尚未完成

父集成树完整 Python 再次 **92/92 PASS，270.394s**，日志 SHA-256 `d6e6f59bea54c1f3338d65c4980dadfa7c54a91cca39bfb3dc89ede59c16fea8`。五份文档的相对文件链接及增量空白检查通过；产品和矩阵数据没有再改。固定本地 `main` 为 `c73c43b9c37678746da8b1d94a6169e4008bcfda`，审查 fixture `4bf36edc73cc014581bd991d15ce64cc1d7fb35a` 与其完整 tree **`1f23318d56fc9bfc58bcf5876d158e64d6ed2e87`** 相同；fixture 仅用已有子提交作父节点以传递五份文档，不改变任何受审内容。

实际上传 bundle 为 **20,823 bytes**、SHA-256 `2cd0ad872fad8e06344b5f35263605b52855b8a590f54c66d1aae8738f0aaf94`，前置 `df57cdd`；子实际 verify／fetch 并核对 tree。双方重算 **821 个 acceptance inputs** 逐项相同，canonical SHA-256 **`088c7013ff7420b5b3178e24c9b574edace562d750257824c8cb79dbe4e47788`**。输入清单文件 104,705 bytes、SHA-256 `1d0376ced5c986e403fa61ebb3506ba12e8b1363e90559ca99036ae377679108` 已实际传输。此进度记录不参与该输入身份；报告／批准须独立绑定它，不通过修改原文或布尔状态关闭门禁。

新独立 R2 的实际 CLI **0.233.0**；stream init 确認 **claude-opus-5-5／medium**，会话 `67e3d3fb-70c7-4c48-a091-6e77ecc9c53a`，外层原进程 PID `118306`，固定目录 `/tmp/kepub-s0-droid-review-r2`。目前只有启动与阅读进度证据，**没有 completion、实际 exit 或批准结论**。初次发送启动指令异常后先查证未送达，再重新投递；文件传输成功不误当作任务已启动，没有并发或重复审查。继续跟随原进程，不提前恢复 T1。

## S0 实现 Droid R2 已实际结束并拒绝批准

上述同一 R2 会话正常 **completion、exit 0，98 turns、1,330,527ms**，最终决定为 **rejected**。父下载实际完整 stream、最终报告和退出文件，并核对报告与 completion 的 `finalText` 相同。固定 HEAD／tree 及 821 输入身份未变；进程退出成功不是审查通过。

- 原报告 **9,203 bytes**、SHA-256 `e10b5836ea28059b131b1cbbe17559f1733decc391e7b68c76b446366b80d9e6`；stream **1,303,653 bytes**、SHA-256 `af1898224e9230e16c3556cf7ee85dc540286e7b3271070ad8dfcceb136b948c`；实际退出文件为 `0\n`，SHA-256 `9a271f2a916b0b6ee6cecb2426f0b3206ef074578be55d9bc94f6f3fe3ab86aa`。另下载的 evidence JSON 仍是较早 running 快照，不能据其认定终局状态。
- Droid 独立完整 Python **92/92 PASS，225.096s**；真实固定 JAR 的 `TestRealREC2026` **PASS，62.8s**；离线重建无漂移。正常 gate 因缺少独立批准而 exit 1，一条组合命令的非零退出由这个预期门禁失败造成，不是前述测试失败。测试成功仍未发现以下规范遗漏。
- **F-A**：`sec-container-iri` 正式 URL 算法的 `ol[1]/li[9]`、`li[10]` 两个返回 true 分支未映射。**F-B**：`sec-property-datatype/ul[1]/li[2]` 的空字符串无效约束未映射。**F-C**：`sec-alternate` 表内 `li[2..4]` 的三项 alternate 语义，以及 `sec-nav-def-model/ul[1]/li[1..2]` 的导航列表语义未映射。相关章节却仍声明 complete。它们是来源矩阵遗漏，不能写成 CLI 运行该算法失败。
- **F-D**：从 review 的 rows 和 manualConstraints 同时删除既有 URL 算法 `li[8]` 后重建，assets／semantics／tests verify 仍通过，计数 1533→1532，section review 仍 complete。该变异改变受审输入身份，旧批准会失效；**未证明能够绕过真实且绑定原输入的 approval**。所需修复是阻止既有身份静默消失，并独立于剩余 rows 核对已审核的有界结构家族，不是声称程序能证明全部自然语言规范已完整理解。
- **O-1**：伪造 official index 顶层 `reportCommit` 或 upstream `role`，各自 verifier 仍接受。这是验证器反例，不是收到的固定资产已经伪造，也不证明绕过受审输入绑定。

阅读声明须保留边界：R2 自报 `readingComplete:true`，但实际 reader 把**全部** `pre` 块截断为前 160 字符；父直接读取 RS `app-ers-idl` 的规范 WebIDL，规范化正文共 245 字符，含 `Navigator.epubReadingSystem` 声明，不能把这类块一律当示例免读。治理文档的完整阅读也未由父独立确认。`cov` 文件只是给同一正文加覆盖标记，`¶` 嵌套块另行输出，因此不能仅凭换成 `cov` 或出现 `¶` 就断言遗漏正文。下一轮须读未截断的规范性内容，不以这个自报布尔值证明全文已审。

父另实际下载原探针包 **498,346 bytes**、SHA-256 `880ea853337520835450747901b280af8364f7743fddd009dae79db8b0780e1e`，检查 reader／cover 与原日志。原报告声称探针目录全删除，但编码 Orb 发现保留目录并据此归档；此文字错误保留。部分早期探针打印的 rc 是管道末端状态，不能据其单独判定 validator 成败。编码 Orb 的独立最终 helper 复现 F-A～D／O-1，exit 0 仅表示观察脚本完成；早期 helper `KeyError`／exit 1 也不抹去。

## P9：纠正 P8 对字体算法定义的过度排除

父重新核对官方历史后确认：§1.5 的 “All algorithm explanations are non-normative” 指算法中具名的 `details/summary Explanation`，不把算法所有描述段落都排除。官方 [core 变更](https://github.com/w3c/epub-specs/commit/642a45d0cd81df784ff852d32a55a0f28f87d083) 同时加入该句并移除具名 URL Explanation 内的 `p.note`；[RS 变更](https://github.com/w3c/epub-specs/commit/271f0d7f7c05652ee0fcf77fafe6ac89dc9b08e4) 同样加入说明和具名 Explanation。父实际下载并阅读两份 patch，不只依据研究摘要。

`obfus-algorithm` 的 **p1／p2／p3 是规范算法定义**，分别规定前 1040 bytes、XOR、20-byte key 循环与尾部复制；应恢复 p1／p3 原身份为 mapped，并补 p2。此前父接受把 p1／p3 与九条示例伪代码一同排除是错误，前文“11 个算法解释观察”的结论不再适用。九条明确由 “exemplifies” 引出的示例伪代码仍排除；真正的 Explanation 后代也不能被提升成规范条款。现矩阵未观察到后一种误映射，但变异反例表明验证器尚未拦截。

父独立新增 `.agents/kepub-s0-parent-algorithm-scope-correction.py`，SHA-256 `d1d8ce4749e5fec5932684fdaea5e962cd8e97e671fb67ba30d7b13cbb4aad4a`，在未修复父树实际 **4 tests／5 failures＋1 error，23.542s，exit 1**。有效矩阵和九条伪代码排除正控通过；三定义映射失败，恢复原身份被错误拒绝，而 EPUB／RS 的具名 Explanation 反而允许伪造 mapped。红日志 SHA-256 `d54b373db1a1b8dc12e9b721537abb115d885c721e5eee108d6175e375e2c239`。脚本及两份官方 patch 均已实际上传编码 Orb，要求原样保留父反例并公开纠正旧测试，不删掉失败记录。

当前授权修复仅限 S0 来源数据／导入／验证及相应测试。导入完成所有 review 和 amendment 合并后、写文件前检查既有身份不得丢失；正式 algorithm 步骤／分支和明确审核的列表／定义家族须从冻结 DOM 独立枚举，不能让宽祖先、幸存 sibling、重复项或 Explanation 代替成员。该有限核对不等于增加通用的每段落义务台账，也不取代当前输入的父与 Droid 独立批准。父仍须验收实际修复和选择范围；**R3 未启动，S0 未批准，T1 暂停，未 push／发布**。

## R2／P9 修复已独立复验并集成本地，尚待新审查

父下载固定修复 `4fed666666a2c16380d19b4f18c4e69384afc0f4` 的 bundle，**27,322 bytes**、SHA-256 `cf334f3a710baf0b7a3734f89f03d9a872f1d959a82c352c9d96ac2c76b9dd9b`，前置 `df57cdd`。verify／实际 fetch 成功。初次 fetch 使用 `refs/heads/*` 通配，但该包仅导出 `HEAD`，没有取得对象，后续引用查询／checkout 失败；改用包内真实 `HEAD` 后成功。这个交接命令错误不是修复代码失败，原工作树也未被错误 checkout 改动。

- 父在原独立 worktree 固定修复提交，逐项阅读三实现文件、新测试、14 个新增来源及修复记录。完整 Python **105/105 PASS，412.481s，exit 0**，日志 SHA-256 `e38887dbaa084d535ddbce7d74b480c47ad8f631f0a8ad4a337ad45dbc1c8948`；新增 P9 脚本与父原 SHA 完全一致，原九条伪代码控制也保留。不是只依据子侧 105/105、307.891s 的通过声明集成。
- 首次按更新文档导入三 review／五 amendment、重建两个 index、四类 verify、official reproduce，全部 exit 0；随后 `git diff --exit-code` 和 staged diff 为零。正常 gate 实际 **exit 1：independent acceptance records missing**，不是来源错误造成的偶然拒绝。组合输出 `S0_PARENT_R2_FIXED_TREE_REPLAY_PASS`，日志 SHA-256 `92b823b3eedbb3c6a90611c390925f3ff40babe80a33105b9072d95addd3bcc6`。
- 逐身份对照：**1533 个原身份全保留，14 新增；1547＝1484 mapped＋63 excluded，798 manual**。恰好原字体 p1／p3 两条的决定／理由／适用性／阶段改变，原来源字段不变；500 section records 原样保留，五维仍全部 `not-tested`。118 原资产、全部官方来源／工件／上游原始数据、产品 Go、module/setup 和三主文档无变化。两个新增官方历史 patch 与父独立取得的字节／hash 相同。
- 新增来源分别保留 URL 两成功分支的 OR／AND 区别、空 property 限制、alternate 的两种条件语义及免除超链接义务、nav 的 a／span 含义、文件路径循环条件和 RS 的条件分支。多段步骤的后置判定／返回已有独立原记录，新验证器要求它们继续存在；不能以首段代替。27 个有限列表／定义家族来自固定 DOM，不靠幸存 sibling 触发，也没有扩为自动判断所有自然语言段落。
- 真实反例覆盖删整族／整算法、宽祖先／重复兄弟／Explanation 替代、删后置段落、导入失败后矩阵字节不变、伪 reportCommit／上游角色，以及合法 pending 正控与 gate 负控。旧诊断测试与父原预期未被放宽；F-D 仍不被描述为绕过真实当前输入批准。
- 子侧首次完整 **105 tests／2 FAIL／281.886s** 保留：坏输入实际被拒绝，但新增检查改变旧诊断顺序；修正诊断／顺序后，15 项定向和完整 105 项复跑通过。先前非法 HTML 夹具和文本抽取诊断失败同样保留，详见 [修复记录](S0_ASSETS_R2_CORRECTIONS.md)。本批未改变产品，未把旧 Go 组合重标为本批新产品验证。
- 全增量 `git diff --check` 因原样历史 patch 中的空格／tab 返回 **2**；排除原始 patch 后源码、派生数据和文档范围的检查为 **0**。不格式化官方原始证据来制造全绿。父本地集成提交为 `5c8ad7e`，脚本、资产、产品、Go/module/setup 与受审树逐项相同；父另统一矩阵说明的计数与 P9 更正，不改规范或产品契约。

这批已复现问题关闭不等于 S0 获准。最终父组合及新固定输入的完整 Droid 审查仍须实际结束并通过，再建立真实批准记录；当前无 `acceptance.json`，不恢复 T1，未 push／发布。

### 父最终组合通过，新完整 R3 已真实启动

父完成说明文档校正后固定本地 `main` 为 `03d7eef0a4f126a77d4ab0e589e186e230c81c45`，完整 Python 再次 **105/105 PASS，369.169s，actual exit 0**；日志 SHA-256 `9a29d9589f089d36b0cf31a1dd73be52d3f5d41264bf8352a86a071cb9b6526d`。文档相对链接、增量空白及产品／资产与受审树对照通过。R3 fixture `25746cf144224401c9d0a8fa3f2d3096ddbc8f00` 的完整 tree **`028ea3ee7d35358704c7c331b140e75ae022a114`** 与该父提交相同；它仅以子修复提交为父节点承载五份父文档，不是另一份实现。

实际上传 bundle **26,690 bytes**、SHA-256 `78ff8f6e9bfcf245d5c8bed8e1d8740baa13fcb4bce74ce9ee88e7509caaeb73`，前置 `4fed666`、实际 ref `refs/heads/s0-r3-review-fixture`。输入清单 **105,575 bytes**、SHA-256 `7b480064be55723d9294476e99705e12d73840970cb153326626e6cd8e97b5f5`；双方逐项核对 **828 inputs**，canonical identity **`15de0229f9d4d83bd2d741040b2d6baac44bbd6ce43f1fd8d861aea6ae124341`** 相同。此进度记录不改变受审输入，也不向审查树补写批准。

真实 R3 目录 `/tmp/kepub-s0-droid-review-r3`，外层原 PID **163961**；CLI **0.233.0**，父实际下载 init 核对 **claude-opus-5-5／reasoning_effort medium**，新会话 **`c2941b54-5e19-4699-b9f1-e65f32f8658a`**。不是续 R2 或只审修复片段；要求完整治理文档、实现／测试及未截断的规范正文，包含语法和 WebIDL。当前只有启动和 running 证据，没有 completion／实际 exit／批准结论。

准备时子 Orb 一次 fetch 在非仓库的 `/tmp` 执行失败，改到正确隔离目录后核验成功；父首次长启动消息遇连接异常，先查证仍 idle 且未收到指令，再重新发送，避免重复启动。两项工具失败保留，不算产品失败，也不把成功传文件当审查完成。继续跟随唯一原进程；T1 暂停，未 push／发布。

## S0 实现 Droid R3 已实际结束并拒绝；父确认新的有限遗漏

同一 R3 会话实际 **completion＋exit 0，72 turns、1,142,164ms**，250 stream events、零 error 事件。最终决定 **rejected，readingComplete=false**；正常退出不是审核批准。父下载完整 stream、报告、evidence、exit 及探针包，并核对报告等于实际 completion 的 `finalText`。固定 tree 和 828 输入身份保持原值；没有 `acceptance.json`。

- 原报告 **9,563 bytes**，SHA-256 `1bea493d32a70316a843c370a9760088846bf82bd7ea308b741c79079aebbf2b`；stream **828,201 bytes**，SHA-256 `994979a30fa17f0832bb471bdd0110077d5a736af2d8e95b8b831194d2df364d`；探针包 **107,142 bytes**，SHA-256 `5b50165094cfd2334488aea26986cf02d629cf4140c724714c985494fe05f32b`。实际 exit 文件仍为 `0\n`。
- Droid 的完整 Python **105/105 PASS，298.633s**；真实 EPUBCheck 5.3.0 的 `TestRealREC2026` 12 子例通过；首次离线回放、四类 verify、reproduce 和 no-drift 通过。正常 gate 因缺少独立批准 exit 1。原报告把负控计为 21；父后读原 `gate_ctl.py` 和 `gate_ctl.log` 确认是 **20 个负控均拒绝＋1 个合成一致批准正控通过**（N0、N2～N20，P1 不是 N1），纠正本记录先前沿用的错误计数。这只说明凭据／输入一致性检查有效；**不能据此认定真实批准被绕过，也不能用机械检查证明自然语言规范完整**。
- 阅读边界：R3 阅读实现和多数治理文档，但开发计划仅部分、多数测试未逐行读；主要核对三个 REC 的未映射残余，未通读完整 REC。部分原 Read 输出被截断，不能把随后局部重读补成全文阅读。其 `readingComplete=false` 如实保留。

父逐项读冻结原文后确认：**F-R3-1** 的通用豁免必须同时满足“不被 spine 引用”和“不直接嵌入”两个条件；**F-R3-2** 的 SVG reference／inclusion 分别关联 SVG 文档和 §6.2.3 共用限制。这些定义没有映射，不是声称当前 CLI 的 SVG 或资源行为已实际失败。

**F-R3-3** 的元数据文本／空白、主标题／作者顺序、数据块例外、脚本上下文、meta 两表达形式、linear、路径／URL、默认词汇表、属性命名空间、role 顺序及媒体类型编码／安全定义也需补。父特别复核三个易误判边界：`dc:subject` 仅在指定 scheme 要求时区分大小写；role 的小写 should 不提成 BCP14 MUST；包媒体类型的 UTF-16 binary 是 MIME content-transfer-encoding，不是改变 EPUB ZIP 算法。脚本语义登记不是授权本轮增加脚本执行功能。

XHTML 的两段近似原文须区分：`sec-overview-relations-html p[3]` 继承 `sec-intro-relations class=informative`，不能提升为要求；真正应补的是 **`sec-xhtml-req p[2]`**，保留 “Unless specified otherwise” 的覆盖条件。父初次精确搜 “inherits” 只找到前者，随后读 §6.1.2 才确认后者；不能据最初搜索把报告中这一类遗漏整体判为误报。

父新增独立 `kepub-s0-parent-r3-definitions.py`，在未修父树实际 **4 tests／45 failures，27.131s，exit 1**：41 个固定原文定义无直接映射，四组全部移除仍 verify 成功；原有效矩阵和 informative／normative 两段区别正控通过。脚本 **6,957 bytes**、SHA-256 `26f6a9f35b9539545a234fed0dc9636771fa425843ece110e841db7041b52d9e`；原红日志 **41,834 bytes**、SHA-256 `6f7856ed98cea2178e9783fb47cd3408bd11e73a7c88bb5f558713d13d892077`，均已实际传给编码 Orb。有限反例不能替代其他未读条款的审查。

现仅授权原编码 Orb 修复 S0 来源数据、有限来源对账及测试，维护原身份、资产字节和五维 `not-tested`。相关章节须记录实际语义而非用模板充当全文证据。父继续拥有主文档和最终验收；**修复未验收，R4 未启动，S0 未批准，T1 暂停，未 push／发布**。

## R3 修复已独立复验并集成本地，尚无新审查批准

父实际接收 `4bb5cc4e341700ee6fc5b310a50c3a2b4a599569` 的固定修复 bundle：**40,094 bytes**，SHA-256 `6a685e7c5254ca1d86c0c4f1a9c567067e1341c7a0d1012a6c22031aa5568815`，前置 `4fed666`、实际导出 `HEAD`。verify／fetch 成功，在原独立 worktree 固定此树后，逐项阅读实现差异、amendment 全部来源规则、章节说明及新增测试。一次合并输出被工具截断，父随后按具体 rule 序号读回遗漏文本，不把截断输出当完整阅读。

- 父独立完整 Python **115/115 PASS，549.780s，exit 0**，日志 SHA-256 `4f83ba63ba09d5d4f0839c850c2f15986b774e32be2b5822457c0062532ee1d8`。父原四项脚本 SHA 与上传件完全一致；新十项附加测试分别核验真实原文、advisory 等级和删除后拒绝。子侧完整 **115/115 PASS，435.073s** 是另一份结果，日志 SHA-256 `8cd7ba4fd05a9bdbe7de6e194e1ef907ac79c94a596366ea05939c73c2728cf7`，不替代父检查。
- 父首次执行三 review／六 amendment 导入、semantic index、四 verify、official reproduce，全为 exit 0；tracked／staged diff 均无漂移。正常 gate 实际 **exit 1：independent acceptance records missing**，组合输出 `S0_PARENT_R3_FIXED_REPLAY_PASS; GATE_EXPECTED_EXIT_1`；日志 SHA-256 `2ce899473780c9dfbbb4c86310f4a6d1e08b32e50a79593816060718d65b0ae1`。子侧 replay 也通过，日志 SHA-256 `785d2e0b4f4868e05c4bcdb9c3c2a2fce9dfd73675c88d41f161ffdc0174bb3f`；两侧都没有真实批准记录。
- 父逐身份逐字段对照：原 **1547 rows／798 manual 全部不变，51 新增**；现 **1598＝1535 mapped＋63 excluded，849 manual**。500 章节身份／状态不变，仅 25 条 reviewer／notes 更正为具体有界阅读。118 原资产、官方来源／工件、上游资料、产品 Go、module/setup 均无变化，所有五维仍 `not-tested`。新 46 组来源结构对账仍不是自动证明自然语言完整。
- 父追加十项来自同组原文：数据块免 fallback、标题／作者四项小写建议、meta property/text 定义及 meta／link／manifest／spine 的默认词汇表。实现保留条件与建议性质，没有把它们变成必须新增 CLI／脚本／UI 功能。父确认 AND／OR、条件大小写、HTML 继承、SVG 两范围及 MIME transfer／ZIP 区别；旧行没有被新行或宽祖先覆盖。
- 新 `sectionNotes` 只修改既有且与原 DOM 一致的记录；无源、空或非字符串 note 拒绝，失败不部分写矩阵，重复导入字节幂等。新增回归和原 pending 正控／gate 负控通过。子侧最初 **3 tests／8 failures** 与追加十项 **1 test／10 failures** 保留；不删旧红记录或改变父预期来制造绿。
- 父本地集成为 `791f147`，脚本、规范资产／矩阵和产品与受审树完全相同；父另更新矩阵说明计数、有限范围与完整 replay 命令。增量空白检查通过。本批没有产品改动，不将旧 Go 组合重新标为本批新结果。读取子线程状态曾误返回旧 R2 结果，父用具体 R3 提交／进程重新查询并核对实物后才验收，没有采用错误摘要。

当前仅关闭上述已复现的 S0 修复。最终父组合及新固定输入的完整 Droid 审核仍待完成；**S0 未批准，T1 暂停，未 push／发布**。

### 父最终组合通过，完整 R4 已真实启动

父最终组合固定于本地 `main` **`4c7e7ab9dc56181dd1aad43ae4d273666d894084`**，完整 Python 再次 **115/115 PASS，562.210s，actual exit 0**；日志 SHA-256 `f54be9e2b001990817848b7743eda76a3e54dcfca62b2ec32b6852d2e55aae50`。源码／资产与固定修复一致，文档链接及增量空白检查通过，tracked clean。fixture **`ae6a8a0f681344b6dbc26595410d205c7e07b56d`** 与父完整 tree **`320f45c42ce652f1393ffabbc48895fd15598b9c`** 相同，以子修复提交为父节点只承载父文档，不是另一个实现。

已实际上传、verify／fetch 的 bundle **31,247 bytes**，SHA-256 `138834d710e39ad2c26bd6d63351cf5b8318f0fb8f233363a9f9e4bfdb0a8239`，前置 `4bb5cc4`，ref `refs/heads/s0-r4-review-fixture`。inputs 文件 **106,255 bytes**，SHA-256 `de9872ba473d66090aa02e87405b95427508bf1ad1c2b06f5027d7fa8bbe1b68`。双方重算 **832 个 inputs** 逐项相同，canonical identity **`5b08614f87ba713dfc736cbf29d25d1eb236b90d4275c133b42fe1616f11baae`**。本进度记录不属于批准输入，也不产生真实批准。

R4 冻结目录 `/tmp/kepub-s0-droid-review-r4`；唯一启动外层 PID **206057**，同次实际 CLI **0.233.0**。父下载初始 stream **114,073 bytes／28 events**，直接核对 init 的 **claude-opus-5-5／medium**、cwd 及新会话 **`55477517-bf66-4b68-8899-60f3171fe579`**；该快照没有 completion 或 error，不能据此声称审核通过。实际 prompt 也已下载全文核对，范围是完整 S0，批准前须完整核心规范／治理／脚本／测试阅读，不只是 R3 的 51 项或未覆盖残余。

父两次启动消息因多行 JavaScript 参数字符串构造错误没有送达；每次先核对仍 idle、最新消息仍为准备完成，修正字符串后才成功发送。最初把错误称为连接异常的推测已纠正；没有重启或并发 Droid。继续跟同一会话至实际 completion／exit 后核对决定和阅读范围；**T1 暂停，未 push／发布**。

## R4 因子 Orb 重启中断，没有终局审查结论

原进程后来不再存活，也没有实际 exit／ended 文件。完整保留 stream 的最后事件为 **2026-10-06 07:03:59.998 UTC**，随后子 Orb 的 `/proc/stat` 显示本次启动时间 **07:04:08 UTC**；原 `/tmp` 实现、审查和探针目录均已消失。重启原因和原进程精确终止状态未知，不能猜测为某个 signal／137，也不能把 shell 状态工具找不到 PID 当作 Droid 实际 exit 1。

父实际读取并核对保存的 **180 条完整事件、1,111,749 bytes**，stream SHA-256 `12ee92902a697b5bab7387960d59097ee7a955ba86dd4df489130ab9b9d74b80`，无 completion／finalText。中断证据包 **376,199 bytes**、SHA-256 `87b38b7febc7e92db8bd5f854ff0c76c7285e455545f81cc99051782ff9c4772`，保留初始输入、prompt、launcher 和实际 tool calls／results。两条 isError 是 grep／状态组合命令返回 1，不作为产品测试失败；中间推理中的调查也不是最终 finding。没有完整回放、全文阅读或正式 JAR 本轮通过结论。

启动前固定 tree／832 输入身份已核验；原目录丢失后无法核验其结束状态，重新恢复相同提交不能反向证明原会话没有改动。R4 只记为**环境中断**，既非批准也非拒绝，不代替新完整审核。

恢复仍使用原编码 Orb 和同一冻结内容。后续审核／scratch／证据改置持久 `/home/user/` 路径，先无模型测试受监督服务、日志和原子 launch-once guard，避免服务重启重复发出模型请求，再单独授权新轮次。受监督服务也不保证跨整机重建保留运行进程。**S0 未批准，T1 暂停，未 push／发布**。

### 持久恢复与防重复启动已核验，新完整 R5 已真实启动

父实际核对准备包 **52,352 bytes**、SHA-256 `6878af2fb452fe1c3ed0914f7b2d2269b70844749cceb20152f046ba88901dfe`，全文读取启动脚本、完整审核 prompt 及三份 dummy 原日志。真实受监督服务的三种控制分别为正常 child exit 0、故意 child exit 7、运行中停止而 child 结果未知；服务自动／显式再次启动均被原子 `mkdir` guard 抑制，没有重跑 body，也没有覆盖原退出值或伪造中断 child 的结果。真实模式缺授权先返回 64，不发模型请求。上述是启动防护验证，不是产品测试或 S0 批准。

原编码 Orb 从实际保留的 bundles 恢复固定 fixture 到 `/home/user/kepub-s0-durable/review-r5`，实现及 scratch／探针也使用独立持久目录，未覆盖原 T1 初稿。双方重算 **832 项输入及 canonical identity** 与 R4 启动前清单完全相同；父本地只追加本验收记录，不改受审输入。Go／Java／固定 EPUBCheck 版本已实测恢复，准备阶段的空测试仅证明编译就绪，不冒称执行 12 个 checker witness。

父核验准备后单独授权唯一新 R5。实际下载的 init **1,389 bytes**、SHA-256 `5a35feceaadf6bc85bee508586927df9b475bc88376ab6149e4443701bb90656` 确认新会话 **`d29068a1-ec74-47ef-8999-e60bd42e71d4`**、上述持久 cwd、**claude-opus-5-5／medium**；CLI 实际 **0.233.0**。冻结 fixture／tree／输入身份仍为前述 R4 的同一内容，完整 S0 范围不缩为增量审核。当前仅确认启动，尚无 completion／actual child exit／最终决定；继续跟随唯一受监督会话，**S0 未批准，T1 暂停，未 push／发布**。

## R5 实际结束并拒绝；父独立确认继承及建议登记遗漏

同一会话实际 **completion／child exit 0，124 turns、1,840,968ms**，原始决定 **rejected，readingComplete=true**。父实际下载报告、完整 stream、退出文件和证据包，核对报告等于 completion 的原始 `finalText`。报告 SHA-256 `cb8d9f8b1b600154b6e2cba55265aaecfeebd46aa467d29625fd3b4bf20f9708`；stream **1,804,821 bytes**、SHA-256 `ed94ec64c4d946ec63aa9eb139e8a6285de2a0667fe123adf92869a4b31bfa4f`；完整证据包 **1,349,900 bytes**、SHA-256 `66a65d2a970383ccbcaac4a8c83b6e4482a0028ae8770db7629a8df1a4e90b41`。原始冻结 HEAD／tree／832 项身份前后相同，没有真实批准。服务自动重启只进入 guard 停车，未重复发模型请求，随后停车服务已停止。

- Droid 独立 Python **115/115 PASS，551.780s**；真实 EPUBCheck **2 tests／12 subtests PASS，51.711s**，无 skip。首次完整 replay、四 verify、official reproduce 及三次 tracked／staged diff 检查通过。正常 gate 仍 exit 1：缺少独立批准。**14 个合成门禁对照＝1 个正控＋13 个负控**，全部符合预期；不代表官方阅读系统用例已执行或真实批准已产生。
- R5 声明全文读完 EPUB **5784 单元**、RS **1948 单元**、A11y **799 单元**，以及治理文档、四生产脚本和 19 份测试／1932 行。父查看实际分段 Read／批量读取命令和 reading ledger，不把计数或映射前缀本身作为语义完整证明，也不声称父重新全文读了一遍三个 REC。原报告把 B.1 WebIDL 归于 EPUB 的措辞有误；实际 `EpubReadingSystem`／`Navigator` 声明位于 **RS B.1**，ledger 和原读取内容保留正确位置。
- **F1**：`sec-opf-dccontributor p[2]` 明文继承 `dc:creator` 的其余要求，但整节无矩阵行，section review 仍 complete。父直接读冻结原文及所有相关行复现该遗漏。修复须保留 p1 的 secondary-role 区别及 “in all other respects”，不把 contributor 改成 primary creator，也不把可选 refinement 变成强制功能。合成正控接受带此语义缺口的数据，说明机械检查不能证明自然语言完整；**不是绕过已有真实批准**。
- **C5**：父逐条确认七个普通建议来源：identifier 持久性、额外日期、subject 标签／条件性 code 许可、修改日期更新、多个媒体类型中首项优先、使用 DRM 时的隐私偏好、RS 的 CSS 支持／用户代理样式说明。应与既有 title／creator 普通建议一致登记，不将小写 should／条件 must 提升为 BCP14 MUST／SHOULD，不授权开发 DRM、CSS 渲染或新 CLI 功能。

父独立反例 `.agents/kepub-s0-parent-r5-inheritance.py` **4870 bytes**、SHA-256 `682ab95860030d86de36c18e26910466556614c1e7c828f05d4aebc502df608b`，在未修树实跑 **3 tests／11 failures，22.466s，exit 1**：8 处缺直接来源记录，3 处完整来源族在成员不存在时仍验证通过。原有效矩阵和既有 title advisory 正控通过。原红日志 **10,862 bytes**、SHA-256 `a9375cff0c1181a1c5a195142ad62b3964cd1a1d6d8101bb74e77adfa842ce30`；两者均已真实上传原编码 Orb，要求保留原预期并做最小来源／有限对账修复。

### R5 次要观察保留，不混同已确认缺陷

**C3：Caution 的解释有版本边界。** 父逐一读取固定 3.3 的九个 caution 内容，并经外部仓库研究后直接读取 [EPUB 自定义标题脚本](https://github.com/w3c/epub-specs/blob/43a28160b7d4014a115db93a18d848b24588cd88/epub33/common/js/add-caution-hd.js)：它仅加 ID／标题／ARIA heading，不把 caution 变成 note 或 informative。后来的 [2026-03-27 EPUB 3.4 变更](https://github.com/w3c/epub-specs/commit/176ee09517386fde30ad67b5d4902262d61764a4) 才显式加入 “All caution boxes … are also non-normative”；父实际下载并核对相关 authoring／RS hunks。不能把后续 3.4 文本倒写成 2026-01-13 的明文规则，或称 ReSpec 自动赋予这种语义。本批保留说明性警示及边界观察，不因 Caution 标题或小写 should 新造 MUST，也不泛化修改 `non_normative` 来绕过真实约束。

**M1：摘录上下文与批准输入是两件事。** 算法 li[1] 的来源摘录包含 Explanation 上下文，确实比规范步骤宽；具名 Explanation 本身仍被排除，不能将其解释文字提升为独立义务。父进度记录不纳入其自身受审输入是既定设计：规范、矩阵、代码和契约已绑定，实际父／Droid 决定另由 report hash、原始 completion 与输入身份校验。本记录的追加不是批准或认证机制，不为此扩大或放松 gate。本批不改旧来源摘录或悄悄移除这些观察。

四次原始 isError 保留：提前查看未完成流程的输出、零匹配 grep、许可资料缺失时探针 `None.replace` 抛出 `AttributeError` 等；不是产品测试失败。早期 hash 探针误判和已撤回的 C2 定义遗漏也保留。只有来源／验证最小修复已授权，**修复尚未验收，R6 未启动，S0 未批准，T1 暂停，未 push／发布**。

### R5 F1／C5 修复已独立复验并集成本地，仍待新完整审查

父实际核对修复包 **15,274 bytes**、SHA-256 `ed1d066565810d56d0fa28235c1d59fe665e2cd7be548183ef4846bd30ae0172`，前置 `4bb5cc4`，verify／fetch 成功。在原隔离 worktree 固定 `58a0247c6f4436206829a2a4d326ab032afbead4`，阅读全部增量、八个原文来源及 contributor／creator、date、subject、RS CSS 上下文。修复保留继承限定、条件和建议等级，不把可选 role、DRM 或 CSS 渲染变成本轮必做功能。

- 父首次完整三 review／八 amendment、两个 index、四 verify、official reproduce 均成功；tracked／staged diff 均无漂移。完整 Python **121/121 PASS，575.903s**，组合实际 **exit 0／`S0_PARENT_R5_FIXED_CHECKS_PASS`**。正常 gate 仍实际 **exit 1：independent acceptance records missing**，不是实际批准。父完整日志 SHA-256 `d1eccb2456c774e86e81d5915f78700e7c315c161f12b91dad7135541a2ac98f`。
- 原父反例在仓库内逐字保留，SHA 与原 `682ab958…502df608b` 相同。逐身份／逐字段比较确认 **1598 旧 rows、849 旧 manual 全不变**；新增八条后为 **1606＝1543 mapped＋63 excluded，857 manual**。500 section 身份／状态保留，仅八条 notes、其中七条 reviewer 变化；其余顶层字段未变，五维全 `not-tested`。118 原资产、官方及 upstream 工件、产品 Go、依赖和三主文档未改。父矩阵说明更新为 53 个有限来源家族及八 amendment 重建命令，不把这些计数当作完整性证明。
- 子完整原日志经父读取核对：同一 `58a0247` 上 **121/121 PASS，594.080s**；首次 replay、四 verify、reproduce 及前后 diff 均 exit 0，gate 预期 exit 1，HEAD／tree 前后不变。证据包 **10,107 bytes**、SHA-256 `c503294c86725b679667f9e2bb429e2e671b99f2531fc3597be8363995dbd57d`。父／子执行结果分开，不相互冒充。

父检查发现新增原子性测试的首 packet 已导入、无净变化，因此不足以捕获“先写首包、再因坏包失败”。没有观察到生产导入器实际部分写入；这是回归测试不足。后续仅测试提交 `bfc93e7ca109a84682d356773fce2678bdab9077` 令临时首包实质改变 contributor note，先以单包正控证明只改预期字段，再恢复基线并验证首包＋错误 hash 第二包不会写入任何字节。bundle **1,066 bytes**、SHA-256 `4fca961e74ca1c493e686bfe8a2684d5fb7af490d74c368a0b2eb4fa8a96b8d7`，父 verify／fetch 并读 diff；只有一个测试文件变化。

新固定 test-only 树父定向 **6/6 PASS，37.493s**，无 tracked／staged 漂移；日志 SHA-256 `244b5951da34eafc7f052fe815581ae165002f475726379dd5f454993110b94a`。子定向 **6/6 PASS，38.515s**，故意先写首包的内存 mutant 被检出为 **1 failure／0 errors，18.870s**；其 harness exit 0 表示预期失败已观察到，不把 mutant 说成通过。证据包 **2,062 bytes**、SHA-256 `1da525182e0a0293d8b942b06e814393baed06759c025e744d1fe78fbbaaa8ce`。旧 121 项组合不冒称覆盖这一后加测试修改。

父本地集成为 `a0935ce`／`ce7d659`；脚本、资产、产品及依赖与最后受验子树逐项相同。保留子先前误用 paragraph 的 list-only contextDOM 失败、选中原语句充当 sibling 的测试错误以及全部原红结果，详见 [本批记录](S0_REVIEW_R5_FIXES.md)。C3／M1 的版本和解释边界未改，没有放宽门禁、改写原始文档、运行新 Go 大组合或恢复 T1。修复本身关闭这些已确认问题，但 **S0 未批准，R6 尚未启动，未 push／发布**。

### 最终组合复验完成；新完整 R6 已真实启动，尚无决定

父本地固定 `26b32fbfa85605d621046710bdad3d9ef95d31b9` 上完整 Python **121/121 PASS，589.402s，actual exit 0**，这次明确包含后加原子性测试和最终矩阵说明。日志 SHA-256 `ea75592a4898ffcbe13ba365d606bcf3c2a278185166bcb8d77a17cd340b7c02`。父另以独立内存 mutant 模拟先写首包：加强测试只在最终字节比较处出现 **1 failure／0 errors，18.850s**；harness exit 0 证明预期错误被检出，不是生产测试失败，日志 SHA-256 `8f1e0208db22f8737d86fdccc3905e840cc09e456c477f4a2918746a90e6c59d`。测试后无 tracked／staged 漂移，全部批准输入逐项不变。

R6 fixture `b4e052f590d00665a4d05e02e95f7033bc1bb7f2` 的完整 tree **`7ce11e0150fe3ed2007349ae8bf264ae07a71eaf`** 与上述父本地提交相同，不是 `origin/main`。实际传输 bundle **37,713 bytes**、SHA-256 `e8d89463856dc37b1f08e3e9460502bf8e0abc912e0d870b2188436594bb004b`，前置 `bfc93e7`；输入包 **106,741 bytes**、SHA-256 `b30acb424a844df89bdc23b9b793e930c18f8c7a82eadef0cbd1aec9f3dcd4d2`。双方逐字节核验 **836 个输入**，canonical identity **`329c917fa55cea90e4c260832310a3f837789b71b1c3e5df9cf3e3723a7c2cd1`**。子使用持久、独立 `review-r6`／`scratch-r6`／`probes-r6`，保留原 T1 初稿及 R5 证据。

父全文检查 **11,281-byte prompt** 与 **3,536-byte launcher**，SHA-256 分别为 `53b83688e0537e1074079611ddc4e49f220f2ee3d4b03a9551ff08dda5f9fe3e`／`8f6a45026ccfc908a1ff516a1b95d2cfd4d25612364cf343a2237f46233e131e`；准备证据包 **92,493 bytes**、SHA-256 `e041c732cb5a0a2d3600ce7e28197495feb6ac8a32f3732af1cbd879dee29f5b`。两个目录的实际输入清单均与父完全相同，clean、无 acceptance。未授权 wrapper 的实际 **exit 64** 未创建 claim 或模型请求；此退出不是 Droid 的结果。完成检查后父单独授权唯一 R6，沿用已验证的持久 launch-once 防重机制和无 portal 的受监督服务。

实际 init 为 **1,077 bytes**、SHA-256 `00a25360de1d2306d8b96a27ca085000dfd89a42c981aaa2618ad3057bcf0c42`，确认会话 **`bfff4154-b0d3-4ba3-8f12-349bd12f135e`**、正确持久 cwd、**claude-opus-5-5／medium**；CLI 实测 **0.233.0**。要求全新完整 S0 阅读和实际 demo，不以 R5 阅读账或八条增量代替。当前只有启动／初始阅读证据，没有 completion、actual child exit 或批准；继续跟随这一会话。**S0 未批准，T1 暂停，未 push／发布。**

## R6 因 Factory 周额度耗尽中止，没有审查决定

同一会话在 **2026-10-06 09:15:08 UTC** 实际 child **exit 1**。父核对原 stream 最后两个 `error` 事件，均为 **HTTP 402／weekly Droid Core usage limit reached**；服务商提示约一天后重置，或补充 Extra Usage。两个实际 child exit 文件均为 `1\n`。这不是产品测试失败、审核 rejected、completion 或 approved；原 stream 只有一个 init、**390 个事件、零 completion／finalText**。父未充值、改账号设置、换模型、续接或重试。

原始完整 stream **2,363,109 bytes**、SHA-256 `8cdd93dd2e17459d46ed8595a2199686251d74ada162d20ee878364c863b2a59`；父确认初始 stream 是其精确前缀。中止说明 **3,854 bytes**、SHA-256 `8c300ea033b0760383b702ca5ff774b5373bf0b66608f2040bedba4f5e6d8101` 由协调者写成，只是生命周期记录，不能冒充 Droid 原最终报告。完整证据包 **966,898 bytes**、SHA-256 `6dcdb4789baad160bff2c6827e91955ba5271f406f806d6a5edd1b5f93195d70`，包括 raw stream、服务日志、真实退出、before／after、抽取脚本／阅读账和 cache；父读取并核对 archive 中的 stream 与单独文件相同。

服务 supervisor 于 09:15:10 重启 wrapper，但已有 claim，实际只输出 `DUPLICATE_SUPPRESSED` 并停车；09:16:31 已停止服务。没有第二次模型请求。父稍后的进度文字仍根据最后阅读调用称“继续审查”，未及时识别同一快照末尾的终止错误；此处按原始事件和服务时间纠正，不以协调线程仍活动或 guard 停车误称 Droid 仍运行。

- **实际未完成范围：**阅读账记录了 EPUB 19,287 行、RS 5,738 行、A11y 2,517 行抽取结果及截断后的重读；父保留原阅读记录，不据它宣称完整 S0 验收。额度终止时仍在核对 amendments，资产／index／upstream／official 来源审查没有终局结论。R6 没有执行完整离线 replay、四 verify／reproduce demo、Python 全套、真实 EPUBCheck witnesses 或合成门禁对照；不能把此前各轮或父侧通过结果移填到 R6。
- **固定内容与目录状态分开：**after-state **107,024 bytes**、SHA-256 `14c17609ca06b15222a0b6a7ccfe1c66b6d399b4306c4d967643c1f817a872f2`。父逐项比对其 836 输入、HEAD／tree／identity 与原包完全相同，无 acceptance。但 frozen 工作树并非 clean：抽取助手未用 `-B`，生成未跟踪的 `scripts/__pycache__/epub33_assets.cpython-311.pyc`，**112,441 bytes**、SHA-256 `1980f7bc9789508eb75c0cc2cdfd0e816181517e03a0b815503962cf15d62312`；不在 836 输入内。原严格 after-status 检查因此失败，后续独立字节检查区分了这两件事；cache 原样保留归档，不删除后冒称初始检查通过。
- **工具错误保留：**两次 `tool_result.isError` 是提前读取尚不存在的结束文件，以及临时 REC 抽取器对字符串调用 `walk`；后者修正后重跑。它们与末尾两个 quota `error` 事件分开统计，不当作产品缺陷或成功执行。

本地修复、测试和证据已保存，但 **S0 仍未批准，T1 暂停，未 push／发布**。当前阻塞需要 Factory 额度恢复；没有自动重试、自动充值或定时续跑。恢复后须单独启动新的完整审查，不将这次中止改写为通过。

## DeepSeek D1 已结束；父核验报告更正，新增协议另待 D2

用户更换审计者后，父新建 [DeepSeek D1 Orb](https://ampcode.com/threads/T-01a1109b-33dc-747c-9e0b-b29aef93f67f)。受审 HEAD `45473b4a5e869f325aab19bb35d833550813d8b7`、tree `a81a3d68ac6c3f45a10fbd4c2fb6875f6180442d`、836 输入 identity `329c917fa55cea90e4c260832310a3f837789b71b1c3e5df9cf3e3723a7c2cd1`。实际导出中的 agent definition 与全部 assistant usage 均为 `deepseek-v4.1-flash`，最终消息确为 `complete/end_turn`，决定为 `approved / complete-S0 / findings=[]`；不是仅按线程标题认定模型或按 idle 推定完成。该决定仅绑定旧树，不是当前适配组合的父批准。

父下载原证据包 **24,116 bytes**，SHA-256 `58aad7b24980f51bce42951e4aeb88ae5b25e8653f4077e0ce2d6b53b365b74d`；原技术报告 SHA-256 `66be1d051dbadaf5bc80c9d600f259d8d5a4195a5d182060a60998aed66cf62c`。实际 final 文本 SHA-256 `6bbab24969d799f4492a54cce186f60b56cc409c7f48f5f21924df249e48f47e`、私有原始导出 SHA-256 `c3f3104cc674221a2ec23e66943fc333481b6486fecb57b1d82e838ae318c7f5`。执行证据保留在 `.agents/`，不公开内部推理内容。独立 Python **121/121 PASS，643.895s**，真实固定 EPUBCheck 的 `TestRealREC2026` **exit 0，67.280s**；首次重放、四 verify、reproduce 通过且零漂移，正常 gate 缺批准仍 exit 1。另有 14 份固定来源重取、3 组官方用例源码对照和明确抽样范围，不冒称阅读系统执行、渲染、全部 1606 行人工重推或后续 CLI 功能通过。

父读取原报告、阅读账、控制结果，并逐条核对真实导出的可见 Read 行与固定源。四生产脚本、21 份测试及已读治理文档的可见范围吻合，但确认下列更正，原报告和包均未改写：

- 原“全文、无未读”遗漏 `epub.txt` **14242–14287** 的 46 行非规范 Change log，以及第 19187 行末尾空白。D1 确认无其他读取证据，于 **10:31:04 UTC** 补读并核对字节；不能将事后补读伪称为原终局前已覆盖。
- 原“13/13 均拒绝”应为 **13 控制满足各自预期：11 拒绝反例、2 通过正控**。
- 原“R1–R5 均 rejected”错误；R4 是中断、无决定。R1 结束未批准，R2／R3／R5 rejected，R6 额度中止无决定。

独立更正记录 **4,140 bytes**，SHA-256 `99b5786b1cb7ec1cb8c07a138b90f21dc2822243173636bd9cf5cfcc8777f03a`，已由父下载全文核对。更正没有改写旧 final、决定或资产，亦不批准新组合。D1 探针的 tarball 前缀错误、抽样初筛假阳性与未落盘后重跑等失败仍保留。

### Amp 协议适配已固定并通过父复验，不等于独立批准

父仅在审批入口增加用户授权的 `parent + amp` 证据路径，保留 `parent + droid` 和全部来源／派生／语义校验。Amp 路径核对导出 hash、真实线程、实际模型 usage、最终完成状态和报告全文一致性；两份报告仍须批准同一 `complete-S0` 输入且无未解决 finding，独立报告必须声明完成阅读。没有创建真实 `acceptance.json`，也未翻转 `semanticComplete`。导出是执行记录一致性证据，不是提供商签名；阅读真实性仍须父核对。

旧入口的 Amp 正控先实际失败；定向 **10/10** 随后通过。初稿将 export 的 `v=5` 误当固定格式版本，真实执行导出变成 `589`，父真实对照检出了这个错误，已移除该错误限制并增加变值正控。初轮全套 **124 PASS，638.214s** 在修正前启动，不能充当最终树证据。最终本地固定 `487a806a7b47de108d7a910d11f61a8f57a76790` 在独立 worktree 完整 **124/124 PASS，652.007s，exit 0**；3 reviews＋8 amendments 首次重放、两 index、四 verify、official reproduce、diff 均通过，输出 `AMP_GATE_FIXED_TREE_ALL_CHECKS_PASS`。最终组合日志 SHA-256 `d558ca0dca861b0b012d53ee375fec7ef334a41bbbf706f76c0480485def45cc`。真实 D1 final 导出通过执行一致性正控，真实未完成导出在 completion 检查被拒绝；两者均不替代当前输入批准。

新 tree `99ee292eec23f71a0fb00b09a9de7253217c869f` 的 836 输入 identity 为 **`1e3055f507824e62e58e51380b509347c8bc22bc9413a30a27c072705b0ac1eb`**。恰好两 Python／测试及四治理文档变化；原始 archive、全部产品 Go 与测试字节不变。父进度记录不属于批准输入，新增记录不改这项身份。当前改动仅本地提交，未 push／发布。

用户随后直接在 D1 线程要求新建 DeepSeek 审计 Orb、只替换审计模型而不改开发流程。D1 确认没有自行创建，父据此创建 [独立完整 D2](https://ampcode.com/threads/T-01a110c5-dd31-748d-83f3-1c398f01c097)，实际指定 `deepseek-v4.1-flash`。交付完整基线 bundle **62,147,462 bytes**、SHA-256 `c747ebcb02e724a7e9d9626a4f56711058f1069bc4ae89fadcef1c8fa1a9f58e` 与输入表 **106,855 bytes**、SHA-256 `33fc87135291d39046ac77e52db6441a7d809bada867e21be19495e9abf9cd12`，要求独立完整阅读和实际检查，不继承 D1 的阅读声明或旧树批准。当前仅创建／准备状态，无 D2 完成、批准或 T1 放行结论；继续跟随该线程，不启动重复审计。

## D2 完整审计及补读已完成；父批准固定 S0 输入

2026-10-06，D2 在上述同一固定树实际完成，父从原始 Amp 导出确认实际 agent definition、全部 assistant usage 为 **DeepSeek V4.1 Flash**，最后消息为 **complete/end_turn**，批准报告逐字等于实际 final text。首次最终文本 SHA-256 `5f66653b3fd86b6f8ba494873be912aba6f49efbbcbc638096ca38772d12a9a2`，首次私有导出 SHA-256 `e90d08ffd5347b3f6af5adf7ecaf8676de0bf21100c4848b0ca41c4238ab2e74`。没有用模式标签、idle 或中间消息代替终局证据。

父下载并核对全部 12 份原证据文件，`SHA256SUMS` 全部通过，清单自身 SHA-256 `9bd23a6c5707acd25af195b1212e74a8efb4bc5f44ec7c0f66b1635cd1ae8df6`。逐一读取原日志、探针和阅读账：

- 独立完整 Python **124/124 PASS，633.795s，exit 0**；首次 3 review＋8 amendment、两 index、四 verify、official reproduce 全通过，前后 tracked diff 为零。正常无收据 gate 仍因缺批准 exit 1。父在同输入固定树的 **124/124、652.007s** 及完整重放结果见前节，双方执行分开记录。
- 真实固定 EPUBCheck **2 个顶层测试＋12 个子测试，54.709s，exit 0**；子测试为 6 个合法正例与 6 个预期非法负例，Java 后端确为 5.3.0。原报告把含顶层测试的 14 条 PASS 称为 14 子例，父按日志更正，不增加虚构测试。
- **21 项控制符合预期**：2 个合成批准正控、18 个针对性拒绝反例及 1 个无真实收据检查。检查实际报告／导出／输入 hash、模型、终局、父批准与独立批准，不把合成正控当真实验收。
- 118 资产、1606 行（1543 mapped＋63 有理由 excluded）、857 手工来源、500 节记录、169 官方用例／170 报告行／170 本书均有原始执行核对；五维仍全 `not-tested`，官方 `executed:false`。上游 14 项目的一个既有许可缺口与 T3 八类待归档资产保留，不虚称已采用或已取得。

父按实际 Read 返回行号及与固定源逐字相同的 shell 输出核对阅读并集：三份 REC **19187／5866／2432 行**、四生产脚本与 21 份 Python 测试无缺段。首次治理阅读确实缺开发计划 **1–790 行**；该部分包含当前范围、五维能力及上游采用边界，不能整体排除为旧产品历史。父未接受首次过宽的 `readingComplete:true`，要求同一 D2 在相同输入补读并重新给出完整终局；没有改实现或重新启动审计 Orb。

D2 随后补读 1–790，父核对实际返回后确认计划 **1065/1065 行**覆盖，并读取其范围／结论核对。补读不改变 complete-S0 实质结论，新的实际最终决定仍为 **approved、findings=[]、readingComplete=true**。原报告／日志保持原字节，补充账和日志单独保存；补充清单 SHA-256 `a022d507a4c6525db14b13dc39107bbe9aebb261c6cb8c9e497a724ac177db49` 全通过。更正后的[完整独立报告](S0_DEEPSEEK_D2.md) SHA-256 **`c9fb367360006bfe47a51787e03ab5ca92d7468cbb04693b38e8723d53c218cc`**；对应私有实际导出 SHA-256 **`9d62784999ae0f8969e1a432ff6ed7804f8d3445aecb3c5ae45056afb0ca1959`**，再次通过真实终局一致性校验。

父综合既有独立来源复验、反例／修复、最终组合与本轮完整审计，**批准 complete-S0**，仅绑定 **836 输入 identity `1e3055f507824e62e58e51380b509347c8bc22bc9413a30a27c072705b0ac1eb`**。父另用独立重构脚本核对本地输入表逐项相同。下列为本记录唯一机器决定；此前 pending／拒绝／中断历史不改写，不借更新进度修改受审输入。

```json
{"scope":"complete-S0","decision":"approved","findings":[],"inputIdentitySHA256":"1e3055f507824e62e58e51380b509347c8bc22bc9413a30a27c072705b0ac1eb"}
```

真实收据已按两份实际报告及私有导出 hash 生成，`python3 -B scripts/epub33_assets.py gate` 实际 **exit 0**，输出 `archiveFiles:118` 与 `mapping:{candidates:1606,excluded:63,mapped:1543,semanticComplete:true}`。这里的 true 是聚合检查结果；没有编辑矩阵中的 `semanticComplete:false` 或五维状态来通过。收据与私有导出不提交，完整批准基线及所需证据须实际传给后续编码 Orb，在无初稿污染的固定 checkout 重验后恢复 T1a；不能只传提交名或复用旧树批准。

原始导出含私有线程内容，只在授权 Orb 内保存／传输，不公开；收据不属于其自身批准输入。本地可读批准记录与独立最终报告纳入版本控制，GitHub 上的源码尚未包含这些新工作。此批准是固定基线的 S0 退出，不是 T1a／T1b 完成、169 个阅读系统用例已执行、渲染／Mac／人工无障碍通过或 Issue #3／#4 关闭。后续输入变化不能复用旧批准；独立审查继续使用用户指定的 DeepSeek，Droid 不自动重试或监控额度。当前仍无 push／发布。
