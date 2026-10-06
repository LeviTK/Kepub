# S0 父 Orb 独立验收记录

## 当前结论：阶段资产未通过总验收

首次检查于 2026-10-05；2026-10-06 已完成下述 P1～P7 修复复验、完整语义增量的资产／来源校验和语义抽查，并集成本地 `main`。计划复审已完成；S0 实现 Droid R1 已实际结束但发现阻塞问题，父另复现列表／用例关联遗漏和 P8 算法说明范围错误，仍需修复、独立复验与新固定树审查。T1 继续暂停。父 Orb 的 [EPUBCheck 增量核验](S0_EPUBCHECK_2026.md) 已通过定向及全仓普通／race／vet、Darwin 交叉编译，但不替代本记录的资产与矩阵验收。

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
