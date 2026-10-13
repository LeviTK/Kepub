# Validation：失败报告的分类

- 输入冻结／复制／hash 的原始 DeadlineExceeded 在 failedInput 生成报告前归为
  CHECKER_TIMEOUT/5，使返回 error 与报告 diagnostics 同类；只看返回错误链，
  不因 context 后来过期覆盖真实 I/O。CLI 的信号取消映射仍由命令层负责。
- `TestD2EarlyValidationDeadline` 同时核公开错误分类和报告诊断；
  超时／取消不产生正式成功，schema、检查器资格与真实 I/O 分支不扩权。
