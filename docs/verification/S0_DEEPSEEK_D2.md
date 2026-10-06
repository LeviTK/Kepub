# D2 独立 complete-S0 审计实质报告（补充更正后的完整终局，Kepub）

## 一、结论

按开发计划 §11.8 的 **complete-S0** 范围，在同一固定树（HEAD `487a806a7b47de108d7a910d11f61a8f57a76790` / tree `99ee292eec23f71a0fb00b09a9de7253217c869f`）上完成全部治理文档阅读、离线重放、真实 checker 执行与自建门禁反例后，未发现未解决的实质问题：**approved**，findings 为空，readingComplete true。父指出的两处问题已如实更正（见 §二、§六）。本线程未创建真实批准 receipt、未调用 Droid、未改受审实现/输入/旧证据，未推送/发布，T1 未解锁。

## 二、此前 readingComplete 的范围偏差与本次补读依据（更正）

- **偏差**：首次终局声明 `readingComplete:true`，但首次 `reading_ledger.md` 同时如实记录 `docs/DEVELOPMENT_PLAN.md` **只读了 791–1065（§11–§13），1–790 未读**。即：账本本身准确，但终局“完整治理文档”的覆盖声明比账本记录更宽，属于**终局措辞的范围偏差**。此前委托要求完整治理文档，该缺口属实。
- **补读**（同一固定树，未改任何受审字节）：`docs/DEVELOPMENT_PLAN.md` 全文缺失行 1–790 分三块读回——1–400（400 行/55,832 bytes）、401–600（200 行/22,488 bytes）、601–790（190 行/31,784 bytes），每块工具返回均无 `[... omitted ...]` 省略标记；目标文件 SHA-256 `cffc3621c3910dd4ea4e6cb183735230eaef467c1ca6d3e4bdbbb13f820b207e`（1065 行/163,281 bytes）。加上首次已读的 791–1065，计划全文 1–1065 覆盖完毕。
- **补读是否改变结论**：不改变。核对要点：① complete-S0 的退出条件仍由已读的 §11.8 定义（4 份固定基线＋7 份配套＋errata/实现报告/测试入口＋XML/Namespaces/HTML-XML 章节；直接引用去重清单；三份 REC 逐条矩阵；官方用例报告提交与源码提交分别固定；EPUBCheck 5.3.0 对 2026 增量核对），补读未新增或删除任何 S0 验收条件；② §7.4 的五维字段与状态集（`preserve/parse/edit/render/validate`、`supported|partial|unsupported|policy-disabled|not-tested`）与树内 `matrix.json` 完全一致，8030 个能力单元全部 `not-tested`，符合 S0“登记完整≠实现支持”；③ §7.4/§11.8 的 BCP14 完整关键字集与最小可独立判定约束的清点单位，与矩阵 1606 行＝1543 mapped＋63 有理由 excluded、857 条手工来源一致；§8.3“研究登记≠采用、许可缺口阻止采用”与上游 14 项目/1 缺口一致；④ §3/§6/§10 等是 T1～T6 与 S2–S4 的未来阶段要求与安全边界，明示“资产已建立不等于门槛通过”，不构成对 S0 数据的反证。故 complete-S0 实质结论不变。
- 补读校验日志与补充账见 §七；首次终局与证据包原字节全部保留（其 `SHA256SUMS` 仍全部 OK，自身 SHA-256 `9bd23a6c…` 不变）。

## 三、固定输入与身份（前后不变）

| 项目 | 值 |
| --- | --- |
| bundle / inputs | `c747ebcb02e724a7e9d9626a4f56711058f1069bc4ae89fadcef1c8fa1a9f58e` / `33fc87135291d39046ac77e52db6441a7d809bada867e21be19495e9abf9cd12` |
| HEAD / tree | `487a806a…` / `99ee292e…`（审计前后两树 `git status` 空，无 `acceptance.json`） |
| 输入身份 | 836 项逐项匹配，canonical identity `1e3055f507824e62e58e51380b509347c8bc22bc9413a30a27c072705b0ac1eb` |

## 四、执行证据（首次原日志保留，未重跑耗时项）

1. 完整 Python：`Ran 124 tests in 633.795s, OK, exit 0`（日志 SHA-256 `5a2eccec…`）。
2. 按矩阵文档首次 3 reviews + 8 amendments 重放＋两 index＋四 verify＋reproduce 全 exit 0：assets `1606=1543+63`；semantics `169/283/63`；tests `169 cases/170 reportRows/170 publications/0 gaps/executed:false`；upstreams `14/1`；`git status` 空、`git diff --exit-code 0`（零漂移）；正常 gate **exit 1：independent acceptance records missing**（预期拒绝）。
3. 真实固定 EPUBCheck 5.3.0 `TestRealREC2026`：**PASS, ok 54.709s, exit 0**，真实 Java 后端 version=5.3.0/rules=3.3；JAR SHA-256 `f7f96617…`、`SHA256SUMS` 40 项 OK。
   **措辞更正**：日志结构为 **2 个顶层测试（`TestRealREC2026Viewport`、`TestRealREC2026SVGType`）＋12 个子测试**——**6 个正例**（backend exit 0）与 **6 个预期负例**（backend exit 1，上游诊断 RSC-005/RSC-016/HTM_056）；14 条 `--- PASS` 含这 2 个顶层测试自身。此前“14 个子例”的说法有误。
4. 自建门禁反例 21/21 符合预期：P1/P2 合成一致批准正控通过（export v 5→589 不影响）；N01–N18 全部按目标约束拒绝（旧 D1 identity `329c917f…`、缺父批准、rejected/readingComplete=false/findings/scope、错误 agent/usage 模型、非 end_turn/最后消息非 assistant、report≠final text、export/report/输入变化、pending 父报告被重标等）；N19 固定树无真实 `acceptance.json`。未产生真实批准。
5. 数据核对：1606 行/857 manual 摘要 hash 全对/500 章节 complete/五维全 `not-tested`；169/170/170；227+4/332/336；14/1；118 资产 12,599,439 bytes；T3 8 项 pending-null；22 派生文件 hash 全对。

## 五、实际阅读账（补读后完整）

三份固定 REC 全文（含 RS B.1 完整 WebIDL）；**开发计划全文 1–1065**（本次补读 1–790）；全部 S0 治理文档（S0_PARENT_ACCEPTANCE、EPUB33_SUPPORT_MATRIX、S0_EPUBCHECK_2026、全部 S0_ASSETS_*/R1–R5 修复记录、ORB_EPUBCHECK、README、CLI 契约）；四生产脚本与 21 个测试全部逐行。大体积矩阵/索引用执行验证与逐项核数处理（未逐行通读）；M1/M2/C1–C3/AMP/DROID_REVIEW 等产品批次历史为 S0 范围外，明确未读。工具失败保留：首次 epub/rs 读取因 50k 上限被截断（缺 10,558/1,142 字符）后丢弃并分块重读；自建反例脚本首跑因自身 fixture 未绑定 PROJECT 失败（工具错误）；完整 Python 日志 SHA 曾运行中误取、终值重算。

## 六、边界与更正保留

Amp 导出是执行一致性证据而非供应商签名；未执行 169 个官方阅读系统用例、未渲染、未做 Mac 与人工无障碍（S0 范围外）。首次终局与证据包原字节未被改写；本次更正以独立补充文件承载，不改旧账/旧日志。

## 七、证据包

- 首次（原字节保留，未改）：`.agents/kepub-s0-deepseek-d2-evidence/`（12 个文件，`SHA256SUMS` 自身 SHA-256 `9bd23a6c5707acd25af195b1212e74a8efb4bc5f44ec7c0f66b1635cd1ae8df6`，`sha256sum -c` 全 OK）。
- 本次补充（独立目录/独立 hash）：`.agents/kepub-s0-deepseek-d2-supplement/`：
  - `reading_ledger_supplement.md` — SHA-256 `858ee76be4f5dd0aefd9b4fb3f2313bfc20aebda199b73969630cfb621f6bd1a`；
  - `7_plan_full_read_check.log` — SHA-256 `26282305f133b11c379857f941cfafeb34e7366c8e9c9727a866814dd12ea07a`（含计划 1–790 三块的行数/字节/块 SHA-256/省略标记=0 校验）；
  - `SHA256SUMS` 自身 SHA-256 `a022d507a4c6525db14b13dc39107bbe9aebb261c6cb8c9e497a724ac177db49`。

唯一决定 JSON：

```json
{"scope": "complete-S0", "decision": "approved", "findings": [], "readingComplete": true, "inputIdentitySHA256": "1e3055f507824e62e58e51380b509347c8bc22bc9413a30a27c072705b0ac1eb"}
```