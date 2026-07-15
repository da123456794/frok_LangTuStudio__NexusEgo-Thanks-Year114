# NexusEgo

NexusEgo 是一个面向《我的世界》网易版租赁服的建筑导入/导出工具，核心用途是让机器人进入服务器后，自动完成结构导入、区域导出，以及部分结构格式转换。

当前仓库是完整源码工程，主程序入口为 `cmd/nexusego`，语言为 Go。

## 功能概览

- 导入建筑到网易版租赁服
- 从租赁服导出区域为 `.mcworld` 或 `.nexus`
- 支持交互式任务创建
- 支持 CLI 直接创建并执行导入/导出任务
- 自动缓存 Token 到 `NexusEgo_Storage/token.fsm`
- 非 `.mcworld` 输入可先转换为 `.mcworld` 再导入
- 支持图片转 `.mcworld`
- 支持 MIDI 转 `.mcworld`
- 支持导入后修补模式

## 项目结构

```text
.
├─ cmd/
│  └─ nexusego/                 主程序入口
├─ control/                     任务装配、交互流程、存储目录管理
├─ constants/                   常量定义
├─ defines/                     任务与结构基础类型
├─ utils/                       导入、导出、转换、客户端连接等核心实现
├─ modules/
│  ├─ WaterStructure/           结构格式解析与转换能力
│  └─ RaaBel/     NBT 处理器依赖子模块
├─ modules/Conbit/      底层协议/连接相关依赖
├─ NexusEgo_Storage/            运行期数据目录
├─ build.bat                    多平台构建脚本
└─ CLI命令行参数.md             CLI 参数说明
```

## 环境要求

- Go `1.26.0`
- Windows / Linux / macOS 均可构建
- 可访问认证服务 `https://studio.aurelrune.com/`
- 能连接目标网易版租赁服
- 导入或导出时，机器人需要在服内获得 OP 权限

## 快速开始

### 1. 安装依赖

```powershell
go mod download
```

### 2. 直接运行

```powershell
go run ./cmd/nexusego
```

程序会进入交互模式，自动创建任务，必要时提示输入 Token、租赁服号、结构文件、坐标等信息。

### 3. CLI 方式运行

导入：

```powershell
go run ./cmd/nexusego `
  -token YOUR_TOKEN `
  -mode import `
  -server 123456 `
  -password YOUR_PASSWORD `
  -file demo.mcworld `
  -x 0 -y 64 -z 0 `
  -dimension overworld `
  -speed 2000 `
  -usefill=true `
  -region 5 `
  -importnbt=true `
  -importcmd=true `
  -cmdspeed 11 `
  -clear=false `
  -deny=false `
  -border=false `
  -closecmd=true `
  -fix=false `
  -progress 0 `
  -croparea "0 0 0 100 100 100"
```

导出：

```powershell
go run ./cmd/nexusego `
  -token YOUR_TOKEN `
  -mode export `
  -server 123456 `
  -password YOUR_PASSWORD `
  -dimension overworld `
  -exportarea "0 0 0 100 100 100" `
  -exportfile backup.mcworld
```

导出为 Nexus：

```powershell
go run ./cmd/nexusego `
  -token YOUR_TOKEN `
  -mode export `
  -server 123456 `
  -dimension overworld `
  -exportarea "0 0 0 100 100 100" `
  -exportfile backup.nexus `
  -exportauthor "yourname" `
  -exportpassword "yourpassword"
```

## 运行模式

### 交互模式

不传 `-mode` 时，程序进入交互流程：

- 自动验证或读取本地缓存 Token
- 检查 `NexusEgo_Storage/task/` 中已有任务
- 若无任务，则引导创建导入或导出任务
- 若有任务，则可直接选择执行

### CLI 模式

传入 `-mode=import` 或 `-mode=export` 时，程序会：

- 根据命令行参数生成任务文件
- 打印任务摘要
- 直接执行任务

## 存储目录说明

程序运行时默认使用 `NexusEgo_Storage/` 目录：

- `NexusEgo_Storage/file/`：结构文件、导出文件默认存放位置
- `NexusEgo_Storage/task/`：任务文件
- `NexusEgo_Storage/log/`：日志目录
- `NexusEgo_Storage/token.fsm`：本地加密保存的 Token

相对路径规则：

- 导入只写文件名时，优先读取 `NexusEgo_Storage/file/`，找不到再读取程序根目录
- 导出只写文件名时，默认落到 `NexusEgo_Storage/file/`
- 写成 `file/demo.mcworld` 时，也会解析到 `NexusEgo_Storage/file/demo.mcworld`
- 导入写成其他相对路径时，如果程序运行目录下存在该路径会直接读取，否则拼接到 `NexusEgo_Storage/` 下
- 导出写成其他相对路径时，会拼接到 `NexusEgo_Storage/` 下
- 绝对路径会直接使用

## 输入与输出格式

### 导入输入

主流程最终只导入 `.mcworld`，但源码会在导入前尝试把其他结构格式先转换为 `.mcworld`。

当前工程确认支持的来源包括：

- `.mcworld`
- `.nexus`
- `.schematic`
- `.schem`
- `.litematic`
- `.mcstructure`
- `.bdx`
- `.construction`
- `.kbdx`
- `.tibi`
- `.ibi`
- `.mcfunction`
- `.txt`
- `.json`
- `.building`
- `.buildingX`
- `.reb`
- `.fhbuild`
- `.bds`
- `.bcf`
- `.covstructure`
- `.np`
- 图片格式：`.png` `.jpg` `.jpeg` `.bmp` `.webp` `.gif`
- MIDI：`.mid` `.midi`

说明：

- 不同格式的成熟度不完全一致，具体成功率取决于 `WaterStructure` 中对应解析器实现
- `.nexus` 源文件在转换时，如有密码，需要手动输入密码
- 部分格式虽然已注册解析器，但不代表所有样例都能无损转换

### 导出输出

导出阶段支持：

- `.mcworld`
- `.nexus`

当导出为 `.nexus` 时，程序会先导出 `.mcworld`，再转换成 `.nexus`。

## CLI 参数摘要

更完整的参数说明见 [CLI命令行参数.md](./CLI命令行参数.md)。

### 通用参数

| 参数 | 说明 |
| --- | --- |
| `-token` | API Token，可省略，省略后启动时手动输入 |
| `-mode` | 运行模式：`import` / `export` |
| `-server` | 租赁服号 |
| `-password` | 租赁服密码 |
| `-dimension` | 维度 |

### 导入参数

| 参数 | 说明 |
| --- | --- |
| `-file` | 导入文件 |
| `-x -y -z` | 导入起点坐标 |
| `-speed` | 导入速度，默认 `2000` |
| `-usefill` | 是否启用增量导入 |
| `-region` | 增量导入边长 |
| `-importnbt` | 是否导入 NBT |
| `-importcmd` | 是否导入命令方块数据 |
| `-cmdspeed` | 命令方块写入速度 |
| `-clear` | 是否清理导入区域 |
| `-deny` | 是否自动铺设拒绝方块 |
| `-border` | 是否自动铺设边界方块 |
| `-closecmd` | 导入前是否关闭命令方块启用 |
| `-fix` | 是否直接进入修补模式 |
| `-progress` | 起始进度百分比，范围 `0-100` |
| `-croparea` | 裁剪范围，格式：`"x1 y1 z1 x2 y2 z2"` |

### 导出参数

| 参数 | 说明 |
| --- | --- |
| `-exportfile` | 导出文件名 |
| `-exportarea` | 导出范围，格式：`"x1 y1 z1 x2 y2 z2"` |
| `-exportauthor` | 导出 `.nexus` 时的作者名 |
| `-exportpassword` | 导出 `.nexus` 时的密码 |

## 维度写法

支持以下写法：

- 空值：`overworld`
- `overworld` / `ow` / `0`
- `nether` / `the_nether` / `1`
- `the_end` / `end` / `2`
- `dm` / `3`
- 自定义维度：`名称:ID`

示例：

```powershell
-dimension overworld
-dimension nether
-dimension the_end
-dimension dm
-dimension custom:7
```

注意：

- 单独写 `4` 这类未知数值不会被当作自定义维度
- 自定义维度应写成 `name:id` 格式

## 导入流程说明

导入任务的大致流程如下：

1. 解析任务与 Token
2. 连接租赁服
3. 等待机器人获得 OP
4. 若输入文件不是 `.mcworld`，先转换为 `.mcworld`
5. 根据参数决定是否裁剪、清空区域、增量导入、导入 NBT、导入命令方块
6. 导入完成后可进入修补模式

如果导入过程需要 NBT 或命令方块处理，程序会启动额外处理流程。

## 导出流程说明

导出任务的大致流程如下：

1. 连接租赁服
2. 等待机器人获得 OP
3. 读取目标区域
4. 导出为 `.mcworld`
5. 若目标后缀为 `.nexus`，继续执行 `.mcworld -> .nexus` 转换

## 内置终端命令

程序连接服务器后，终端中支持一些内置命令：

- `help`：显示帮助
- `exit`：退出程序
- `cdump`：操作导入相关命令
- `cexport`：导出区域到 `mcworld/nexus`
- `/xxx`：直接发送 MC 指令
- `.xxx`：发送并等待返回的 MC 指令

## 构建

### 普通构建

```powershell
go build -o NexusEgo.exe ./cmd/nexusego
```

### 使用仓库脚本构建

仓库提供了 `build.bat`，会尝试构建：

- android arm64
- linux amd64 / arm64
- windows amd64 / arm64
- darwin amd64 / arm64

脚本依赖：

- `garble`
- 可选 `upx`

执行：

```powershell
./build.bat
```

产物默认输出到 `NexusEgo_Storage/dist/`。

## 注意事项

- 首次运行需要有效 Token
- 导入和导出都依赖机器人成功进服
- 机器人必须在目标服中获得 OP
- 某些结构格式会先转换到 `.mcworld`，耗时可能较长
- `.nexus` 导出依赖从 `mcworld` 名称或世界名中解析边界信息
- 当前仓库内存在部分历史中文编码残留，但不影响本 README 的 UTF-8 内容

## 相关路径

- 主入口：`cmd/nexusego/main.go`
- CLI 参数：`cmd/args/args.go`
- 任务交互：`control/app.go`
- 结构转换：`utils/convert/`
- 格式解析：`modules/WaterStructure/structure/`

## 许可证与依赖

本仓库包含多个本地依赖模块与第三方组件，实际许可证请分别查看：

- `modules/Conbit/LICENSE`
- `modules/RaaBel/LICENSE`
- `modules/WaterStructure/LICENSE`
- `utils/LICENSE`

