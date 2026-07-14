# CLI 命令行参数

这份说明按当前源码整理，适用于 `cmd/nexusego`。

## 启动方式

不带 `-mode` 启动时，程序进入交互模式。

带 `-mode=import` 或 `-mode=export` 启动时，程序按命令行参数直接创建并执行任务。

示例：

```powershell
go run ./cmd/nexusego -token YOUR_TOKEN
```

```powershell
go run ./cmd/nexusego -token YOUR_TOKEN -mode import ...
```

## 通用参数

| 参数 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `-token` | `string` | 空 | API Token。留空时启动后手动输入。 |
| `-mode` | `string` | 空 | 运行模式，可选 `import`、`export`。 |
| `-server` | `string` | 空 | 租赁服号。 |
| `-password` | `string` | 空 | 租赁服密码。 |
| `-dimension` | `string` | `overworld` | 目标维度。 |
## 导入参数

仅在 `-mode=import` 时使用。

| 参数 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `-file` | `string` | 空 | 导入文件名，必填。 |
| `-x` | `int` | `0` | 起始 X。 |
| `-y` | `int` | `0` | 起始 Y。 |
| `-z` | `int` | `0` | 起始 Z。 |
| `-speed` | `int` | `2000` | 导入速度，单位：命令/秒。 |
| `-usefill` | `bool` | `true` | 是否启用增量导入。 |
| `-region` | `int` | `5` | 增量导入区域边长。 |
| `-importnbt` | `bool` | `true` | 是否导入 NBT 数据。 |
| `-importcmd` | `bool` | `true` | 是否导入命令方块数据。 |
| `-cmdspeed` | `int` | `11` | 命令方块数据写入速度。 |
| `-clear` | `bool` | `false` | 是否清理导入区域。 |
| `-deny` | `bool` | `false` | 是否自动铺设拒绝方块。 |
| `-border` | `bool` | `false` | 是否自动铺设边界方块。 |
| `-closecmd` | `bool` | `true` | 是否在导入前关闭命令方块启用状态。 |
| `-fix` | `bool` | `false` | 是否直接进入修补模式。 |
| `-progress` | `int` | `0` | 起始进度，范围 `0-100`。 |
| `-croparea` | `string` | 空 | 裁剪范围，格式：`"x1 y1 z1 x2 y2 z2"`。 |

## 导出参数

仅在 `-mode=export` 时使用。

| 参数 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `-exportfile` | `string` | 自动生成 | 导出文件名。留空时自动生成 `export_时间戳.mcworld`。 |
| `-exportarea` | `string` | 空 | 导出范围，必填，格式：`"x1 y1 z1 x2 y2 z2"`。 |
| `-exportauthor` | `string` | 空 | 导出 `.nexus` 时的作者名。 |
| `-exportpassword` | `string` | 空 | 导出 `.nexus` 时的密码。 |

## 维度写法

源码实际支持的写法如下：

| 输入 | 结果 |
| --- | --- |
| 空值 | `overworld`，ID `0` |
| `overworld`、`ow`、`0` | `overworld`，ID `0` |
| `nether`、`the_nether`、`1` | `nether`，ID `1` |
| `end`、`the_end`、`2` | `the_end`，ID `2` |
| `dm`、`3` | `dm`，ID `3` |
| `名称:ID` | 自定义维度 |

自定义维度要写成 `名称:ID`，例如：

```powershell
-dimension dm:4
-dimension custom:7
```

下面这些写法不会按自定义维度处理：

```powershell
-dimension 4
-dimension dm5
```

## 参数规则

### 导入模式

必须提供：

- `-server`
- `-file`

附加规则：

- `-progress` 超出 `0-100` 会直接报错退出。
- `-croparea` 必须是 6 个整数。
- `-importcmd=false` 时，`-cmdspeed` 不再生效，内部会按 `0` 处理。
- `-cleardrops=true` 时，导入过程中会按区域清理掉落物。

### 导出模式

必须提供：

- `-server`
- `-exportarea`

附加规则：

- `-exportarea` 必须是 6 个整数。
- `-exportfile` 不带扩展名时，自动补成 `.mcworld`。
- `-exportfile` 扩展名不是 `.mcworld` 或 `.nexus` 时，也会改成 `.mcworld`。

### 布尔参数

项目使用 Go 标准 `flag` 包。建议显式写值：

```powershell
-usefill=false
-importnbt=false
-importcmd=false
-cleardrops=false
-closecmd=true
```

## 路径规则

`-file` 和 `-exportfile` 都支持绝对路径和 `file/` 前缀；其中 `-file` 额外支持程序根目录兜底读取。

- `-file` 只写文件名时，优先读取 `NexusEgo_Storage/file/` 下的同名文件，找不到再读取程序根目录。
- `-exportfile` 只写文件名时，默认放到 `NexusEgo_Storage/file/` 下。
- 写成 `file/demo.mcworld` 时，也会落到 `NexusEgo_Storage/file/demo.mcworld`。
- 传绝对路径时，直接使用绝对路径。
- `-file` 传带目录的相对路径时，如果程序运行目录下存在该路径会直接读取，否则拼到 `NexusEgo_Storage/` 下。
- `-exportfile` 传带目录的相对路径时，会拼到 `NexusEgo_Storage/` 下。

示例：

```powershell
-file demo.mcworld
```

实际会按下面的路径查找：

```text
NexusEgo_Storage/file/demo.mcworld
```

## 导入文件补充说明

如果 `-file` 不是 `.mcworld`，程序会先尝试转换，再继续导入。

几点要注意：

- 转换结果会放到 `NexusEgo_Storage/file/`。
- CLI 模式下不会额外提示输入 `.nexus` 密码。
- 带密码的 `.nexus` 文件直接走 CLI 导入时，可能失败。

## 修补模式

`-fix=true` 的作用是跳过正常导入，直接进入修补模式。

另外，正常导入完成后，程序也会继续尝试进入修补模式。也就是说，`-fix` 控制的是“是否先跳过导入”，不是“是否存在修补阶段”。

## 示例

### 交互模式

```powershell
go run ./cmd/nexusego -token YOUR_TOKEN
```

### 导入

```powershell
go run ./cmd/nexusego `
  -token YOUR_TOKEN `
  -mode import `
  -server 123456 `
  -password YOUR_PASSWORD `
  -file build.mcworld `
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

### 导出 mcworld

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

### 导出 nexus

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
