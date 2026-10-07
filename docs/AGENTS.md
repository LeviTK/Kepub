# 文档与证据

- `CLI_CONTRACT.md` 定义行为，`DEVELOPMENT_PLAN.md` 定义批次/依赖，`verification/` 记录真实验证；`DEVELOPMENT_NOTES.md` 保存跨批稳定教训，不能替代契约或批准。
- 每条验证绑定完整 commit/tree（或提交前 working-tree 身份）、runner、命令、退出码、输入/日志 hash 与范围。作者、父、独立 reviewer、fresh-fetch 结果分别标注，不搬用旧树 PASS。
- suite exit0 不代表 reviewer 批准；冻结反例 FAIL 不得删、改弱或被总 PASS 抵消。工具/cwd/编译/测试 oracle 错误单独分类，保留原记录和纠正记录。
- 精确描述 fuzz oracle 和输入族：WriteSet 相等、候选可读、完整资源字节相等分别是不同强度；计数不代表穷举覆盖。
- local main、origin/main、commit 数量、bundle size/hash/prerequisite 用实际 Git/文件证据核准，不把本地集成写成 push/release。
- Linux 合成、故障注入、交叉编译、harness race 分别写清；不得冒称私有书、Mac 实机、真实断电或完整 T2–T6。
- review 确认新不变量时更新最窄 AGENTS.md 和开发注意事项中的规则/回归入口；只写稳定可执行规则，不复制整段聊天、私有路径或敏感日志。未证实猜测不写成根因。
- 不直接修改 `specs/` 的冻结原文、索引、hash 或历史收据来制造 green；这类变更须按既有资产生成和独立门禁流程处理。

- 派生资产（text 摘录等）的 bytes/SHA-256 由实际文件生成并核验；`text/extract.py` 重生成后校验
  manifest 与 ATTRIBUTION 的每一行，不一致即失败。只修派生元数据/生成流程，不改 raw 原文或 S0 身份。

对应开发注意事项：`G-EVIDENCE`、`G-ASSET` 与“Review 维护流程”。
