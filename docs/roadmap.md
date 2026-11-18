# 路线图：用 Go 与 mygo 重写

> 本页是把 Power Meter 从 C#（.NET Framework 4.8.1、WinForms、AntdUI）整体重写为 Go 加
> [mygo](https://github.com/egoist/mygo) 原生界面的计划：决定、代码布局、分阶段内容与验收、风险和进度。
> 功率口径以 `GLOSSARY.md` 为准，数据源结论以 `docs/research/system-input-power.md` 为准，两者在重写中不变。

## 目标

- 产物是一个纯 Go 程序。界面用 mygo 的原生界面（`ui` 包，在 GPU 上自绘，不用 webview）；Windows 接口经
  `golang.org/x/sys/windows` 与 `syscall` 调用，不用 cgo。运行不再需要 .NET Framework。
- 仓里不再有 C#、PowerShell、Inno Setup 和 .NET 配置。构建、单元测试和打包都在 Linux 上完成；只有碰真实硬件、
  UAC、计划任务和安装卸载的验收留在 Windows。
- 用户看到的行为不变，范围见下节。

## 决定

- **行为对齐。** 功率口径、数据源及探测顺序、显示规则（`≈`、`N/A`、托盘 `--`、色调、统计窗口、托盘文字）、
  命令行开关与退出码、设置位置 `HKCU\Software\BatteryChargeMeter`、自启任务名
  `BatteryChargeMeter.Logon.<SID>`、受保护副本路径及 `docs/testing/protected-autostart.md` 的不变量全部沿用。
  重写期间不顺带改功能。
- **数据源与现版本相同。** 电池读 WMI `root\WMI` 的 `BatteryStatus`；CPU 包功率读 EMI；平台功率经 PawnIO 的
  `IntelMSR` 模块读 Psys。WMI 与任务计划程序这两处 COM 调用用纯 Go 的 `github.com/go-ole/go-ole`。
- **切换方式。** 重写期间 C# 版是唯一的发布物，发版流程不变。Go 版通过全部阶段验收后，用一次发版切换，并在同一
  阶段删掉 C# 代码、PowerShell 脚本、Inno Setup 定义和 .NET 配置。切换版的版本档位按 `docs/release.md` 由维护者
  确认。
- **两边一致。** 重写期间 C# 版若改了行为，同一改动也要进 Go 版，切换前两边一致。
- **固定 mygo 版本。** mygo 在 `go.mod` 里固定版本，命令行工具用 `go tool mygo`。升级 mygo 单独做，升级后重跑
  阶段 3 的界面验收。
- **升级起点。** 只支持从最后一个 C# 版本升级。旧版留下的、指向普通权限可写文件的任务，仍按不变量迁移或删除；
  `BatteryChargeMeter.exe` 兼容入口不再提供。
- **发布目标与资产。** 仍只支持 Windows 11 x64（`windows/amd64`）。资产名不变：
  `PowerMeter-vX.Y.Z-windows.exe`、`PowerMeter-vX.Y.Z-windows-setup.exe`、第三方通知、PawnIO 对应源码包，
  各带 `.sha256`。

## 不在本次范围

- mygo 的自动更新。受保护副本只能由提权进程刷新，它与自动更新怎样配合另议。
- 代码签名、Windows on Arm 版、网页界面。

## 代码布局

- Go 模块在仓根。`cmd/powermeter/` 是 mygo 项目目录，放 `main.go` 与 `mygo.json`，产物名 `PowerMeter.exe`。
- `internal/` 下按职责分包：
  - 纯逻辑包不依赖 Windows，在 Linux 门禁里测：功率口径与快照合成、供电状态、统计与趋势、显示会话（对应现版本的
    `MeterSession`）、中英文案、命令行语法、自启策略决策表。
  - Windows 包带 `windows` 构建约束：电池、EMI、PawnIO、提权、单实例、计划任务、受保护副本。它们经接口接入，
    测试里用假实现替换。
- 界面只把显示会话给出的视图画出来，规则不进界面，与现版本 `MeterSession` 和 `MainForm` 的分工相同。

## 阶段

阶段按顺序做。每个阶段在自己的任务分支上完成并合入；切换前 Go 版不发布。

### 阶段 0：可行性试点

在目标机上用最小的 Go 程序逐项实测下面几处。全部通过才进入阶段 1；有不通过的，先在本页记下结论和替代做法。

- **窗口**：隐藏系统标题栏、自绘按钮（`TitleBarHidden`、`HideWindowButtons`、`DragWindow`），置顶钉住；
  Alt+Space 系统菜单、DWM 阴影与圆角、最小化动画都正常；100%、175%、300% 缩放与跨屏切换正确。
- **托盘**：每秒换一次渲染出的整数瓦数图标（`Tray.SetIcon`），16 到 32 像素都看得清；悬停文字、右键菜单勾选、
  双击恢复窗口。
- **数据**：Go 读出 WMI `BatteryStatus`、EMI 的 `RAPL_Package0_PKG`，提权后经 PawnIO 读出 Psys，与现版本
  `--power-probe` 同时段的读数一致。
- **提权**：经 `runas` 重启时只请求一次 UAC，取消后原进程继续运行。
- **构建**：在 Linux 上用 `go tool mygo build -platform windows/amd64` 出 EXE 和 NSIS 安装包；用自带的 `.syso`
  把 manifest 设为 `asInvoker`，并保留 Per-Monitor V2。
- **测试**：mygo 的 `ui.Tester` 能在 Linux 门禁里不开窗口地跑界面视图测试。跑不了的话，视图测试改到 Windows 跑。

### 阶段 1：纯逻辑核心

- 移植口径、供电状态的八种状态、快照合成（现版本 `PowerSnapshot.Compose`）、10 分钟时间加权均值与 5 分钟峰值、
  30 分钟趋势的分桶、显示会话、中英文案、命令行语法与退出码、自启策略决策表。
- 把 `tests/PowerMeter.Tests` 的用例逐条改写成 Go 的表驱动测试，覆盖的情形不少于现有用例。
- 门禁加入 Go 检查：gofmt、vet、测试、Go 版本与依赖核对，用 `scripts/gatelib.sh` 里现成的函数。
- **验收**：移植后的测试在门禁里全部通过。

### 阶段 2：Windows 数据源与诊断

- 电池、EMI、PawnIO（含 RAPL 计数器回绕、能量单位与 EMI 交叉校验）、来源探测与不可用原因。
- `--power-probe`、`--third-party-notices`，以及 `--self-test` 中不涉及界面的部分。
- `IntelMSR.bin` 与第三方通知用 `go:embed` 内嵌；EXE 旁的同名 `IntelMSR.bin` 仍覆盖内嵌副本。
- **验收**：在目标机上，纯电池和插电充电两种状态，普通权限和提权两种启动，各跑一次 Go 版与 C# 版的
  `--power-probe`。两份报告的来源、可用性与不可用原因一致，读数在采样误差内一致。

### 阶段 3：界面与托盘

- 按 `docs/testing/ui-localization.md` 重画窗口：深色仪表主题、自绘标题栏（应用图块、显示模式分段、钉住、最小化、
  关闭）、主读数与 `≈`、单位的对齐、电池电压与估算电流、30 分钟趋势（均值线、最小最大值带、光晕、峰值标记、
  当前值光标、缺失处断线）、10 分钟均值与 5 分钟峰值、十格电量、带点线引导的其余三项口径、诊断区与管理员重启按钮、
  设置弹层（语言、登录自启开关）。
- 托盘图标、悬停文字和菜单；系统默认、中文、英文三种语言即时切换。
- `--screenshot`、`--dpi-preview`、`--tray-preview` 用 `ui.Render` 重做。
- **验收**：用同一份夹具读数，与 C# 版的 175% 中英文空闲、来源不可用、小图标预览逐张并排比对；
  `docs/testing/host-acceptance.md`「Interactive acceptance」第 1 到 5 条在目标机通过。

### 阶段 4：启动、提权与登录自启

- 启动路由与提权：只请求一次 UAC，取消后普通运行，窗口里有重新请求的按钮。
- 单实例与唤回：跨提权级别唤回已有窗口，按来源路径认出自启实例，重复的自启启动直接退出。mygo 的
  `RequestSingleInstanceLock` 满足不了这些时自己实现。
- 计划任务的注册、读取与任务 ACL；受保护副本的复制、DACL 与所有者、`source.txt`；`--sync-autostart` 与
  `--remove-autostart` 的退出码 0、1、3、4、5、6；同名的外来任务不修改、不接管。
- Go 版直接接管 C# 版注册的任务和副本，用户不用重新启用。
- **验收**：`docs/testing/protected-autostart.md` 的不变量与「Host results」各项在目标机重跑通过；
  「Interactive acceptance」第 7、8 条通过。

### 阶段 5：打包、发布与切换

- **安装包**：改用 mygo 生成的 NSIS 当前用户安装包。`windows.installerInclude` 的 `customInstall` 跑
  `--sync-autostart`，需要时请求一次 UAC；`customUnInstall` 跑 `--remove-autostart`，再请求一次 UAC 删副本。
  取消、失败和外来任务的处理与现版本相同。
- **从 C# 版升级**：从最后一个 C# 版本的安装升级时，设置、自启任务与受保护副本保留并刷新；旧安装的文件和
  「应用」列表里的卸载项清掉。
- **发布工作流**：Linux 上跑门禁，构建 EXE 与安装包。Windows runner 跑 Windows 包的 Go 测试、`--self-test`，
  以及用 Go 写的真实安装、升级、卸载验收（取代 Pester）。Go 依赖的许可检查取代 NuGet 许可检查。
  `scripts/verify-release.ps1` 的公开产物复验改成 Go 命令。
- **切换**：以上全部通过，并在目标机做完 `docs/testing/host-acceptance.md` 的全部检查后，发切换版。同一阶段删掉
  `src/`、`tests/`、`installer/`、`scripts/` 下的 PowerShell 脚本与 C# 夹具、`nuget.config`、
  `Directory.Build.props`、`.config/dotnet-tools.json`；把 README、CLAUDE.md 和 `docs/` 改写为 Go 版；
  系统要求去掉 .NET Framework。

## 风险

- **mygo 还在 0.x。** 接口还在变，主要由一人维护。应对：固定版本，升级单独做并重跑界面验收；遇到 mygo 的问题
  向上游报告，本仓先绕开。
- **文字度量。** 现版本按 GDI ascent 对齐 `≈`、单位与数字，mygo 在 Windows 上用 DirectWrite，对齐要按新引擎
  重做。阶段 3 的并排比对兜底。
- **安装包换成 NSIS。** 升级和卸载路径最容易出错。阶段 5 用真实安装的最后一个 C# 版本做升级测试。
- **COM 封装换成 go-ole。** 计划任务的读取和外来任务判定，要用现有用例全部覆盖。

## 进度

| 阶段                   | 状态   | 下一项                 |
| ---------------------- | ------ | ---------------------- |
| 0 可行性试点           | 未开始 | 窗口与托盘在目标机实测 |
| 1 纯逻辑核心           | 未开始 | 等阶段 0               |
| 2 Windows 数据源与诊断 | 未开始 | 等阶段 1               |
| 3 界面与托盘           | 未开始 | 等阶段 1               |
| 4 启动、提权与登录自启 | 未开始 | 等阶段 1               |
| 5 打包、发布与切换     | 未开始 | 等阶段 2 到 4          |
