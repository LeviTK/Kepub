# S0 父 Orb 独立验收记录

## 当前结论：阶段资产未通过总验收

首次检查于 2026-10-05；2026-10-06 已完成下述 P1／P2／P3／P4 修复复验并集成本地 `main`。计划复审已完成，S0 开发进行中；T1 仍暂停。父 Orb 的 [EPUBCheck 增量核验](S0_EPUBCHECK_2026.md) 已通过定向及全仓普通／race／vet、Darwin 交叉编译，但不替代本记录的资产与矩阵验收。

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
