# Windows 笔记本整机输入功率获取路径

> 历史研究记录（2026-08-05 实机验证版）：本文保存选型时的数据源调研与测量推理。术语与公式以 `GLOSSARY.md`（唯一术语表）和 README「功率口径」为准；本文「推荐的产品数据模型」一节是当时的建议，已被 GLOSSARY.md 取代——对照：充电口实测输入功率 → 整机输入功率（System Input Power）、平台功耗（不含电池充电）→ 平台功率（Platform Power）、市电侧输入功率 → 墙端输入功率（Wall Input Power）；平衡式以 README「功率口径」的写法为准。

## 结论

Windows 没有一个在普通笔记本上必然可用、直接返回“充电口边界整机输入功率”的通用 API。标准接口中，只有 **PMI/ACPI Power Meter** 或 **EMI** 在机器确实配有相应电表、固件/驱动又把正确边界暴露出来时，才可能给出该实测值；否则应报告 `Unsupported`，不能把电池功率、USB-PD 合同功率或软件估算标成 `System Input Power`。

**实机验证与精度边界（2026-09-22 修订）**：REDMI Book Pro 14 2025（Core Ultra 5 225H）上，Intel **Psys**（RAPL Platform 域，`MSR_PLATFORM_ENERGY_STATUS`，MSR `0x64D`）经官方签名的 [PawnIO](https://pawnio.eu/) 内核驱动已确认可读。历史 0.02–0.04 W 的交叉校验比较的是 MSR 与 EMI 的 **CPU 包功率**，用于检查 CPU 包的单位换算，不能证明 Psys 的整机覆盖范围和绝对精度。`整机输入 ≈ Psys + 电池端净功率（带符号）` 保留为组合估算，仍受传感器误差、采样时间差、平台覆盖和未测转换损耗影响。

2026-09-20 纯电池供电的 127.42 秒观察中，平台均值为 17.66 W，电池放电均值为 17.08 W，容量变化折算为 17.15 W。两源平均值相近；期间电池数值曾约 28 秒没有变化，不能将每秒读取当作每秒独立测量。2026-09-22 插电诊断在提权后读到约 60–67 W 的整机估算，与维护者电流表报告的约 60 W 属同一量级，但并非同步记录，不能据此宣称误差上限。下文保留历史研究过程，涉及精度的结论以本段和 `GLOSSARY.md` 的测量边界为准。

这里的目标边界应定义为：电能跨过笔记本 DC/USB-C 入口、进入内部 power path 之前的实时有功功率。它同时包含直接供给系统和流向电池的功率，但不包含外置适配器的转换损耗。

## 路径判定

完整判定表按接口类别拆成两页：[Windows 电池与电表接口](system-input-power-paths-windows.md)（电池 API、PMI/ACPI Power Meter、EMI、E3 等），以及 [USB-C、外设、OEM 与芯片路径](system-input-power-paths-hardware.md)（USB-PD、外置串联电表、OEM EC、Intel `Psys`、CPU/GPU 传感器）。

## REDMI Book Pro 14 2025 的在线证据

- 小米官方 `TM2411` [驱动页](https://www.mi.com/service/notebook/drivers/TM2411)确实提供“英特尔平台监控技术”包；包内是 `IntcPMT.sys` 3.1.2.6，并匹配 Arrow Lake/Meteor Lake 的 `PCI\VEN_8086&DEV_7D0D`、`PCI\VEN_8086&DEV_AD0D`。这证明该机型系列准备了 Intel PMT 传输层，但 Intel 对 PMT 的公开定义只是通用遥测框架，可能用于分析功耗问题，并未承诺其中存在 DC-in/VBUS 实时功率字段，也没有公开普通应用可直接调用的该字段 API。[Intel PMT specification](https://www.intel.com/content/www/us/en/content-details/710389/intel-platform-monitoring-technology-intel-pmt-technical-specification.html)
- 上游 Linux 的 [REDMI WMI 驱动](https://github.com/torvalds/linux/blob/master/drivers/platform/x86/redmi-wmi.c)目前只处理 OEM 功能/事件；没有公开整机瓦数读取。**2026-08-05 测试机 实机 + 一手来源调研双重证实**：小米 EC/WMI（Bitland MIFS）确实没有把充电芯片的输入电流、电压或功率暴露给 Windows——不再是"尚不能证明"，而是**证伪**（`MIFS` 未注册任何 WMI GUID，`EcIoSdk.dll` 仅有日志接口，两个独立逆向项目全命令表扫描均未发现功率寄存器）。
- 该机型最值得实机验证的顺序：标准 `ACPI000D`/PMI → EMI 的入口总 rail → `Psys` (`0x64D`) → PMT 元数据/通道，**已于 2026-08-05 在 测试机 上完整执行**。结果：`ACPI000D`/PMI 均证伪（不存在或为存根）；EMI 存在但只有 4 条 CPU 内部 RAPL 通道，无入口总 rail；`Psys`（`0x64D`）**实测确认可读**，经 PawnIO 驱动给出真实平台功率；PMT 驱动虽在运行，但暴露的是未公开的私有接口 GUID，无公开用户态 API，判定不可作为可发布产品依赖。

## 开源与现成方案横向比较

已有开源实现与外置电表、智能插座等现成方案的逐项比较已拆到 [开源与现成方案横向比较](system-input-power-alternatives.md)。

## 推荐的产品数据模型（历史建议，术语已由 GLOSSARY.md 取代）

不要让一个“功率”标签承载不同物理量。建议固定使用以下专业术语：

| 中文 UI 名称                 | 英文/内部名                              | 定义                                                                                                                                                                                                                                                                                                                                              |
| ---------------------------- | ---------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **电池端净功率**             | Net battery-terminal power               | 电池驱动在电池端子边界报告的带符号净功率：充电为正，放电为负。它不等于电芯实际储能速率，也不包含主板功耗。                                                                                                                                                                                                                                        |
| **平台功耗（不含电池充电）** | Platform power / Intel Psys              | 处理器与其余平台的 Psys 总功耗；Intel 明确排除电池充电。**测量口径：Measured**（读 `MSR_PLATFORM_ENERGY_STATUS`/`0x64D`，经 PawnIO）。目标机 2026-08-05 实测：空闲 23.87 W、满载 46.88 W，与 EMI PKG 交叉校验偏差 0.02–0.04 W。“不含电池充电”**已实测确认**：空闲时 `PSYS − PKG = 11.67 W` < 同期电池端充电功率 13.27 W，若含充电该差值必然更大。 |
| **估算整机输入功率**         | Estimated system input power             | `Psys + 电池端净功率（带符号）`。**测量口径：Estimated**（两个 Measured 量的组合，不是入口电表直接实测）。电池补充适配器时该项为负；未测项仍包括充电与转换路径损耗（典型 5–10%）。目标机 2026-08-05 实测：空闲 37.14 W、14 核满载 60.14 W。                                                                                                       |
| **充电口实测输入功率**       | Measured DC input power                  | 串联 USB-C/DC 电表在笔记本入口测得的实时 `V × I`；这是维护者所说“充电口功率”的正确名称。                                                                                                                                                                                                                                                          |
| **USB-PD 协商功率**          | USB-PD negotiated power / contract limit | 协商合同的请求值或上限，不是当前实际流过的功率；只能作为连接信息展示。                                                                                                                                                                                                                                                                            |
| **市电侧输入功率**           | AC wall input power                      | 智能插座/UPS 在适配器之前测得，包含适配器损耗。                                                                                                                                                                                                                                                                                                   |

近似功率平衡为：

```text
充电口实测输入功率
  = 平台功耗
  + 电池端净功率（带符号）
  + 充电与内部电源路径损耗

市电侧输入功率
  = 充电口实测输入功率
  + 外置适配器损耗
```

因此 `Psys + 电池端净功率（带符号）` 应显示为“估算”，不能把负的电池补充功率
截成 0，也不能把缺失的损耗项暗中当作 0。

## 产品数据源范围

1. **便携 EXE 与安装包使用同一个应用程序集。** `BatterySensor` 把电池读数归一为带符号的“电池端净功率”，`EmiSensor` 在标准用户权限下读取 EMI `RAPL_Package0_PKG` 并显示“CPU 包功率”。
2. **Psys 是可选数据源。** `PawnIoSensor` 通过用户另行安装的官方 PawnIO 驱动读取平台计数器，并在 EMI 可用时用 CPU 包计数器交叉校验单位。`PowerSnapshot.Compose` 在外部供电时按“平台功率 + 电池端净功率（带符号）”生成“估算整机输入功率”；未安装驱动、未提权、计数器不推进或校验失败时会保持不可用并给出原因。应用本身不安装驱动。
3. **直接入口电表与 PD 合同 provider 尚未实现。** PMI/OEM 入口电表只有在测量边界可证明时才能新增；PD 合同只能作为连接信息，软件模型必须标为 `Estimated` 并附带校准时间、数据源和置信度。

产品使用机内软件路径，外置硬件 provider 不在产品范围内。维护者已有的电流表可用于验收对照。

推荐所有 provider 返回统一元数据：`valueW`、`boundary`、`measurementKind` (`Measured/Estimated/Contract`)、`source`、`sampleWindow`、`timestamp`、`accuracy/quality`。UI 再按边界决定名称，而不是让 provider 自己都声称返回“整机功率”。

## 可靠 provider 的实现门槛

1. **只接受可证明的物理边界。** PMI 必须同时满足 measurement 支持、`MeterType=input`、整机关联（或 systemwide `MeteredHardware`）；EMI/OEM 通道必须由固件或 OEM 资料明确为 DC jack/VBUS 入口总 rail。仅凭通道名猜测不够。
2. **能力发现而非假定存在。** 先枚举 `Win32_PowerMeter`/PMI 与 `GUID_DEVICE_ENERGY_METER`；均无合格实例时返回 `Unsupported`。型号专用 EC provider 必须白名单到明确的硬件/固件协议版本。
3. **按接口正确换算。** PMI 应应用 `BaseUnits`、`UnitModifier`、平均窗口和 accuracy；EMI 用两次累计能量与时间差计算平均 W，并处理计数器回绕、重置、时间倒退和采样窗口。
4. **处理多入口与混合供电。** 多个 USB-C/DC 输入只能在确认各通道互不重叠且处于同一边界后求和；电池同时充/放电不改变“入口电表”为事实源的原则。
5. **暴露数据质量。** 返回来源、`Measured/Estimated`、边界、采样时刻/窗口、精度和 stale/unknown 状态。E3、PD 合同、额定功率、CPU/GPU 之和与 `BATTERY_STATUS.Rate` 可单独展示，但不得降级冒充 `System Input Power`。

## 最终判断

可以写一个“**发现标准硬件电表则工作**”的 Windows provider，但不能写一个“**所有 Windows 笔记本都能得到整机入口瓦数**”的 provider。要可靠支持目标机器，至少需要以下之一：

- OEM 在充电入口总 rail 上布置电表，并通过 PMI/ACPI Power Meter 或 EMI 正确暴露；
- OEM/芯片厂提供有文档、可安全调用的 EC/充电芯片遥测接口与 Windows 驱动。

没有这些硬件与暴露层时，软件无法从电池净充放电率或 PD 协商值唯一反推出充电口实时输入功率。

**当前实现**：目标机的标准接口不提供 `Measured System Input Power`。应用通过 `Psys + 电池端净功率（带符号）` **估算**整机输入；两个分量均为 Measured，组合精度受各数据源、采样时间差和未测损耗影响。电池补充适配器时净功率为负，必须从 Psys 中扣除。组合结果始终按 `Estimated` 标注，不会因缺少入口电表而冒充实测值。
