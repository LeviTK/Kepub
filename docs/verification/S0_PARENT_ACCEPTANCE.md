# S0 父 Orb 独立验收记录

## 当前结论：阶段资产未通过总验收

首次检查于 2026-10-05；2026-10-06 已完成下述 P1／P2／P3 修复复验并集成本地 `main`。计划复审已完成，S0 开发进行中；T1 仍暂停。父 Orb 的 [EPUBCheck 增量核验](S0_EPUBCHECK_2026.md) 已通过定向及全仓普通／race／vet、Darwin 交叉编译，但不替代本记录的资产与矩阵验收。

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
