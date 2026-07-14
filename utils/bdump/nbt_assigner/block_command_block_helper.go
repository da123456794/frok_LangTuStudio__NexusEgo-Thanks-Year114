package NBTAssigner

import (
	"fmt"
	"nexus/utils/command_upgrader/mc_command_parser"
	"nexus/utils/mcstructure"
	"strconv"
	"strings"
)

// 适用于 detect 修饰子命令中对方块数据值到方块状态的升级。
// 当返回的字符串为空指针时，
// 意味着未能找到对应的映射，
// 此时升级失败，否则认为升级成功。
// 特别地，如果传入的方块数据值为 -1 ，
// 则永远返回非空指针的空字符串
func upgradeBlock(name string, data int64) (states *string, err error) {
	// 特殊情况快速返回
	if data == -1 {
		tmp := ""
		return &tmp, nil
	}

	// 优化字符串操作：直接判断和切片，避免多次Replace
	cleanName := strings.ToLower(name)
	if strings.HasPrefix(cleanName, "minecraft:") {
		cleanName = cleanName[10:] // "minecraft:" 长度为10
	}

	blockStates, err := get_block_states_from_legacy_block(cleanName, uint16(data))
	if err != nil {
		return nil, nil
	}

	// get block_states(map)
	statesStr, err := mcstructure.MarshalBlockStates(blockStates)
	if err != nil {
		return nil, fmt.Errorf("upgradeBlock: Failed to marshal blockStates; blockStates = %#v, err = %v", blockStates, err)
	}

	return &statesStr, nil
}

// 从 command 解析一个 execute 命令。
// 若返回 nil ，则当前不是一个 execute 命令
func parseExecuteCommand(command string) (e *mc_command_parser.ExecuteCommand, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("parseExecuteCommand: %v", r)
		}
	}()

	e = mc_command_parser.ParseExecuteCommand(command)
	return
}

// 新版 execute 子命令关键字集合（包级变量，避免每次调用重新分配）
var newExecuteKeywords = map[string]struct{}{
	"as":         {},
	"at":         {},
	"positioned": {},
	"align":      {},
	"anchored":   {},
	"facing":     {},
	"in":         {},
	"rotated":    {},
	"store":      {},
	"if":         {},
	"unless":     {},
	"run":        {},
}

// isAlreadyNewExecuteCommand returns true when the command already follows the new execute syntax.
// Heuristic: after "execute" the first token is a new-style subcommand keyword, and a standalone
// "run" token is present (old syntax never uses the "run" keyword).
func isAlreadyNewExecuteCommand(command string) bool {
	cmd := strings.TrimSpace(command)
	if len(cmd) > 0 && cmd[0] == '/' {
		cmd = strings.TrimSpace(cmd[1:])
	}
	lower := strings.ToLower(cmd)
	if !strings.HasPrefix(lower, "execute") {
		return false
	}
	rest := strings.TrimSpace(lower[7:]) // len("execute") == 7
	if len(rest) == 0 {
		return false
	}
	// 提取第一个 token，避免对整个字符串做 Fields 分割
	firstEnd := strings.IndexAny(rest, " \t\n")
	var first string
	if firstEnd == -1 {
		first = rest
	} else {
		first = rest[:firstEnd]
	}
	if _, ok := newExecuteKeywords[first]; !ok {
		return false
	}
	if first == "run" {
		return true
	}
	// 仅在首 token 匹配后才扫描 "run" 关键字
	for _, f := range strings.Fields(rest) {
		if f == "run" {
			return true
		}
	}
	return false
}

// 将旧版本的 execute 命令升级为新格式。
// warningLogs 的状态用于指代是否需要提起警告，
// 若其中包含元素，这意味着在处理到部分 detect 字段时，
// 我们未能为其中的方块找到其对应的方块状态的映射。
// warningLogs 含有的元素即代表这些未能找到映射的方块
func UpgradeExecuteCommand(command string) (new string, warningLogs []string, err error) {
	if isAlreadyNewExecuteCommand(command) {
		return command, nil, nil
	}

	var args *mc_command_parser.ExecuteCommand
	nextBlock := command

	// 单一 Builder 直接写入，消除内层 currentBuilder 的中间分配
	var b strings.Builder
	b.Grow(len(command) * 2)

	for {
		args, err = parseExecuteCommand(nextBlock)
		if err != nil {
			return command, nil, fmt.Errorf("UpgradeExecuteCommand: %v", err)
		}
		if args == nil {
			break
		}

		// 构建选择器：使用 strconv.Quote 替代 fmt.Sprintf("%#v", ...)
		b.WriteString("as ")
		sel := args.Selector
		if len(sel.Main) > 0 && sel.Main[0] == '@' {
			b.WriteString(sel.Main)
			if sel.Sub != nil {
				b.WriteString(*sel.Sub)
			}
		} else {
			b.WriteString(strconv.Quote(sel.Main))
		}
		b.WriteString(" at @s ")

		// 位置升级
		pos := args.Position
		if !(pos[0] == "~" && pos[1] == "~" && pos[2] == "~") &&
			!(pos[0] == "^" && pos[1] == "^" && pos[2] == "^") {
			b.WriteString("positioned ")
			b.WriteString(pos[0])
			b.WriteByte(' ')
			b.WriteString(pos[1])
			b.WriteByte(' ')
			b.WriteString(pos[2])
			b.WriteByte(' ')
		}

		// detect 参数升级为 if block
		if args.DetectArgs != nil {
			detect := args.DetectArgs
			blockData, parseErr := strconv.ParseInt(detect.BlockData, 10, 64)
			if parseErr != nil {
				return command, nil, fmt.Errorf("UpgradeExecuteCommand: Failed to convert string into int; args.DetectArgs.BlockData = %#v, err = %v", detect.BlockData, parseErr)
			}

			blockStates, upgradeErr := upgradeBlock(detect.BlockName, blockData)
			if blockStates == nil && upgradeErr == nil {
				tmp := "[]"
				blockStates = &tmp
				// 惰性初始化 warningLogs，仅在需要时分配
				warningLogs = append(warningLogs, fmt.Sprintf("%s(%d)", detect.BlockName, blockData))
			}
			if upgradeErr != nil {
				return command, nil, fmt.Errorf("UpgradeExecuteCommand: %v", upgradeErr)
			}

			b.WriteString("if block ")
			b.WriteString(detect.BlockPosition[0])
			b.WriteByte(' ')
			b.WriteString(detect.BlockPosition[1])
			b.WriteByte(' ')
			b.WriteString(detect.BlockPosition[2])
			b.WriteByte(' ')
			b.WriteString(detect.BlockName)

			if len(*blockStates) > 0 && *blockStates != "[]" {
				b.WriteByte(' ')
				b.WriteString(*blockStates)
			}
			b.WriteByte(' ')
		}

		nextBlock = args.SubCommand
	}

	result := b.String()
	if len(result) == 0 {
		return command, warningLogs, nil
	}

	// 精确预分配最终结果容量
	b.Reset()
	b.Grow(8 + len(result) + 4 + len(nextBlock)) // "execute " + result + "run " + nextBlock
	b.WriteString("execute ")
	b.WriteString(result)
	b.WriteString("run ")
	b.WriteString(nextBlock)

	return b.String(), warningLogs, nil
}
