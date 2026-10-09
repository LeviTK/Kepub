# Archive：统一展开库存预算

- Open 在解压 staging 前收齐并限制唯一文件、显式目录和全部隐式父目录；共享/显式父目录只计一次，
  重复显式 ZIP 条目与大小写/Unicode、文件/目录冲突仍拒绝。不按原始 ZIP 条目数代替展开库存。
- ZIP 在追加下一条中央目录记录前执行库存门禁，不先通过 zip.OpenReader 分配全目录，
  不恢复旧的 `len(z.File)` 原始条目数快拒（它不是 EOCD 声明数），也不改用声明数快拒。
  逐条门禁错误先于后续结构 INVALID_ZIP；移除旧快拒还会改变非碰撞错误的优先级，例如默认
  ZIP 路径策略下，unsafe 首记录＋错误 count、mimetype 后 unsafe 且 Entries=1 均为 UNSAFE_PATH。
  保留 ZIP64、偏移兼容和原数据 CRC
  核验；单条可变元数据按 ZIP uint16 长度读取，不添加私有格式限额。
- framing 回归保留 count+65536 与长 EOCD comment 正控，并拒绝 count 不符、声明记录截断和
  comment 长度越界；核完整资源字节、原书未变和 staging 清理，不能只测正常短 EOCD。
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
