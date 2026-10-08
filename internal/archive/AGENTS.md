# Archive：统一展开库存预算

- Open 在解压 staging 前收齐并限制唯一文件、显式目录和全部隐式父目录；共享/显式父目录只计一次，
  重复显式 ZIP 条目与大小写/Unicode、文件/目录冲突仍拒绝。不按原始 ZIP 条目数代替展开库存。
- 累计精确 BookPath 的 UTF-8 字节，包含目录，不包含末尾 `/` 或根 `.`；加法前比较剩余预算。
  路径/条目预算与实际单文件/总文件字节预算分开，不截断清单或丢未声明资源/空目录。
- Archive 保留入口 limits；Unpack、Inventory、WriteZIP 与 PublishZIP 最终重开沿用同一预算。
  工作区内部管理目录不是出版物；正式导出仍要真实 checker，不用草稿或单元 callback 代替。
- 回归使用逐调用的小 Limits，不修改全局、不耗尽真实资源；核落盘前拒绝、本次 staging 清理、
  原书/已有目标不变与完整展开库存/资源字节。永久入口为 `TestR2*`；规则见 G-INVENTORY。

- 实际资源读取共用 `CopyBounded`，最多读取剩余额度 + 1 探测字节，int64 上界不溢出；
  读故障、短写、取消不得因同时读到探测字节被改成预算错误。原 ZIP 用独立 `MaxInputBytes`，
  不拿压缩字节冒充展开字节。入口 `TestR3ActualReadBudget`、`TestR3BoundedCopyFaults`。
- 目录用 `WalkDirectory` 逐条遍历，callback 在下钻／数据打开前核条目与路径预算；
  不先无界 ReadDir 再限量。Unpack／Inventory／快照及 ZIP 使用同一 Limits 和请求 context，
  不放宽 no-follow、单链接或碰撞拒绝，不把边界取消观测宣称为打断阻塞系统调用。
