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
  命令行开关与退出码、受保护副本路径及 `docs/testing/protected-autostart.md` 的不变量全部沿用。重写期间不顺带
  改功能；名字与路径、界面外观例外，见下两条。
- **界面按 COUI 17（ColorOS 17）重画。** 不照搬 C# 版的深色仪表外观：窗口跟随系统深浅色，内容放在卡片里。
  整窗是透明无边框窗口里自绘的面板，面板与卡片都是连续曲率的同心圆角（面板 32、卡片 20、间距 12）；
  字体全用内嵌的 MiSans；只用一种蓝色表示数据和操作，琥珀色只用于提醒，红色只用于传感器出错。口径切换是
  读数卡片左上角的页签，不用分段胶囊。显示的内容和规则不变，只换外观与排布；外观由维护者看预览图定。
- **新名字。** Go 版只用新名字，不读旧版的设置、任务和副本：显示名 Power Meter（中文界面「功率计」），程序
  `PowerMeter.exe`，设置在 `HKCU\Software\PowerMeter`，自启任务 `PowerMeter.Logon.<SID>`，安装目录
  `%LOCALAPPDATA%\Programs\Power Meter`。
- **数据源与现版本相同。** 电池读 WMI `root\WMI` 的 `BatteryStatus`；CPU 包功率读 EMI；平台功率经 PawnIO 的
  `IntelMSR` 模块读 Psys。WMI 与任务计划程序这两处 COM 调用用纯 Go 的 `github.com/go-ole/go-ole`。
- **切换方式。** 重写期间 C# 版是唯一的发布物，发版流程不变。Go 版通过全部阶段验收后，用一次发版切换，并在同一
  阶段删掉 C# 代码、PowerShell 脚本、Inno Setup 定义和 .NET 配置。切换版的版本档位按 `docs/release.md` 由维护者
  确认。
- **两边一致。** 重写期间 C# 版若改了行为，同一改动也要进 Go 版，切换前两边一致。
- **固定 mygo 版本。** mygo 在 `go.mod` 里固定版本，命令行工具用 `go tool mygo`。升级 mygo 单独做，升级后重跑
  阶段 3 的界面验收。
- **旧版。** 安装包发现旧版（C# 写的 1.5 及更早版本，装在 `Battery Charge Meter` 目录）还装着就停下，提示先在「设置 > 应用」
  卸载旧版；旧卸载器会清掉它的自启任务和副本。不写迁移代码，设置不迁移，自启在新版里重新勾选。
- **切换版本号。** 切换版发 v1.6.0（minor）。
- **发布目标与资产。** 仍只支持 Windows 11 x64（`windows/amd64`）。资产名不变：
  `PowerMeter-vX.Y.Z-windows.exe`、`PowerMeter-vX.Y.Z-windows-setup.exe`、第三方通知、PawnIO 对应源码包，
  各带 `.sha256`。

## 不在本次范围

- mygo 的自动更新。受保护副本只能由提权进程刷新，它与自动更新怎样配合另议。
- 代码签名、Windows on Arm 版、网页界面。

## 代码布局

- Go 模块与 `mygo.json` 在仓根，程序入口在 `cmd/powermeter/`，产物名 `PowerMeter.exe`。
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

- 按 COUI 17 重画窗口：自绘标题栏（应用图标、钉住、最小化、关闭）、显示口径分段控件、主读数卡（口径名、
  `≈` 与单位按基线对齐的主读数、供电状态胶囊、电池电压、估算电流、电量与电池小图）、走势卡（30 分钟均值线、
  最小最大值带、渐变面积、峰值标记、当前值光标、缺失处断线，下方 10 分钟均值与 5 分钟峰值）、其余三项口径的
  列表卡、诊断提示卡与管理员重启、设置弹层（登录自启开关、语言、当前权限与管理员重启）。浅色深色跟随系统。
- 托盘图标、悬停文字和菜单；系统默认、中文、英文三种语言即时切换。
- `--screenshot`、`--dpi-preview`、`--tray-preview` 用 `ui.Render` 重做。
- **验收**：界面测试在门禁里用 C# 版同一份夹具的八个读数场景，核对文字不出窗口、互不重叠、英文无未翻译文字、
  按钮与设置的操作到达程序；在 Windows 上把各场景按中英文、浅深色、100% 与 175% 渲染成图，交维护者看过；
  `docs/testing/host-acceptance.md`「Interactive acceptance」第 1 到 5 条在目标机通过。

### 阶段 4：启动、提权与登录自启

- 启动路由与提权：只请求一次 UAC，取消后普通运行，窗口里有重新请求的按钮。
- 单实例与唤回：跨提权级别唤回已有窗口，按来源路径认出自启实例，重复的自启启动直接退出。mygo 的
  `RequestSingleInstanceLock` 满足不了这些时自己实现。
- 计划任务的注册、读取与任务 ACL；受保护副本的复制、DACL 与所有者、`source.txt`；`--sync-autostart` 与
  `--remove-autostart` 的退出码 0、1、3、4、5、6；同名的外来任务不修改、不接管。
- **验收**：`docs/testing/protected-autostart.md` 的不变量与「Host results」各项在目标机重跑通过；
  「Interactive acceptance」第 7、8 条通过。

### 阶段 5：打包、发布与切换

- **安装包**：改用 mygo 生成的 NSIS 当前用户安装包。`windows.installerInclude` 的 `customInstall` 写出第三方
  通知、跑 `--sync-autostart`，需要时请求一次 UAC；卸载开始时（`un.onInit`）跑 `--remove-autostart`，再请求
  一次 UAC 删副本。
  取消、失败和外来任务的处理与现版本相同。
- **旧版拦截**：`customInit` 发现旧版的卸载项还在就停下，提示先卸载旧版。
- **发布工作流**：Linux 上跑门禁，构建 EXE 与安装包。Windows runner 跑 Windows 包的 Go 测试、`--self-test`，
  以及用 Go 写的真实安装、升级、卸载验收（`tools/release install-test`，取代 Pester）。Go 依赖的许可检查
  （`third_party` 的测试）取代 NuGet 许可检查。公开产物复验改成 `tools/release verify`。
- **切换**：以上全部通过，并在目标机做完 `docs/testing/host-acceptance.md` 的全部检查后，发切换版。需要本人在笔记本前
  操作的项（UAC 的取消与确认、插拔电源、睡眠唤醒、改显示缩放）在其余验收做完后集中做一次。发版后把目标机上的
  旧版卸载、装上切换版并重新启用自启。同一阶段删掉
  `src/`、`tests/`、`installer/`、`scripts/` 下的 PowerShell 脚本与 C# 夹具、`nuget.config`、
  `Directory.Build.props`、`.config/dotnet-tools.json`；把 README、CLAUDE.md 和 `docs/` 改写为 Go 版；
  系统要求去掉 .NET Framework。

## 风险

- **mygo 还在 0.x。** 接口还在变，主要由一人维护。应对：固定版本，升级单独做并重跑界面验收；遇到 mygo 的问题
  向上游报告，本仓先绕开。
- **文字度量。** `≈`、数字与单位按基线对齐，靠 mygo 的 DirectWrite 度量；Linux 门禁用的替代字体与 Windows
  不同，排布以 Windows 上渲染的预览为准。
- **安装包换成 NSIS。** 升级和卸载路径最容易出错。阶段 5 用 Go 版的两个版本做覆盖升级测试，并用真实安装的
  最后一个 C# 版本测旧版拦截。
- **COM 封装换成 go-ole。** 计划任务的读取和外来任务判定，要用现有用例全部覆盖。

## 进度

| 阶段                   | 状态                            | 下一项                                            |
| ---------------------- | ------------------------------- | ------------------------------------------------- |
| 0 可行性试点           | 完成                            |                                                   |
| 1 纯逻辑核心           | 完成                            |                                                   |
| 2 Windows 数据源与诊断 | 纯电池时两种权限与 1.5 对照一致 | 插电时的对照并入集中验收                          |
| 3 界面与托盘           | 外观经维护者看预览图逐项定下    | 维护者确认最终外观；交互验收并入集中验收          |
| 4 启动、提权与登录自启 | 代码完成，实机测试在 CI 跑      | 不变量与「Host results」并入集中验收              |
| 5 打包、发布与切换     | 工作流与文档已改为 Go 版        | Windows 验证工作流通过后合入，发 v1.6.0，集中验收 |
