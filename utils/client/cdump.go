package client

import (
	"encoding/json"
	"fmt"
	"nexus/constants"
	"nexus/utils/cdump"
	"reflect"

	"github.com/pterm/pterm"
)

/*
Name: Name,
Value: Value,
Type: Type,
Description: Description,
Operand: Operand,
*/
type cdump_decode_import struct {
	Name        string `cdump_name:"Name"`
	Value       bool   `cdump_name:"Value"`
	Type        int    `cdump_name:"Type"`
	Description string `cdump_name:"Description"`
	Operand     int    `cdump_name:"Operand"`
}
type Cdump_Setting struct {
	Speed               int  `cdump_name:"导入速度" cdump_description:"当前设置的导入速度%d/s,默认3000/s"`
	Terrain_Destruction bool `cdump_name:"关地形破坏" cdump_type:"2" cdump_description:"关地形破坏: 导入前会执行指令/gamerule mobgriefing false,可防止导入时岩浆等液体烧坏建筑等情况。"`
	Close_Command_Block bool `cdump_name:"关命令方块" cdump_type:"2" cdump_description:"导入前关闭命令方块启用,以保证导入过程中不会因为启用命令方块导致GG(建议同时启用禁玩家入服)"`
	Stable_Foliage      bool `cdump_name:"稳定的树叶" cdump_type:"1" cdump_description:"将替换数据的特殊值ID,替换为不会自然凋落的版本"`
	No_NBT              bool `cdump_name:"无NBT" cdump_type:"2" cdump_description:"无NBT：导入时放弃nbt数据,视为普通方块。丢弃NBT数据后导入速度可能加快。但是会丢弃数据。"`
	Clear_Building      bool `cdump_name:"清空导入区" cdump_type:"2" cdump_description:"对导入文件所在的区域全部清空（此选项会大幅度降低导入速度并无视完整导入选项）"`
	Clear_Drops         bool `cdump_name:"清空掉落物" cdump_type:"2" cdump_description:"导入时按区域清理掉落物,可减少方块破坏产生的物品堆积。"`
	// 完全导入
	Full_Import bool `cdump_name:"仅建筑高度" cdump_type:"2" cdump_description:"启用后,在启用清空导入区的情况下,仅清空建筑所在高度的区域，而非全部高度"`
	// Forbidden_Honkai_Backtracking bool `cdump_name:"禁崩坏回溯" cdump_type:"3" cdump_description:"机器人崩溃后将直接以当前进度运行而非上一次tp操作前。"`
	Unbuilderwater bool `cdump_name:"水方块禁用" cdump_type:"2" cdump_description:"导入时将水方块替换为淡蓝色玻璃"`
	// Disable_Water_After_Import bool `cdump_name:"禁用水流出" cdump_type:"2" cdump_description:"导入时会在导入范围外的地方将空气替换为玻璃,以防止水流出。(仅涉及接触到水方块时此方法有效)（启用水方块禁用会使得本参数失效）"`
	Unbuildermagma bool `cdump_name:"岩浆禁用" cdump_type:"2" cdump_description:"导入时将岩浆方块替换为红色玻璃"`
	// Disable_Magma_After_Import bool `cdump_name:"禁岩浆流出" cdump_type:"2" cdump_description:"导入时会在导入范围外的地方将空气替换为玻璃,以防止岩浆流出。(仅涉及接触到岩浆方块时此方法有效)（启用岩浆禁用会使得本参数失效）"`
	Silverfish_Blocks bool `cdump_name:"禁蠹虫方块" cdump_type:"1" cdump_description:"导入时自动替换为正常版本的方块。(强烈建议启用)"`
	Close_Sign        bool `cdump_name:"禁修改告示牌" cdump_type:"2" cdump_description:"导入时无视蜜蜡选项,强制使用蜜蜡"`
	No_Import_bar     bool `cdump_name:"禁进度条" cdump_type:"2" cdump_description:"导入时禁用进度条显示"`
	// 每个区域包含的区块数量边长（区块为16方块），例如4表示 4x4 区块 -> 64x64 方块
	RegionSize int `cdump_name:"区域大小" cdump_type:"1" cdump_description:"每个处理区域的区块边长，例如4表示4x4区块（64x64方块）"`
	// 当缓存区域数量超过该阈值时开始流式处理
	StreamFlushThreshold int `cdump_name:"流式阈值" cdump_type:"1" cdump_description:"当未处理的区域数超过该阈值时会先处理最早区域以节省内存"`
	// NoAir       bool `cdump_name:"过滤空气" cdump_type:"2" cdump_description:"启用后将不再导入空气方块,如果你需要清空导入区域,此选项建议启用"`
	// NoWater     bool `cdump_name:"无水导入" cdump_type:"2" cdump_description:"启用后将不再导入水方块,如果你需要导入水,此选项建议关闭"`
	// NoLava      bool `cdump_name:"无岩浆导入" cdump_type:"2" cdump_description:"启用后将不再导入岩浆方块,如果你需要导入岩浆,此选项建议关闭"`
	// SingleWater bool `cdump_name:"单层水" cdump_type:"1" cdump_description:"启用后导入的水方块将延迟导入并只导入顶层水,导入时将自动沿着水面导入冰块。（有岩浆块和灵魂沙的水除外）"`
	// Surface     bool `cdump_name:"表面建筑" cdump_type:"2" cdump_description:"启用后只导入建筑表面,建筑内部将不再导入。(适合导入迷宫,地下类文件,目前版本算法不稳定,会出现误判。为保证稳定性,建议不要使用)"`
	// BottomWater bool `cdump_name:"底层水玻璃" cdump_type:"1" cdump_description:"启用后导入的水如果在导入范围外没有除空气以外的方块,则会在导入区域内防止玻璃防止水流出。(玻璃外围的方块不会导入)"`
	// NoCommand   bool `cdump_name:"无命令" cdump_type:"2" cdump_description:"启用后将不再导入命令方块,如果你需要导入命令方块,此选项建议关闭"`
	// NoTree      bool `cdump_name:"无树木" cdump_type:"2" cdump_desription:"启用后将不再导入树叶,而是使用羊毛方块代替"`
	// NoGrass     bool `cdump_name:"无草" cdump_type:"2" cdump_description:"启用后将不再导入草方块,如果你需要导入草,此选项建议关闭"`
	// NoSnow      bool `cdump_name:"无雪" cdump_type:"2" cdump_description:"启用后将不再导入顶层雪方块,如果你需要导入顶层雪,此选项建议关闭"`
	// NoNBT       bool `cdump_name:"不要NBT" cdump_type:"2" cdump_description:"启用后将不再导入方块的NBT数据,如果你需要导入方块的NBT数据,此选项建议关闭"`
	// FastChest   bool `cdump_name:"快速箱子" cdump_type:"1" cdump_description:"启用后没有NBT物品的箱子会使用指令来导入。此选项导入速度会变快。"`
	// NoSpider    bool `cdump_name:"不要蠹虫石" cdump_type:"2" cdump_description:"启用后将蠹虫石方块替换为正常版本的方块,如果你需要导入蠹虫石,此选项建议关闭"`

}

func (c *Cdump_Setting) Get_Speed() int {
	return c.Speed
}
func (c *Cdump_Setting) Get_Setting() []string {
	type_list := []string{}
	s := reflect.ValueOf(c).Elem()
	for i := 0; i < s.NumField(); i++ {
		field := s.Type().Field(i)
		if field.Name == "Speed" {
			type_list = append(type_list, fmt.Sprintf(
				"%s: %s",
				field.Tag.Get("cdump_name"),
				fmt.Sprintf(field.Tag.Get("cdump_description"), c.Speed),
			))
		} else {
			cdump_type_str := pterm.Yellow("未知")
			/*
				> | 值 | 说明 |
				> | --- | --- |
				> | 0 | 默认 |
				> | 1 | 纯优化 |
				> | 2 | 普通 |
				> | 3 | 危险 |
			*/
			switch field.Tag.Get("cdump_type") {
			case "0":
				cdump_type_str = pterm.Green("默认")
			case "1":
				cdump_type_str = pterm.Blue("纯优化")
			case "2":
				cdump_type_str = pterm.Magenta("普通")
			case "3":
				cdump_type_str = pterm.Red("危险")
			default:
				cdump_type_str = pterm.Yellow("未知")
			}
			var is_use string
			if s.Field(i).Bool() {
				is_use = pterm.Green("启用")
			} else {
				is_use = pterm.Red("未启用")
			}
			type_list = append(type_list, fmt.Sprintf(
				"%s: %s, 类型:%s,说明:%s",
				field.Tag.Get("cdump_name"),
				pterm.Green(is_use),
				cdump_type_str,
				field.Tag.Get("cdump_description"),
			))
		}
		// fmt.Printf("Field: %s, Name: %s, Tag cdump_name: %s, cdump_type: %s, cdump_description: %s\n", field.Name, field.Type, field.Tag.Get("cdump_name"), field.Tag.Get("cdump_type"), field.Tag.Get("cdump_description"))
	}
	return type_list
}

// 版本 1 没实装参数控制
func (c *Cdump_Setting) Check_Parameter(name []cdump.Parameter) (bool, []string) {
	return true, []string{}
}

// 版本 2 参数转换
// 参数不包括速度设置
func (c *Cdump_Setting) Convert_Parameter(name []cdump.Parameter) {
	for _, v := range name {
		switch v.Name {
		case "关地形破坏":
			c.Terrain_Destruction = v.Value
		// case "清空掉落物":
		// 	c.Clear_Drops_After_Import = v.Value
		case "关命令方块":
			c.Close_Command_Block = v.Value
		case "稳定的树叶":
			c.Stable_Foliage = v.Value
		case "无NBT":
			c.No_NBT = v.Value
		case "清空导入区":
			c.Clear_Building = v.Value
		case "清空掉落物":
			c.Clear_Drops = v.Value
		case "仅建筑高度":
			c.Full_Import = v.Value
		// case "禁崩坏回溯":
		// 	c.Forbidden_Honkai_Backtracking = v.Value
		case "水方块禁用":
			c.Unbuilderwater = v.Value
		// case "禁用水流出":
		// 	c.Disable_Water_After_Import = v.Value
		case "岩浆禁用":
			c.Unbuildermagma = v.Value
		// case "禁岩浆流出":
		// 	c.Disable_Magma_After_Import = v.Value
		case "禁蠹虫方块":
			c.Silverfish_Blocks = v.Value
		case "禁进度条":
			c.No_Import_bar = v.Value
		}
	}
}

// 从 json 字符串中获取参数
func (c *Cdump_Setting) Get_Parameter_From_Json(json_str string) (err error) {
	var json_map []cdump_decode_import
	err = json.Unmarshal([]byte(json_str), &json_map)
	if err != nil {
		return err
	}
	for _, v := range json_map {
		switch v.Name {
		case "关地形破坏":
			c.Terrain_Destruction = v.Value
		// case "清空掉落物":
		// 	c.Clear_Drops_After_Import = v.Value
		case "关命令方块":
			c.Close_Command_Block = v.Value
		case "稳定的树叶":
			c.Stable_Foliage = v.Value
		case "无NBT":
			c.No_NBT = v.Value
		case "清空导入区":
			c.Clear_Building = v.Value
		case "清空掉落物":
			c.Clear_Drops = v.Value
		case "仅建筑高度":
			c.Full_Import = v.Value
		// case "禁崩坏回溯":
		// 	c.Forbidden_Honkai_Backtracking = v.Value
		case "水方块禁用":
			c.Unbuilderwater = v.Value
		// case "禁用水流出":
		// 	c.Disable_Water_After_Import = v.Value
		case "岩浆禁用":
			c.Unbuildermagma = v.Value
		// case "禁岩浆流出":
		// 	c.Disable_Magma_After_Import = v.Value
		case "禁蠹虫方块":
			c.Silverfish_Blocks = v.Value
		case "禁进度条":
			c.No_Import_bar = v.Value
		}
	}
	return nil
}
func New_Cdump_Setting() *Cdump_Setting {
	return &Cdump_Setting{
		Close_Command_Block:  true,
		No_NBT:               false,
		Speed:                constants.DefaultImportSpeed,
		RegionSize:           4,
		StreamFlushThreshold: 4,
	}
}
