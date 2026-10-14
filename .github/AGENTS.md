# CI 证据与分流

- 必需工作流始终产生真实检查；保留准确 head/base/tree 与模式，不用 paths-ignore 或空绿色检查代替产品验证。
- 是否可走文档快路径由准确 base 的分类器判断，不执行本 PR 修改的分类器来给自己降级；base 缺失/不可用/非祖先及首次安装必须完整路径。先分类再执行 head 控制测试。
- 分类器/工作流改动须用实际 workflow shell 的公开正反例核来源边界、GITHUB_OUTPUT 与 artifact；脚本自身测试通过不证明调用者取了正确来源。
- 静态链接检查不能跨段落剥离代码而漏掉可见链接；docs 模式绿色必须同时记录 product_tests=not_run。此流程不是任意 PR 代码的安全隔离。
