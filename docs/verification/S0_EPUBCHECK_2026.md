# S0：EPUBCheck 5.3.0 对 2026 REC 三项增量的核验

2026-10-05，Linux amd64 Orb。对应开发计划 §11.8 的 S0 第 5 项；这是规则来源与实际 JAR 回归证据，不是 S0 整体完成、整个 EPUB 3.3 覆盖、阅读系统渲染或无障碍认证。

## 结论与身份

12 个完整 EPUB 样本均得到预期的正式校验结果：合法 6 个 backend exit 0，非法 6 个 backend exit 1，并核对非法样本的上游诊断与资源位置。普通、race、vet 通过，无 skip。未修改产品实现、正式门槛、checker 版本或旧机器 schema。

- 目标规范：[EPUB 3.3 REC 2026-01-13](https://www.w3.org/TR/2026/REC-epub-33-20260113/)。核验用原 HTML SHA-256：`f927cf3182598c552037445837b73faee346f97d4a396059440830e49e737d99`；正式离线归档由 S0 资产清单另行交付。
- 规则来源：[EPUBCheck v5.3.0](https://github.com/w3c/epubcheck/tree/029831b8f477e4519e9734c984ee24357547a698)。不是移动 main，也不升级到 3.4。
- Go `1.27.1 linux/amd64`；OpenJDK `17.0.20.1`。
- 官方发行 ZIP SHA-256：`6c07e68584b2e2ce2f89fe06e1246dfead3eb36b46b340e7d93524f29dcff6c5`，安装来源与完整性规则见 [setup](../../.agents/setup)。本轮复用已安装工具，未重新下载 ZIP。
- 实测主 JAR SHA-256：`f7f96617c929371821609b88c8484d6dc9f24fe916499863c46094c5fb778a65`。
- 安装完整文件清单 `SHA256SUMS` SHA-256：`2a5456d59b1a2aebedea1a58afc98fb36546d74be39624c3f79ae624fe34a835`，40 个 JAR（主文件和 39 个依赖）。
- 每次校验重新计算的完整工具集合 fingerprint：`158b7c2778c3b64d5dbd87151b7d1f870b03beda3bee973d52b3fe58bea16266`。算法为对排序后的 `relative/path:sha256\n` 再算 SHA-256；与安装清单文件的 hash 是两个不同值。
- 产品基线为已冻结 v0.10 计划、未修改的旧产品；本增量仅新增 [rec2026_test.go](../../internal/validation/rec2026_test.go)。测试源码 SHA-256：`f2cc9181677de4e0c29dc7b3be8ce7a6ad3cd3198dc60d97737ef10df940df7a`。

`targetSpec` 可以记上述 REC URI；checker 的完整已证 `specBaseline` 仍为 `unknown`。运行报告 `rules="3.3"` 不证明该 checker 全部规则精确覆盖这份日期固定的 REC，也不代表 EPUB 包的 `version` 改成了 3.3。本记录仅把以下三个改动关联到已核查的规则与样本，T2 字段冻结时必须保留这个区别。

## 固定规范与规则来源

[REC change log](https://www.w3.org/TR/2026/REC-epub-33-20260113/#changes-from-rec-20230523) 列出这三项实质变化。

| 变化 | 规范依据与检查器来源 | 实测覆盖与边界 |
|---|---|---|
| #2637：viewport 空白从 Infra 定义改为 XML 空白 | [REC 附录 F.2](https://www.w3.org/TR/2026/REC-epub-33-20260113/#app-viewport-meta-syntax)；[规范变更](https://github.com/w3c/epub-specs/commit/bebfc620c6dcec112a54b2c5070df33f93f2215a)；固定 checker 的 [ViewportMeta](https://github.com/w3c/epubcheck/blob/029831b8f477e4519e9734c984ee24357547a698/src/main/java/org/w3c/epubcheck/util/microsyntax/ViewportMeta.java) 与 [OPSHandler30.processMeta](https://github.com/w3c/epubcheck/blob/029831b8f477e4519e9734c984ee24357547a698/src/main/java/com/adobe/epubcheck/ops/OPSHandler30.java) | 固定版式 spine XHTML 的首个 viewport：SPACE/TAB/LF/CR 通过，FF 被 XML 层拒绝，NBSP 不作为分隔空白。解析函数仍使用 Infra ASCII whitespace（包含 FF），但不能据函数单独行为声称完整 EPUB 错误接受 FF：XML 1.0 已先拒绝。没有穷举整个 viewport 文法。 |
| #2556：SVG `epub:type` 按 renderable elements 限制 | [REC §6.2.3](https://www.w3.org/TR/2026/REC-epub-33-20260113/#confreq-svg-structural-semantics)；[checker schema 变更](https://github.com/w3c/epubcheck/commit/6cd9789ca94afd0ec408f94a8f32c9fe33b0c320)（早于规范采纳，不能当规范提交）；固定 checker 的 [epub-svg-forgiving-inc.rnc](https://github.com/w3c/epubcheck/blob/029831b8f477e4519e9734c984ee24357547a698/src/main/resources/com/adobe/epubcheck/schema/30/mod/epub-svg-forgiving-inc.rnc) | 独立 SVG：`a` 允许，`defs` 与 `title` 拒绝。schema 的允许列表包含 `a`，不含 `defs`；`title` 明确使用禁止属性集合。未穷举所有 SVG 元素。 |
| #2555：同一限制适用于所有 SVG 定义，包括内嵌 SVG | [REC §6.1.4.2](https://www.w3.org/TR/2026/REC-epub-33-20260113/#sec-xhtml-svg-inclusion)、[§6.2.3](https://www.w3.org/TR/2026/REC-epub-33-20260113/#sec-svg-restrictions)；[规范变更](https://github.com/w3c/epub-specs/commit/8478066101bfb595eaa6a6f7f5ceb6bc5420326c)；上述固定 schema | XHTML 内嵌 SVG 的同组三个正反例，与独立 SVG 得到一致结果。manifest 明确 `properties="svg"`，不以缺少声明引起的无关错误充当本条规则证据。 |

预期的合法性来自规范；诊断编号来自固定上游规则，不把“任意失败”视为目标规则通过。SVG 锚点有真实目标，固定版式样本明确 `rendition:layout=pre-paginated`。TAB/LF/CR/FF/NBSP 使用字符引用，避免 XML 的字面属性空白归一化把被测字符变成普通空格。

## 实际用例与归档绑定

测试复用 `integration_test.go` 的合规出版物生成器，而非故意含无效导航的通用 read-query fixture。每例经 `validation.Validate` 冻结完整 EPUB、运行官方 JAR；检查实际 backend exit、版本、完整工具 fingerprint、原始 JSON、归档与 tree 身份，并再次核对源 ZIP SHA 不变。

| 用例（测试子名） | backend exit／上游诊断 | 实际输入 EPUB SHA-256 |
|---|---|---|
| viewport/space | 0 | `071601019c0f2e95aefbeb17ef38c0a2368727e0b0ff762c627060ef343e878c` |
| viewport/tab | 0 | `ac3d947aa9b4fc21e9f3d8f6ef6c3c2a88f87edbca942af17a936d8633348bf7` |
| viewport/lf | 0 | `c89baed72086f333dec2489128096d4faaf970d7f5e1571bc6e5068972ebc036` |
| viewport/cr | 0 | `19b4651ab0f15d81fdf961a8347af93e3ad45763bb647a7515a0187273be9b33` |
| viewport/form-feed-not-xml | 1／`RSC-016` fatal | `53fca64be7099e435934f9a9dcbda802a864e9e5bf4e20a33ba959c23f65fb37` |
| viewport/nbsp-not-xml-space | 1／`HTM_056` error | `eefda2fa71793df0927fafb19a2bbc9305d15c485df6baf3343a74843f4aae77` |
| SVG/inline=false/a-allowed | 0 | `a3939116eb1606e44c88852f55c72269f92b097380b653584d28a8d0912426eb` |
| SVG/inline=false/defs-forbidden | 1／`RSC-005` error | `4eb7620cb7a563c16ba6243f9f1ebd515abfb37913cc8f489561d05336173052` |
| SVG/inline=false/title-forbidden | 1／`RSC-005` error | `97052105a70381653e141c1d43f96dc7b69bfada14224d5f06858e1e7fa0438c` |
| SVG/inline=true/a-allowed | 0 | `d3d23ee3a144b3927328762c632e8fe892edc22a7d66dd56e4588c999868c281` |
| SVG/inline=true/defs-forbidden | 1／`RSC-005` error | `c181a8a98f41e73d56d589fdd7106060f53492eafae61977ceae13ddb62cda9f` |
| SVG/inline=true/title-forbidden | 1／`RSC-005` error | `ec5177f7cfc1831847eb3957173e39f4100c531c81b93ba75a2253c1e5bfcb1e` |

viewport 与内嵌 SVG 的诊断位置为 `EPUB/chapter.xhtml`，独立 SVG 为 `EPUB/diagram.svg`。这是默认非 strict 校验；公共 fixture 自带空目录，产生 `PKG-014` warning，不把 default pass 说成零 warning 或 strict pass。没有验证渲染结果。

## 可重复命令、失败与最终结果

先运行仓库 `.agents/setup`，核验 Java 与完整 checker 后执行：

```sh
test -n "$KEPUB_EPUBCHECK_JAR"
go test -count=1 -v ./internal/validation -run '^TestRealREC2026'
go test -race -count=1 -v ./internal/validation -run '^TestRealREC2026'
go vet ./internal/validation
```

未配置 checker 时测试按照仓库现有模式 skip；skip 不满足本记录的门槛，不能作为 S0 核验通过。普通/race 均实际启动官方 Java 后端，不以 mock 代替。

| 运行 | 结果 |
|---|---|
| 初次普通 | FAIL，51.843s：12 例中 11 例通过，NBSP 已被正确拒绝，但测试误预期 `HTM-047`；实际是两个 `HTM_056`（缺 width、height）。核对固定 `ViewportMeta` 与 `OPSHandler30` 后确认 NBSP 保留在属性名中，未产生语法解析错误，故进入 missing-dimension 分支。仅修正具体诊断断言，非法预期、fixture、产品均不变。 |
| 最终普通 | PASS，51.495s，12/12，无 skip |
| 最终 race | PASS，55.705s，12/12，无 skip |
| vet | exit 0，无诊断 |

定向组合进程实际 exit 0，最后输出 `S0_REC2026_NORMAL_RACE_VET_PASS`。另独立比对普通／race 两份日志：12 例 backend 结果、归档 hash、tree hash 逐项一致，记录中的 12 个归档 hash 均吻合，无 skip。

随后在已提交的相同测试与产品树上执行全仓组合，原进程实际 exit 0，输出 `S0_CHECKER_ROOT_NORMAL_RACE_VET_DARWIN_PASS`：

| 命令 | 实际结果 |
|---|---|
| `go test -count=1 ./...` | 全部 PASS；CLI 125.112s、workspace 155.776s、validation 233.693s |
| `go test -race -count=1 ./...` | 全部 PASS；CLI 124.026s、workspace 191.144s、validation 248.511s |
| `go vet ./...` | exit 0，无诊断 |
| `GOOS=darwin GOARCH=arm64 go build ./...` | exit 0 |
| `GOOS=darwin GOARCH=arm64 go test -c -o /tmp/kepub-s0-validation-darwin.test ./internal/validation` | exit 0；`file` 确认为 Mach-O arm64；未在 Mac 运行 |

本记录不替代整个 S0 的归档／矩阵／测试来源验收；完整 S0 资产集成后仍须其离线检查和真实 Droid 审查，不能据本轮结果恢复 T1。

临时原始日志保存在父 Orb `/tmp/kepub-s0-rec2026-{normal,normal-final,race}.log`；不是已发布资产，持久证据是本记录的结果、输入 hash 和可重建测试。对应日志 SHA-256 依次为：

```text
1d2da39947ef6a86be82f99e9bc8fed2bb3e3bbe348e6cc333815b9950c236fc
3b0ef06c38c24ea63abdb68c3d35c774112ba2407dfcce60bad2d072d4e09039
2fad533bdbeb12a4ae7454fce3c74b84a36785c9531ccbc688b82b4d2e7b1d3a
```
