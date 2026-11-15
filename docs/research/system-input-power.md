# Windows 笔记本整机输入功率获取路径

术语与公式以 `GLOSSARY.md` 为准。本页记录各数据源能否给出整机输入功率，以及在目标机 REDMI Book Pro 14 2025（Core Ultra 5 225H，小米机型系列 `TM2411`）上的实测结论。新增或更换数据源前先读本页。

## 结论

- Windows 没有在普通笔记本上必然可用、直接返回充电口边界整机输入功率的通用 API。只有 PMI/ACPI Power Meter 或 EMI 在机器确实配有入口电表、固件或驱动又暴露了正确边界时，才可能给出实测值；否则报告不支持，不能把电池功率、USB-PD 合同功率或软件估算标成整机输入功率。
- 目标边界是电能跨过笔记本 DC/USB-C 入口、进入内部 power path 之前的实时有功功率：包含直接供给系统和流向电池的功率，不包含外置适配器的转换损耗。
- 目标机的标准接口不提供实测整机输入功率。产品用 `Psys + 电池端净功率（带符号）` 估算；电池补充适配器时净功率为负，必须从 Psys 中扣除，不截成零；结果始终标为估算。
- 外置硬件 provider（串联电表、智能插座）不在产品范围内；维护者已有的电流表只用于验收对照。

## 目标机实测

按 `ACPI000D`/PMI → EMI → Psys → PMT 的顺序逐项验证过：

- `ACPI000D` 与 PMI：不存在或只是存根。
- EMI：标准用户权限（`RunLevel Limited`，即双击 EXE 的场景）下可完整读取，不需要驱动或管理员；只暴露 `RAPL_Package0_PKG`、`DRAM`、`PP0`、`PP1` 4 条 CPU 内部通道，没有 DC-in 或 Psys 通道，边界止于 CPU 封装，只能提供 CPU 包功率。
- Intel Psys（RAPL Platform 域，`MSR_PLATFORM_ENERGY_STATUS`，MSR `0x64D`）：这块主板布线并启用了 Psys，经官方签名的 [PawnIO](https://pawnio.eu/) 驱动可读，计数器持续推进。
- Psys 不含电池充电：空闲时 `PSYS − PKG = 11.67 W`，小于同期电池端充电功率 13.27 W；若充电功率计入 Psys，该差值必然更大。
- PMT：小米 `TM2411` [驱动页](https://www.mi.com/service/notebook/drivers/TM2411)的「英特尔平台监控技术」包是 `IntcPMT.sys` 3.1.2.6，匹配 `PCI\VEN_8086&DEV_7D0D`、`PCI\VEN_8086&DEV_AD0D`。驱动在运行，但暴露的是未公开的私有接口 GUID，没有公开的用户态 API；Intel 对 PMT 的[公开定义](https://www.intel.com/content/www/us/en/content-details/710389/intel-platform-monitoring-technology-intel-pmt-technical-specification.html)只是通用遥测框架，没有承诺 DC-in/VBUS 实时功率字段。不能作为可发布产品的依赖。
- 小米 EC/WMI（Bitland MIFS）不暴露充电芯片的输入电流、电压或功率：`ACPI\PNP0C14\MIFS` 未注册任何 WMI GUID；`EcIoSdk.dll` 只导出 EC 日志接口（`GetECLog`/`GetECLogPageCount`/`FreeECLog`/`RegisterECLogReadyNotifier`），不含寄存器读取。两个独立逆向项目 [`Oksion/XiControl`](https://github.com/Oksion/XiControl)（同代 TM24 系列机型全命令表 `0x01–0x30` 扫描）与 Linux 主线 [`bitland-mifs-wmi`](https://github.com/torvalds/linux/blob/master/drivers/platform/x86/bitland-mifs-wmi.c) 结论一致：唯一近似信号是 PD 适配器**额定**功率（cmd `0x10`/arg `0x06`），不是实时功耗。上游 Linux [REDMI WMI 驱动](https://github.com/torvalds/linux/blob/master/drivers/platform/x86/redmi-wmi.c)也只处理 OEM 功能与事件。其他 OEM 平台不能据此类推，要按各自 EC 协议逐一核实。

## 精度边界

- 0.02–0.04 W 的交叉校验比较的是 MSR 与 EMI 的 **CPU 包功率**，只证明 CPU 包的单位换算，不能证明 Psys 的整机覆盖范围和绝对精度。
- 纯电池供电的 127.42 秒观察：平台均值 17.66 W，电池放电均值 17.08 W，容量变化折算 17.15 W，三者相近。期间电池读数曾约 28 秒不变，每秒读取不能当作每秒独立测量。
- 插电提权后的 `--power-probe` 读到电池充电 27.98 W、平台 31.80–46.08 W、估算整机输入 59.78–74.06 W；另一次诊断的整机估算约 60–67 W，与维护者电流表报告的约 60 W 属同一量级。这些都不是同步记录，只证明数据通路和公式，不能据此宣称误差上限。
- 估算还受传感器误差、采样时间差、平台覆盖范围和未测的充电路径与转换损耗影响。

## 各路径判定

| 路径                                                          | 返回的量与边界                                                                                                                                                                                                                                                                                | 能否得到整机输入                                                                              |
| ------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------- |
| `BATTERY_STATUS`、Battery WMI、`SYSTEM_BATTERY_STATE`         | 电池端子边界的充放电率，可能未知或只是相对单位（[`BATTERY_STATUS`](https://learn.microsoft.com/en-us/windows/win32/power/battery-status-str)）                                                                                                                                                | 否。不含系统从适配器直接取走的功率；满电时 `Rate≈0` 不代表整机输入为 0                        |
| `Win32_Battery`、`GetSystemPowerStatus`、WinRT `PowerManager` | 电量、预计续航、供电状态，没有瓦数                                                                                                                                                                                                                                                            | 否                                                                                            |
| PMI / `Win32_PowerMeter`                                      | 硬件电表功率；`MeterType=0` 表示相对被测对象的输入功率，系统级电表的 `MeteredHardware` 为空（[PMI](https://learn.microsoft.com/en-us/windows-hardware/drivers/powermeter/power-meter-interface)）                                                                                             | 有条件。须 `MeterType=input`、电表覆盖整机、物理位置就是入口；「input」本身不保证位于充电口   |
| ACPI `ACPI000D` Power Meter                                   | `_PMM` 有功功率，`_PMC` 声明 input/output 与精度，`_PMD` 声明被测设备（[ACPI 6.5 §10.4](https://uefi.org/specs/ACPI/6.5/10_Power_Source_and_Power_Meter_Devices.html#power-meters)）                                                                                                          | 有条件，判据同 PMI；ACPI 不要求每台笔记本都有，也不固定传感器位置                             |
| ACPI AC Adapter `_PSR` / `_PIF`                               | `_PSR` 只有 online/offline；`_PIF` 的最大输入/输出功率是额定值                                                                                                                                                                                                                                | 否                                                                                            |
| EMI（`GUID_DEVICE_ENERGY_METER`）                             | 某条 rail 的累计能量，两次差分得区间平均功率（[EMI](https://learn.microsoft.com/en-us/windows-hardware/drivers/powermeter/energy-meter-interface)）                                                                                                                                           | 有条件。只有 OEM 文档确认为入口总 rail 的通道可用；CPU/GPU/显示等子系统 rail 求和会漏算或重复 |
| E3、SRUM、`powercfg` 报告、PerfMon                            | 进程能耗归因与诊断，默认是软件估算；PerfMon 的来源仍是 PMI                                                                                                                                                                                                                                    | 否，没有独立于 EMI/PMI 的能力                                                                 |
| USB-PD PDO、RDO、UCSI                                         | 能力与合同请求值或上限（[UCSI](https://learn.microsoft.com/en-us/windows-hardware/drivers/usbcon/ucsi)）                                                                                                                                                                                      | 否，合同值不能当实际输入                                                                      |
| USB-PD PPS Status                                             | Source 输出侧的电压/电流状态                                                                                                                                                                                                                                                                  | 不通用：只在 PPS 会话有效，隔着线缆，Windows 没有面向普通应用的接口；UCSI 测试接口默认禁用    |
| USB HID Power Device、外置串联电表                            | 实测 `Voltage`、`Current`、`ActivePower`                                                                                                                                                                                                                                                      | 借助外设可以，不在产品范围                                                                    |
| OEM EC、充电芯片、厂商 SDK                                    | 充电芯片可能有 VBUS 与输入电流 ADC（如 [TI BQ25790](https://www.ti.com/product/BQ25790)）                                                                                                                                                                                                     | 需 OEM 协议与安全的读取通道；小米平台已证伪（见上）                                           |
| Intel Psys / RAPL platform domain                             | 处理器加主板路由进该计数器的平台功率，由整机厂选择实现、默认关闭，不含电池充电（[Intel datasheet](https://edc.intel.com/content/www/us/en/design/products-and-solutions/processors-and-chipsets/core-ultra-200h-and-200u-series-processors-datasheet-volume-1-of-2/platform-power-control/)） | 不是充电口输入；加上带符号的电池端净功率是近似值，还缺充电路径和内部 DC/DC 损耗               |
| CPU/GPU 厂商传感器（RAPL、NVML）                              | 组件功率                                                                                                                                                                                                                                                                                      | 否。相加仍缺显示、内存、存储、风扇与转换损耗，也可能重叠                                      |

## 可靠 provider 的门槛

1. 只接受可证明的物理边界。PMI 须同时满足 measurement 支持、`MeterType=input`、整机关联（或 systemwide `MeteredHardware`）；EMI/OEM 通道须由固件或 OEM 资料明确为 DC jack/VBUS 入口总 rail，仅凭通道名猜测不够。
2. 能力发现而非假定存在。先枚举 `Win32_PowerMeter`/PMI 与 `GUID_DEVICE_ENERGY_METER`，均无合格实例时返回不支持；型号专用 EC provider 须白名单到明确的硬件/固件协议版本。
3. 按接口正确换算。PMI 应用 `BaseUnits`、`UnitModifier`、平均窗口和 accuracy；EMI 用两次累计能量与时间差计算平均功率，并处理计数器回绕、重置、时间倒退和采样窗口。
4. 多个 USB-C/DC 输入只能在确认各通道互不重叠且处于同一边界后求和。
5. E3、PD 合同、额定功率、CPU/GPU 之和与 `BATTERY_STATUS.Rate` 可以单独展示，但不得冒充整机输入功率。

## 现成方案与驱动选择

- Windows 上读 MSR 必须经内核驱动。Intel PCM 的 `msr.sys` 与 WinPowerMonitor 所用 Hubblo RAPL 驱动的公开版本是测试签名，正常 Windows 要开 test-signing，不能作为发行依赖。
- LibreHardwareMonitor 把 `0x64D` 显示为 `CPU Platform`；PawnIO 的 `IntelMSR` 模块已把该寄存器列入白名单。PawnIO 官方驱动有数字签名，需要用户以管理员单独安装一次（用官方签名安装包，不用 `-unrestricted` 无签名版），本程序不代为安装。
- HWiNFO shared memory 免费版有连续运行时限，打包、嵌入或商用需要授权；没有总功率硬件时它也不能凭软件造出入口传感器。
- BatteryMonitor-Windows 里名为 `Psys` 的字段只是用户手填的基线，不是 Intel Psys 计数器。
- 外置表（TC66、FNIRSI FNB58、AVHzY CT-3/C3、POWER-Z KM003C、Dr. PD 等）能实测 USB-C V/I/W；智能插座与 UPS（Shelly、Tasmota、NUT）测的是墙端输入功率，含适配器损耗。两类都不在产品范围。AVHzY 通信协议未公开、库的再分发授权未核实；KM003C 的逆向工具要用 Zadig 把设备驱动换成 WinUSB。
- PowerAPI、Joulemeter 类软件模型只能给出估算：插电时操作系统观测不到真实标签，亮度、风扇、外设、温度与电源效率都会让模型漂移。
- 商业应用「USB Connection Information」的 PD 电压/电流/功率只支持 macOS，不能说明普通 Win32 应用能读实时 PD 电流。
