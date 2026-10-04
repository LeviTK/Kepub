# M2-A 工作区基础验证

日期：2026-10-04（Linux orb）；实现基线为 M1-A 与开发方案 v0.4。修改仅在 `internal/workspace/` 和本文，没有修改 archive/publication、根依赖锁或 CLI。

## 已交付的库边界

`workspace.Create(dir, source, Options{Rootfile: ...})` 创建并持有单写者工作区；`Open` 重开并取得相同所有者锁；`Close` 释放锁。`State` 返回导入来源与初始清单的独立副本。

`NewCandidate` 创建唯一活动候选并返回绝对出版目录；`Candidate` 找回已有目录；`Checkpoint` 返回独立快照的 ID 与清单；`Checkpoints` 枚举并验证已存快照；`Restore(id)` 恢复指定快照。`HashTree` 独立计算目录清单与内容哈希。

这不是 workspace CLI、socket 或任务服务，没有 plan/apply、accept/reject、export、Agent 启动或检查器。`initial` 仅是导入快照名称，不是审核通过；状态和快照中没有伪造的 `passed`、`accepted` 或 `valid` 字段。损坏 XHTML 等普通文件可以被检查点保留供调查。

```text
<workspace>/
  owner.lock                        # 保留 inode，Close 不删除
  state.json                        # version、原书摘要、rootfile、初始树、只读原因
  original/book.epub                # 输入容器字节的独立副本
  revisions/initial/pub/            # M1-A 安全解包后的所有条目
  tasks/active/
    task.json                       # 仅版本和 baseRevision
    work/pub/                       # 唯一可写出版候选
    checkpoints/<random-id>/
      checkpoint.json
      pub/
  staging/                          # 库专用、可清理；不是用户存储目录
  journal/restore.json               # 仅恢复进行中存在
```

不预造 plans、preview、reports、通用任务状态机或数据库；本轮没有候选销毁/接受流程，因此一个工作区最多创建一次 `active`，再次创建返回存在冲突。

## 数据保护与失败处理

- **源书与初始版本**：先将普通输入文件复制到私有 staging，再调用冻结的 `archive.Open`、`publication.Load` 和 `Archive.Unpack`。原书按容器字节保留，未改出版资源按展开条目字节保留，包含空目录和 OPF 未登记文件。多 rootfile 必须显式选择并记录。源路径随后删除不影响重新打开。
- **完整发布**：目标必须不存在，父目录必须已存在。Linux 用 `renameat2(RENAME_NOREPLACE)`，Darwin 实现用 `renameatx_np(RENAME_EXCL)`；拒绝已有普通文件、空/非空目录和符号链接，也拒绝预检后才出现的空目录。创建期间已有工作区不会被覆盖。
- **独立复制**：original、revision、candidate、checkpoint 之间不创建普通硬链接或 APFS 克隆。候选通过普通截断、原位写入、改名、增加或删除文件，不会改变已有基线与检查点；恢复后的候选仍是新复制。
- **清单哈希 v1**：精确 POSIX 路径按字符串字节序排序；每条记录包括 `path/type/size`，普通文件另含内容 SHA-256，目录 size 为 0。以固定 Go JSON 字段顺序编码清单，哈希输入为 `kepub-tree-v1\n` 加编码字节。根自身不作为条目；包含空目录。mtime、权限、遍历顺序不参与；相同内容改路径、大小写、类型或大小均改变清单。不是只串接文件内容或只哈希 OPF manifest。
- **路径和文件类型**：根及其祖先必须是真实目录，调用方需传已解析系统别名的路径（例如 macOS 的 `/private/tmp` 而非符号链接 `/tmp`）。后续读写使用 `os.Root` 限制在根内；出版树拒绝任何符号链接、普通文件硬链接、FIFO/特殊文件、不规范 BookPath 和大小写/Unicode 碰撞。快照 ID 必须是 32 位小写十六进制，不允许路径输入。源 EPUB 本身的符号链接/硬链接也拒绝。
- **所有者锁**：非阻塞 advisory `flock` 绑定打开的 `owner.lock` 文件描述符；第二个句柄/进程收到 `ErrBusy`。不依赖 PID 文件，不 unlink/recreate 锁 inode；正常退出和 SIGKILL 均由内核释放。句柄方法通过 mutex 串行，关闭后拒绝写操作。
- **候选恢复**：先复制并核对完整 checkpoint，原子发布 restore journal，再将旧 pub 移至内部备份，将新 pub 移入固定位置。恢复只替换 `work/pub`；其中在所选检查点之后增加的文件会移除，这是显式恢复的语义。`work/` 旁边的笔记和 workspace 根的其他用户文件不动。该流程可恢复，但不是允许并发读者观察的单次原子目录交换。
- **I/O 失败**：journal 前失败清理本操作 staging，候选不变；journal 后失败保留旧/新树与 journal，句柄返回错误并进入 `ErrRecovery`，必须关闭后重开。`Open` 验证原书、revision、所有 checkpoint，完成已有合法 journal 的恢复，再验证候选文件系统并清理库专用 staging。损坏清单、危险路径或坏 journal 拒绝打开且不删除恢复证据。不把这些错误转成新工作区或绿色状态。
- **用户文件**：非 manifest 资源与候选中新建普通文件均进入树和 checkpoint，不按扩展名丢弃。未知工作区根文件不自动删除；`staging/` 是明确的内部保留区，其遗留内容在成功重开时清理，不能放用户文件。
- **已知不支持输入**：保留原书与 revision，但 M1-A 除 `READ_ONLY_PARTIAL` 以外的 limitation 均限制候选创建。已知书内配置组件 `AGENTS.md`、`CLAUDE.md`、`.amp`、`.claude`、`.mcp.json`、`mcp.json`、`.vscode`、`.git`（不区分大小写，任意深度）同样只读；不复制到可运行的 Agent 工作目录。这不是证明所有未来 Agent 配置发现已隔离，也没有实际启动 Agent。

所有库创建的目录为 0700、普通文件为 0600（受 umask 收紧）。所谓不可变，是库不修改 original/revision/checkpoint 并在重开/使用前核对清单；同一用户仍可直接改它们。锁、cwd、权限和复制不是同用户恶意进程的 OS 沙箱。调用方必须先停止源书/候选的所有外部写入者；检查点的复制前后哈希能发现一般竞争，但不能证明并发目录扫描是原子快照。

## 已执行验证

环境：Go 1.27.1，Linux x86_64，非 root 用户，临时生成的小型 EPUB；无真实书籍、付费模型或外部检查器。

```text
go test -count=1 ./...
  所有包通过（workspace 约 0.43s）
go test -race -count=1 ./...
  所有包通过（workspace 约 4.91s），无 race 报告
go vet ./...
  退出 0，无诊断
GOOS=darwin GOARCH=arm64 go test -c -o <temporary>/workspace.test ./internal/workspace
  成功生成 Mach-O 64-bit arm64 executable；没有执行它
```

主要回归测试：

| 测试 | 验证区别 |
|---|---|
| `TestIndependentCopiesRestoreAndReopen` | 普通直接写入、删除、改名、新用户文件；原书与基线字节不变；两个检查点恢复；恢复后的写入不改检查点；移除输入后重开 |
| `TestTreeHashContract` | 独立字面量确定哈希编码，已知 SHA-256 样本；mtime 不影响，同大小内容改变影响 |
| `TestTreeOrderPathTypeAndSize` | 反向创建顺序一致；同内容不同精确路径不同；空文件和空目录不同；实际字节大小 |
| `TestTreeRejectsUnsafeEntries` | 文件/目录/根内符号链接、硬链接、大小写与 Unicode 碰撞 |
| `TestExistingDestinationAndConcurrentCreation` / `TestCrossProcessCreateRace` | 已有用户对象不变；8 goroutine 与4独立进程分别竞争创建，仅一者成功、失败者无残留 |
| `TestPublishDoesNotReplaceLateEmptyDestination` | 预检后才出现的空目录仍不替换，stage 保持完整 |
| `TestCrossProcessLockAndExitRecovery` | 同进程和跨进程冲突；子进程不调用 Close 的正常退出与 SIGKILL 后可重开；锁 inode 不变 |
| `TestCreateFailuresLeaveNoPartialDestination` | ZIP/OPF/路径/多 rootfile/缺源/源 symlink 失败，没有目标或 staging 残留 |
| `TestSpecialFilesAndDiskFailure` / `TestLateIOFailuresAndRecovery` | FIFO 拒绝；真实权限拒绝发生在 stage 创建、checkpoint 发布、journal 后旧 pub 移动处；失败后磁盘状态、句柄限制与重开恢复 |
| `TestRestoreInterruptionRecovery` | journal 前、journal 后、移开旧树、发布新树、删除备份、完成这六个真实磁盘状态的重开结果；未使用 mock rename |
| `TestFailurePreservesCandidateCheckpointAndEvidence` | 损坏 checkpoint/恢复 stage/journal、候选和元数据 symlink；失败保留用户内容和恢复证据 |
| `TestCheckpointIDsNeverTraverse` / `TestTamperingAndStateCopies` | ID 路径穿越拒绝；基线改动拒绝；失败 Open 释放锁；State 不泄漏内部可变 slice |
| `TestReadOnlyInputsPreserved` | 书内指令/MCP/加密声明/签名资源保留但不产生可写候选 |

## 尚未证明与后续约束

- **macOS 未实测**。Darwin 平台文件仅交叉编译；APFS 大小写/Unicode 行为、`flock`、目录 fsync 与 rename 语义仍需 Apple Silicon 实机验证。Linux 碰撞 fixture 不能直接证明默认大小写不敏感文件系统的行为。
- **断电耐久性未证明**。实现检查文件/目录 Sync 并按 journal 顺序发布；未证明突然断电、磁盘控制器缓存、NFS、文件系统损坏或 macOS `F_FULLFSYNC`。中断测试重建操作边界，不冒充真实断电实验。
- **ENOSPC 未注入**；实测故障是权限拒绝及实际损坏文件，不能将其写成所有磁盘故障均已覆盖。若失败清理本身失败，会返回合并错误并保留残留。
- 创建阶段若进程在最终发布前被强杀，目标仍不存在，但父目录中可能保留私有 `.kepub-create-*`。本轮不会扫描删除用户目录中这些兄弟项；需人工核实后删除，不能把名字前缀当作删除授权。已发布工作区内的候选/检查点 staging 则由 `Open` 清理。
- 候选可能是无效出版物；本库不运行引用检查/EPUBCheck，不保证审核可接受。Open 遇到候选危险文件系统条目会拒绝，保留磁盘供人工检查，不静默移除链接。没有容量淘汰策略、空间配额或性能验收。
- 原书签名/加密、配置隔离和全部出版结构的生产编辑政策仍由后续应用层负责；本轮保守只读策略不是完整安全认证。
