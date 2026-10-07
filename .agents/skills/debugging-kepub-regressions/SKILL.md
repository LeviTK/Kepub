---
name: debugging-kepub-regressions
description: "Diagnoses Kepub EPUB editing regressions and review rejections with frozen repros, provenance tracing, sibling checks and red-green tests. Use when repairing transaction, identity, XML byte, URL, or recovery failures."
license: MIT
compatibility: "Requires repository read/shell tools, Git and the project's Go toolchain; real conformance checks additionally require pinned EPUBCheck and Java."
metadata:
  upstream: "https://github.com/tw93/Waza"
  revision: "6b6c736561eaf24c4d1c5360c3c90767ab470805"
  adapted-from: "skills/hunt/SKILL.md"
---

# Kepub：先证实根因，再修回归

这是 Waza hunt 的项目适配，不是 upstream 原样安装；来源/差异见 [安装说明](../README.md)，许可见 [MIT](../LICENSE.waza)。遵循宿主工具/授权规则与项目 AGENTS；skill 不授予产品改动、push、发布或其他线程操作权限。

## 工作流

1. 在项目根核 `git status --short --branch`、HEAD/tree、输入 bundle/seed hash。读取对应契约、模块 AGENTS 和 `docs/DEVELOPMENT_NOTES.md`；旧证据是线索，不冒充本轮执行。
2. 列真实失败与合法控制，定义独立 oracle。先执行/读取可绑定输入的最小 repro；不能因全仓 PASS、编译成功或“看起来同一个原因”跳过实证。区分 assertion FAIL 与环境/fixture/工具错误。
3. 沿 caller → 所属状态/来源模型 → consumer 写一句具体因果：哪个函数/分支在什么输入下破坏哪个契约。若反例不支持假设则丢弃，不叠猜测式补丁。
4. 选择最窄修复，保持旧 schema、权限、预算与合法正控。多次相似失败先调整错误模型/快路径，而不是加 special case。重点对照 G-FINAL/G-FACTS/G-BINDING/G-PROVENANCE/G-URL。
5. 提取同形模式，在所属 first-party 目录 scoped rg 并阅读其他入口：同问题/安全且说明原因/未核准。明确未覆盖边界；无关问题报告，不扩展修复任务。
6. 原冻结 probe/seed 不改弱；添加永久测试。在隔离旧树/overlay 实测 red（应由目标断言失败，不是编译失败），在修复树实测 green；不要 revert、stash 或重写共享候选。相同输入、编码、运行 flags 与原始期望保持。
7. 对受影响模型跑成对正反、两序实际 Plan+Apply、完整 bytes/图 oracle 和 focused race；有效事务 fuzz 意外拒绝也 FAIL。共享批次的全仓与 CLI 验证服从根 AGENTS，不调用 Waza 通用测试脚本。
8. 将已证实不变量更新至最窄 AGENTS 和开发注意事项的回归入口。保留 red/green/工具错误；清理仅自己创建且身份核准的临时编译副本，不删证据。

## 完成条件与交接

报告根因、修复范围、同形分支检查、red/green 命令与输入身份、永久测试、未验证边界及当前交付状态。证据缺失就写未核准，指导存在不等于防错成功。

任何冻结/live FAIL 均阻止集成。产品改动需新 commit/tree/bundle，旧批准不迁移；作者自审不是独立审查。没有授权不修改其他工作树，不自动启动 agent/线程，也不 push/release。
