# CLI：契约与真实闭环

- 参数、帮助、capabilities 和版本化 operation schema 复用既有注册表来源；保留旧 envelope、退出码、schema shape 与摘要语义，不静默升级旧请求。
- 使用既有 `workspaceBinary`、`processJSON`、`checkExport` 测真实二进制，不拿库测试替代 CLI 证据。负例核退出码/code/无 plan 或正式产物，正例核实际 task diff 候选值。
- 正式 accept/export 使用固定 EPUBCheck 5.3.0 及校验过的依赖。缺 checker 或 skip 不是符合性 PASS；checker 正确拒绝合法编辑但非合规候选时，分别报告产品编辑和合规门禁结果。
- 独立 ZIP oracle 包含文件、保留的目录条目、资源字节与原 EPUB hash；不要只数文件、只 Contains 或从实际输出反推期望。
- `go test -race` 中 helper 的普通 `go build` 子 binary 不带 race instrumentation；报告 harness/library race，不称 race CLI，除非确实另行 `go build -race`。
- 新鲜进程重开、历史状态与 accepted revision 绑定按实际执行记录；原书只读，失败无部分正式导出。

对应开发注意事项：`G-ORACLE`、`G-EVIDENCE`。
