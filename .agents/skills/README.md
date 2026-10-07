# Kepub 项目 skills 与 Waza 来源

本项目只安装两个**Kepub 适配版**，不是 Waza 全量原样安装：

| Skill | 触发 | 基础来源 |
| --- | --- | --- |
| `debugging-kepub-regressions` | 排查/修复回归、review 拒绝、字节/引用/绑定问题 | Waza hunt：根因先于补丁、实测 red/green、同形路径检查 |
| `reviewing-kepub-changes` | 编码自审、固定批次交接、独立复审 | Waza check：项目上下文、冻结 diff、finding 实证、规则同步 |

上游：[tw93/Waza](https://github.com/tw93/Waza)，固定
[commit 6b6c736](https://github.com/tw93/Waza/commit/6b6c736561eaf24c4d1c5360c3c90767ab470805)，
2026-10-07 审阅；版本字段为 3.39.0，但**commit pin 才是本次来源身份**。

| 审阅源文件 | 原文件 SHA256 |
| --- | --- |
| `skills/hunt/SKILL.md` | `8ae2315c6242a7f59a938650e398d977569831ade1346e0fceee5a12cf69d1a1` |
| `skills/check/SKILL.md` | `1939fba3ea348a8713b7df158ec7d5c9baa5bfd26c0e89296bb7dd8ebfc57488` |
| `LICENSE` | `83ef2e3caa22ff257740df8684627ef8c4e21102986cf943e3fdd31ca7b430d5` |

## 适配边界与安全

- 以 MIT 许可适配流程，保留 [完整上游许可](LICENSE.waza)。skill 改用 gerund 名称，描述范围收窄为 Go/EPUB；项目规则与宿主授权优先。
- 移除通用发布/ship/triage、自动 specialist 分派、全局 memory/routing 与 UI 模式；不安装其他 Waza skills、agent prompts、hooks 或全局规则。
- 不含任何脚本、MCP/server、环境变量、网络安装动作或新的 Go 依赖；没有运行 `npx skills add` 或上游 rule installer。
- 不复制 `check/scripts/run-tests.sh`：它无 go.mod 分支。不复制 `release_gate.py`：它的 WARN/FAIL 仍 exit0，不能当 Kepub 批次门禁。使用项目实际 Go/CLI/checker 流程。
- 适配正文自包含，未引用未安装的 Waza references；长期不变量由项目 AGENTS 和开发注意事项维护，不复制上游整套提示词。
- 这些是行为指导，不会强制执行校验或消灭 bug。只有实际回归/字节 oracle、fixed identity 与独立证据可批准批次。

安装位置是 `.agents/skills/`，适用于支持 Agent Skills 的宿主；Amp 使用 `reload_skills` 后加载。不同 orb 不共享本地文件，须先传递相同文件，再分别 reload；不升级/重启正在执行的产品测试。
以后更新先审阅新的精确 commit、许可、entrypoint/依赖与权限差异，再改 pin/hash/正文；不得自动追 main，也不增加全局安装或发布授权。
