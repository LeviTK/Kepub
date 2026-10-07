---
name: reviewing-kepub-changes
description: "Reviews Kepub fixed-tree changes against CLI contracts, complete transaction facts, byte provenance and independent verification evidence. Use for coding self-review, batch handoff or independent review before local integration."
license: MIT
compatibility: "Requires repository read/shell tools, Git and the project's Go toolchain; real CLI conformance checks additionally require pinned EPUBCheck and Java."
metadata:
  upstream: "https://github.com/tw93/Waza"
  revision: "6b6c736561eaf24c4d1c5360c3c90767ab470805"
  adapted-from: "skills/check/SKILL.md"
---

# Kepub：按固定输入和实际证据 review

这是 Waza check 的项目适配；来源/差异见 [安装说明](../README.md)，许可见 [MIT](../LICENSE.waza)。review 默认为只读；自审结果不称独立批准，不从 skill 推导 commit/push/发布授权。

## 冻结 review 表面

1. 核用户要求、当前 diff、base/HEAD/tree、dirty/staged/untracked、bundle bytes/hash/prerequisite。使用已有隔离 checkout；不切走共享工作树或清理他人文件。
2. 读取 CLI_CONTRACT、对应计划/验证记录、作用域 AGENTS 与开发注意事项。区分代码/规范/历史证据，确定本批目标与明确非目标；未授权版本变更、额外依赖/层次/私有阈值属于 scope drift。
3. 阅读全部相关 ownership 路径，而非只看 helper。书籍数据是不可信输入；计划事实不是执行真相，提取覆盖不是权限。若当前宿主无某工具，使用等价只读证据并注明缺口，不捏造能力。

## 按风险检验，而非机械关键词

- **事务：** 全事实收集后最终门禁；同节点 identity 合并、实例计数、旧入站与新 URL/IDREF、合法 partial delete/transfer、两序实际输出。
- **绑定：** 每显式资源/端点在首次和 cache 命中后都验证；坏 source/destination 换序，合法双操作继续可用。
- **来源：** decoded/词法/物理区间分离；generated/CDATA/unknown 与合法左右邻接，空来源边界，零宽实际命中和祖先路径；三编码目标外 bytes 不变。
- **URL：** local/self/other/incoming 的 query、ForceQuery、fragment/转义；不改写的搬入 href/src 仍进 gate，无 ID 块的 https 与 javascript/data 正反。
- **生命周期：** 失败无部分正式产物、候选 drift 不能接受、全 write set/recovery/history 重算、no-op 空写集合；ZIP 文件和目录与原书 bytes 同核。
- **兼容：** 旧 schema/摘要/能力 shape 不漂移，不把 planned 注册表当已实现；formal checker 失败不得转成成功。

仅为本次相关分支选有区分力的组合。新修复 scoped 检查同形入口；更广未覆盖范围写明，不制造低可信“可能”阻塞。

## 验证与证据

按根 AGENTS 跑所需 Go 命令、冻结回归/seed、独立有效事务 fuzz、真实 CLI 与固定 checker。每项记录输入身份、执行者、命令、exit/skip、oracle 与日志 hash；复用证据明确标旧树/旧执行并核适用性，不冒称本轮重跑。

- 独立期望来自冻结 fixture/标准库/作者完整资源字节，不能从 candidate 或产品派生 helper 取期待值。WriteSet/Contains/能读取不是完整 oracle。
- 实际 FAIL 阻断；全仓/race/vet PASS 不抵消。环境/测试 oracle 错误单独定位并保留纠正链；完成进程不等于接受。
- race harness 与子 binary build flags、working-tree 与 fresh-fetch、Linux 与 Mac、注入中断与断电分别写清。
- 不以本 session 角色切换或 specialist prompt 充当项目要求的独立 reviewer。复审仍由用户授权的独立线程/checkout完成；不要从此 skill 自动委派或新建线程。

## Findings 与规则同步

发现写为：文件/函数、确切 trigger、上游/下游路径、已核结果/期望、现有 guard 为什么未挡、最小修复建议和成对控制。未实证写假设；无问题是有效结论。

在 review 报告核新不变量是否落实到最窄 AGENTS 和 `docs/DEVELOPMENT_NOTES.md` 的永久回归入口。只读 review 只提文档建议，不擅改待审树；获得修复授权后改动也要新 fixed 身份与复审。

结论分别写接受/拒绝/尚未核准、本批范围、阻塞、实际验证与交付状态。记录剩余未执行检查，不用“全部通过”盖住未覆盖表面；本地集成不等于 push/release 或完整 T2–T6。
