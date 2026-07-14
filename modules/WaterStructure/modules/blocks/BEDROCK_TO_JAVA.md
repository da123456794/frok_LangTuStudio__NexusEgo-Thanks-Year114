# Bedrock to Java Block Conversion API

完整的 Bedrock 基岩版到 Java 版方块转换功能，基于 PyMCTranslate 数据，支持 63,528 条转换规则。

## API 函数列表

### 1. RuntimeIDToJavaBlockStr
将 Bedrock RuntimeID 转换为 Java 方块字符串（带属性）

```go
func RuntimeIDToJavaBlockStr(runtimeID uint32) (javaBlockStr string, found bool)
```

**示例:**
```go
javaStr, found := blocks.RuntimeIDToJavaBlockStr(1798)
// 返回: "stone", true
```

### 2. RuntimeIDToJavaBlockNameAndStateStr
将 Bedrock RuntimeID 转换为 Java 方块名称和状态字符串（分离）

```go
func RuntimeIDToJavaBlockNameAndStateStr(runtimeID uint32) (blockName, blockState string, found bool)
```

**示例:**
```go
name, state, found := blocks.RuntimeIDToJavaBlockNameAndStateStr(737)
// 返回: "oak_log", "axis=\"y\"", true
```

### 3. RuntimeIDToJavaBlockNameAndState
将 Bedrock RuntimeID 转换为 Java 方块名称和属性 Map

```go
func RuntimeIDToJavaBlockNameAndState(runtimeID uint32) (name string, properties map[string]any, found bool)
```

**示例:**
```go
name, props, found := blocks.RuntimeIDToJavaBlockNameAndState(738)
// 返回: "oak_log", map[string]any{"axis": "x"}, true
```

### 4. BedrockBlockStrToJavaBlockStr
将 Bedrock 方块字符串转换为 Java 方块字符串

```go
func BedrockBlockStrToJavaBlockStr(bedrockBlockStr string) (javaBlockStr string, found bool)
```

**示例:**
```go
javaStr, found := blocks.BedrockBlockStrToJavaBlockStr("coral_block[coral_color=\"yellow\",dead_bit=0b]")
// 返回: "horn_coral_block", true
```

### 5. BedrockBlockNameAndStateToJavaBlock
将 Bedrock 方块名称和属性转换为 Java 方块名称和属性

```go
func BedrockBlockNameAndStateToJavaBlock(name string, properties map[string]any) (javaName string, javaProperties map[string]any, found bool)
```

**示例:**
```go
bedrockProps := map[string]any{
    "direction": int32(3),
    "door_hinge_bit": false,
    "open_bit": false,
    "upper_block_bit": false,
}
javaName, javaProps, found := blocks.BedrockBlockNameAndStateToJavaBlock("oak_door", bedrockProps)
// 返回: "oak_door", map[string]any{"facing": "north", "half": "upper", ...}, true
```

## 转换特性

### 1. 精确匹配
优先使用精确的属性匹配，确保转换准确性

### 2. 模糊匹配
当精确匹配失败时，自动降级到模糊匹配，最大化转换成功率

### 3. 属性映射
自动处理 Bedrock 和 Java 版之间的属性名称和值差异：
- `direction` (Bedrock) → `facing` (Java)
- `door_hinge_bit` (Bedrock) → `hinge` (Java)
- `0b/1b` (Bedrock byte) → `false/true` (Java boolean)

### 4. 特殊方块处理
- `coral_block[coral_color="yellow"]` → `horn_coral_block`
- `tallgrass` → `short_grass`
- `dirt[dirt_type="normal"]` → `dirt`

## 性能

- **查找速度**: O(1) HashMap 查找
- **转换规则数**: 63,528 条
- **数据文件大小**: 309KB (Brotli 压缩)
- **内存占用**: 约 3-5MB（运行时解压后）
- **并发安全**: 支持多线程并发查询

## 架构

```
Bedrock RuntimeID
    ↓
Bedrock Block Info (name + states)
    ↓
BedrockToJavaConvertor
    ├── Precise Match (O(1) HashMap)
    └── Fuzzy Match (遍历相似方块)
    ↓
JavaBlockString (name + SNBT states)
```

## 数据来源

- **PyMCTranslate**: 官方 Minecraft 跨版本转换库
- **数据版本**: Java 1.21.2 / Bedrock 1.21.2
- **生成工具**: `gen_map_neo/main.go`
- **源数据**: `bedrock_to_java_snbt_convert.txt` (196K 行)

## 使用示例

```go
package main

import (
    "fmt"
    "github.com/Yeah114/blocks"
)

func main() {
    // 转换单个方块
    rtid := uint32(1798)
    javaBlock, found := blocks.RuntimeIDToJavaBlockStr(rtid)
    fmt.Printf("Bedrock %d → Java: %s\n", rtid, javaBlock)

    // 批量转换
    bedrockBlocks := []uint32{1798, 737, 9533, 6990}
    for _, rtid := range bedrockBlocks {
        name, state, found := blocks.RuntimeIDToJavaBlockNameAndStateStr(rtid)
        if found {
            fmt.Printf("%s[%s]\n", name, state)
        }
    }
}
```

## 限制

1. 不支持 Entity 和 BlockEntity 数据转换
2. 某些 Bedrock 独有方块（如 `info_update`）无法转换
3. 部分新版本方块可能缺少映射（需更新数据文件）
4. 返回的是方块状态，不包括 RuntimeID（因 Java 版不使用此概念）

## 更新数据

如需更新到新版本：

```bash
cd gen_map_neo/data
python bedrock_to_java_translate.py  # 生成新的转换数据
cd ..
go run main.go  # 重新生成 .br 文件
```

## 相关函数

- `BlockNameAndStateToRuntimeID`: Java/Bedrock → Bedrock RuntimeID
- `RuntimeIDToBlock`: Bedrock RuntimeID → Bedrock Block
- `JavaBlockStrToRuntimeID`: Java 字符串 → Bedrock RuntimeID (别名)
