# 运行时与构建工具链：.NET Framework 4.8.1、SDK 不固定、Inno Setup 7

状态：已采纳（2026-10-18）

程序以 .NET Framework 4.8.1 为目标，仍是一个内嵌全部依赖、不用另装运行时的单文件 EXE：Windows 11 22H2 起自带 4.8.1，AntdUI 和 TaskScheduler 用各自的 net48 版本。构建用的 .NET SDK 不在仓库里固定，本机和 CI 各用已装的最新 SDK（README 建议用当前 LTS 版本）。安装包用 Inno Setup 7 编译，版本号和安装包的 SHA-256 写在 `scripts/inno-setup.ps1`，CI 和 Release 在测试前先装好；简体中文用 Inno Setup 自带的官方翻译。

这三项是用户 2026-10-18 在问答中选的：目标框架「提到 4.8.1」、SDK「装 SDK 10，不固定」、安装包「迁到 Inno Setup 7」。实现见提交 3299dcc。

## 当时考虑过的其他做法

- **目标定为 .NET Framework 4.8**：同样覆盖所有 Windows 11，库也用 net48 版本；用户选了 4.8.1。
- **改用 .NET 10（LTS）**：用户要么另装 .NET 桌面运行时，要么 EXE 打包运行时后涨到几十到上百 MB，和「单文件、免装运行时」的定位冲突。
- **用 `global.json` 固定 SDK 10.0.x**：本机和 CI 的编译器能保持一致；用户选择不固定。
- **继续用 CI 机器自带的 Inno Setup 6**：省掉安装步骤，但编译器版本会跟着 CI 镜像变，简体中文翻译还得随源码另带一份。

## 带来的影响

- 没更新到 22H2 的 Windows 11 要先通过 Windows 更新装上 .NET Framework 4.8.1 才能运行。
- 本机和 CI 用的 SDK 可能不是同一个版本；如果以后需要逐字节可复现的构建，再考虑固定 SDK。
- 升级 Inno Setup 时，要同时改 `scripts/inno-setup.ps1` 里的版本号和 SHA-256（用 GitHub 发布页公布的摘要）。
