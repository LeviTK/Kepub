# CLI：契约与真实闭环

- 参数、帮助、capabilities 和版本化 operation schema 复用既有注册表来源；保留旧 envelope、退出码、schema shape 与摘要语义，不静默升级旧请求。
- 使用既有 `workspaceBinary`、`processJSON`、`checkExport` 测真实二进制，不拿库测试替代 CLI 证据。负例核退出码/code/无 plan 或正式产物，正例核实际 task diff 候选值。
- 正式 accept/export 使用固定 EPUBCheck 5.3.0 及校验过的依赖。缺 checker 或 skip 不是符合性 PASS；checker 正确拒绝合法编辑但非合规候选时，分别报告产品编辑和合规门禁结果。
- 独立 ZIP oracle 包含文件、保留的目录条目、资源字节与原 EPUB hash；不要只数文件、只 Contains 或从实际输出反推期望。
- `go test -race` 中 helper 的普通 `go build` 子 binary 不带 race instrumentation；报告 harness/library race，不称 race CLI，除非确实另行 `go build -race`。
- 新鲜进程重开、历史状态与 accepted revision 绑定按实际执行记录；原书只读，失败无部分正式导出。
- NotifyContext 必须传到 info／toc／inspect 初始 Read 和 unpack 的读取／复制；输出路径核验
  的取消／超时复用既有分类，不能改为 INVALID_OUTPUT。信号回归须核真实子 PID、运行阶段、
  CANCELLED/130 和未发布 staging 清理；成功发布后不因迟到取消改报失败。
- validate／pack 的冻结、复制、hash 原始 DeadlineExceeded 也须映射 CHECKER_TIMEOUT/5；
  以返回错误链判超时，不因 ctx 后来过期把真实 I/O 错误改为超时。保留信号 CANCELLED/130
  与已发布成功；`TestD2EarlyValidationDeadline` 核两入口的早期超时、取消、I/O 与成功控制。

- 已存在外部输出保留冻结 `OUTPUT_EXISTS`/2；emit-request 在写文件前执行 256 操作预算
  （256 成功、257 `INVALID_OPERATIONS`/2 且无产物）；错误码断言按契约精确匹配，不接受两种 code 的宽松断言。

- 所有不可信终端错误／writer失败、摘要字符串与动态键共用 `terminalQuote`；控制字节、
  CR/LF/TAB、C1、双向格式字符与非法 UTF-8 字节可见转义，中文／正常文字保留。
  程序排版换行与消息内容分开；JSON 的原 message 只由 JSON 编码器处理，实际 task diff
  已有安全输出原样保留，不二次转义。doctor/help 的静态注册表与安全 readiness 原因不伪称用户输入。
- 摘要的 JSON 形状归一化不能作为字符串／动态键的字节来源；保留原 Go 字符串，
  区分非法 FF、FE、合法 U+FFFD 与字面反斜杠。截断按有效 rune 或单个非法字节推进，
  不在任意 continuation 字节处向前无界回退；机器 JSON 仍保持编码器的原替代语义。
- `TestR5MissingPathTerminalEscape` 必须经过真实 run/OS错误链；
  `TestR5ErrorControlsAndJSONSemantics`、`TestR5OutputWriterErrorPaths` 核 code/exit/envelope
  与三个实际 writer分支；测试先捕获到 Buffer，日志用 `%q`，不能直接向测试终端发控制序列。

对应开发注意事项：`G-ORACLE`、`G-EVIDENCE`、`G-QUALIFY`。
