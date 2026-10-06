# S0 R1：门禁、代码关联和未执行状态修复批次

这是 R1 后的第一稳定修复批次，**不是完整 S0 通过**。仅修 F1、F2 的证据校验／相关代码关联、F5；F3/F4/F6、算法来源纠错和旧版资产登记在后续数据批次整合。产品、三主文档、Go module、checker 测试和父验收记录均不修改。

## 保留真实 R1 与独立反例

实际固定审查树为 `be198c6`（父未推送 local main 的同树 review fixture）。Factory Droid 0.233.0 的实际 init 为 `claude-opus-5-5 / medium`，会话 `6b0b6a6e-c262-4267-930e-9a062aa9dd34`；94 turns、1587830 ms，真实 completion、exit 0。exit 0 不代表审核批准。

R1 原报告提出 F1–F6 及 F7／观察，必须保留其原说法而非回写。报告明确：EPUB 未覆盖块约后 189 条只粗看，Notes/CSS/tests 抽样，未读实时 Issues #3/#4；不能称已完成全部正文全文审核。1 次临时 Counter/list-key TypeError 是审查脚本错误，不是产品测试失败。原始 stream、completion、report、probe tar 和独立红日志保存在编码 Orb 工作区 `.agents/kepub-s0-droid-r1-*` 供父下载。

编码 Orb 在原固定 clone 原样再次运行 `mutants.py pos gate support exec upstream`：有效正控四类 verify 全 0／gate 1；只改 `matrix.semanticComplete=true` 后 index，gate 变 0；不存在的 `internal/does-not-exist_test.go:TestImaginary` 可伪报 supported；官方 `executed=true/PASS` 可通过；上游伪行为／采用并删除真实许可 gap 可输出 gaps 0。该脚本的进程 exit 0 仅表示探针跑完，不是这些反例通过验收。

## F1：一致性验收协议，而非签名系统

`gate` 先验证逐条／逐节矩阵和原始资产、派生字节、完整语义索引、官方来源工件、上游许可事实。`semanticComplete` 不再是放行授权。之后必须有 `docs/specs/epub-3.3/acceptance.json`；本批不创建真实批准文件。

协议 schemaVersion 1 包含 `inputs`（相对路径→实际 SHA-256）及 `reviews.parent/droid`。`inputs` 覆盖原始包全部文件／矩阵／索引／review inputs、Python 脚本和测试、Go 产品和测试、module/setup、三主文档、矩阵说明与父真实 checker 记录；批准文件自身排除以避免自引用。任何相关原文、矩阵、索引、脚本或产品字节改变使旧批准失效。canonical identity 为 UTF-8 `json.dumps(inputs, sort_keys=True, separators=(",", ":"))` 的 SHA-256。

两份 review 均须有 `scope="complete-S0"`、`decision="approved"`、`reviewer`、同一 `inputIdentitySHA256`、仓库内 `reportPath/reportSHA256`。两份实际报告都须包含唯一 JSON decision block，明确 scope／decision／findings／输入 identity；receipt 的 approved 字段不能把父 pending 报告重新标成批准。Droid review 还须有 `cliVersion="0.233.0"`、`sessionId`、`streamPath/streamSHA256`、`exitPath/exitSHA256`。validator 核实际唯一 init／completion 的会话、Opus5.5／medium、真实 exit 0，以及报告文本等于真实 completion。父批准由父登记；编码 Orb 不能替父或失败审查造批准。批准报告／stream／exit 应放在归档及被计量的资产说明文件之外（例如 `docs/verification/S0_REVIEW_*`），避免批准文件自引用。

下次完整 Droid 的实际 completion 需在报告中包含唯一 JSON decision block：

```json
{
  "scope": "complete-S0",
  "decision": "approved",
  "readingComplete": true,
  "findings": [],
  "inputIdentitySHA256": "<actual canonical current-input SHA-256>"
}
```

这只是未来无问题且全文审核完成时的协议示例，**不是当前批准**。R1 拒绝／阅读不完整、只有 completion/exit 0、手写布尔值、旧输入批准都不能满足。人工语义批准仍由审查者负责，hash 协议不证明规范解释本身正确，不实现签名／身份访问控制。

正控是明确标识的 synthetic receipt fixture，mock 仅替代三类底层 validator 以隔离协议；不是一次真实 Droid 审核或 S0 PASS。另有真实当前归档的自洽 flip/index 正反控制，确保拒绝来自缺少批准而非坏来源／坏派生。

## F2：关联已有事实，不制造 supported

`reviews/implementation.json` 给 13 行追加 `evidence/testIds/gap`，关联 mimetype raw-header／字节测试、rootfile／显式选择、nav namespace／toc 数目、实际 viewport 和 standalone/inline SVG checker 样本。Go 代码／测试引用是相关性，不是自动提升行为证据。所有五维仍 `not-tested`，没有新 supported 或 RS 执行声明。

`evidence` 为仓库内相对文件路径或 `path_test.go:TestName`，校验文件、范围、真实测试函数。`testIds` 必须有相同完整 evidence 引用。支持声明须另有 JSON passed execution record，至少绑定 featureIds、testIds 与 dimensions；代码／测试存在和 markdown 记录本身不能成为 full supported。人工审查仍要检查执行和预期可靠性，不把一个自写 JSON 当权威行为。

每条关联明确覆盖边界；例如当前 multi-rootfile 测试实际允许显式选 EPUB2/3 混合 package，这不是 same-version MUST 已实现。12 个 checker 样本只支持已记录的窄族事实，`specBaseline` 仍 unknown，未穷举 viewport／SVG。其余条款保留真实缺口。

## F5：来源登记不得暗含执行／采用

官方索引必须保持 `executed=false`、未测试结果和实际 source-inventory-match 状态；伪 PASS、伪执行和 paired fixture 执行声明被拒绝。输出读取已验证的字段，而非硬编码隐藏伪造输入。未来 RS 执行属于单独阶段／证据机制，不在此源归档索引中冒充执行。

上游必须保留 public-source-and-license-only、未测行为、deferred/not-bundled（已有 checker 单列）身份。已取得许可逐字对照真实 license API 内容／SPDX／源路径；不可取得许可的已知条目仍 adoption blocked。failures 必须等于从实际许可证据推导的缺口，删除失败列表不能变成 gaps 0。仅该后置 adapter 许可缺口不额外阻塞 S0 研究登记。

## 验证命令

```sh
python3 -B -m unittest discover -s scripts -p 'test_*.py' -v
python3 -B scripts/epub33_assets.py verify
python3 -B scripts/epub33_semantics.py verify
python3 -B scripts/epub33_tests.py verify
python3 -B scripts/epub33_upstreams.py verify
python3 -B scripts/epub33_tests.py reproduce
python3 -B scripts/epub33_assets.py gate  # 当前应 exit 1，未批准
```

完整 review import/rebuild 命令继续用 [矩阵说明](../EPUB33_SUPPORT_MATRIX.md)。稳定提交后从该固定树重放并要求 `git diff --exit-code=0`；不把第二次幂等当成第一次没有漂移。实际运行结果另记于随增量的 evidence/log，不用未跑的占位 PASS。

本批最终代码实际运行 **79/79，155.740 s，exit 0**。此前 77 项首次跑有一个失败：新增执行状态检查抢先遮盖原缺来源反例的诊断；调整为先检查必需来源证据，原测试预期未改。该首次失败（158.389 s）及其后 77/77、78/78、最终 79/79 的原日志均保留在源工作区 `.agents`。最后一次完整 review import／index、四类 verify、official reproduce 均 exit 0；gate 实际 exit 1，原因是缺少当前输入的独立批准记录，而不是伪造一次 S0 批准。当前源工件仍 107，官方 169 case／170 report rows／170 publications 未执行，上游 14 projects／一个真实许可 gap。没有再运行 Go 大组合，没有启动 R2，没有恢复 T1。
