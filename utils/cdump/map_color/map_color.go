package map_color

import (
	"strings"

	"github.com/lucasb-eyer/go-colorful"
)

type B struct {
	Block  string         // 地图基色
	LOW    colorful.Color // M:0 (LOW)
	NORMAL colorful.Color // M:1 (NORMAL)
	HIGH   colorful.Color // M:2 (HIGH)
}
type C struct {
	Base B
	M    int // 颜色修饰M
}

var MapColor []colorful.Color = []colorful.Color{}
var MapColorMap = []B{}

func MapColorToC() map[colorful.Color]C {
	init_map := map[colorful.Color]C{}
	for _, v := range MapColorMap {
		init_map[v.LOW] = C{v, 0}
		init_map[v.NORMAL] = C{v, 1}
		init_map[v.HIGH] = C{v, 2}
	}
	return init_map
}

func Get_main_color(c colorful.Color) colorful.Color {
	var List []float64
	for _, v := range MapColor {
		s := c.DistanceCIE94(v)
		List = append(List, s)
	}
	return MapColor[getMin(List)]
}
func getMin(t []float64) int {
	min := t[0]
	index := 0
	for i, v := range t {
		if v < min {
			min = v
			index = i
		}
	}
	return index
}

func init() {
	datas := `
grass	#597D27	#6D9930	#7FB238	#435E1D
sandstone	#AEA473	#D5C98C	#F7E9A3	#827B56
web	#8C8C8C	#ABABAB	#C7C7C7	#696969
redstone_block	#B40000	#DC0000	#FF0000	#870000
packed_ice	#7070B4	#8A8ADC	#A0A0FF	#545487
iron_block	#757575	#909090	#A7A7A7	#585858
oak_leaves	#005700	#006A00	#007C00	#004100
white_concrete	#B4B4B4	#DCDCDC	#FFFFFF	#878787
clay	#737681	#8D909E	#A4A8B8	#565861
dirt	#6A4C36	#825E42	#976D4D	#4F3928
stone	#4F4F4F	#606060	#707070	#3B3B3B
light_blue_concrete	#2D2DB4	#3737DC	#4040FF	#212187
oak_planks	#645432	#7B663E	#8F7748	#4B3F26
quartz_block	#B4B1AC	#DCD9D3	#FFFCF5	#878581
orange_concrete	#985924	#BA6D2C	#D87F33	#72431B
magenta_concrete	#7D3598	#9941BA	#B24CD8	#5E2872
light_blue_concrete	#486C98	#5884BA	#6699D8	#365172
yellow_concrete	#A1A124	#C5C52C	#E5E533	#79791B
lime_concrete	#599011	#6DB015	#7FCC19	#436C0D
pink_concrete	#AA5974	#D06D8E	#F27FA5	#804357
gray_concrete	#353535	#414141	#4C4C4C	#282828
light_gray_concrete	#6C6C6C	#848484	#999999	#515151
cyan_concrete	#35596C	#416D84	#4C7F99	#284351
purple_concrete	#592C7D	#6D3699	#7F3FB2	#43215E
blue_concrete	#24357D	#2C4199	#334CB2	#1B285E
brown_concrete	#483524	#58412C	#664C33	#36281B
green_concrete	#485924	#586D2C	#667F33	#36431B
red_concrete	#6C2424	#842C2C	#993333	#511B1B
black_concrete	#111111	#151515	#191919	#0D0D0D
gold_block	#B0A836	#D7CD42	#FAEE4D	#847E28
diamond_block	#409A96	#4FBCB7	#5CDBD5	#307370
lapis_block	#345AB4	#3F6EDC	#4A80FF	#274387
emerald_block	#009928	#00BB32	#00D93A	#00721E
podzol	#5B3C22	#6F4A2A	#815631	#442D19
netherrack	#4F0100	#600100	#700200	#3B0100
white_terracotta	#937C71	#B4988A	#D1B1A1	#6E5D55
orange_terracotta	#703919	#89461F	#9F5224	#542B13
magenta_terracotta	#693D4C	#804B5D	#95576C	#4E2E39
light_blue_terracotta	#4F4C61	#605D77	#706C8A	#3B3949
yellow_terracotta	#835D19	#A0721F	#BA8524	#624613
lime_terracotta	#485225	#58642D	#677535	#363D1C
pink_terracotta	#703637	#8A4243	#A04D4E	#542829
gray_terracotta	#281C18	#31231E	#392923	#1E1512
light_gray_terracotta	#5F4B45	#745C54	#876B62	#473833
cyan_terracotta	#3D4040	#4B4F4F	#575C5C	#2E3030
purple_terracotta	#56333E	#693E4B	#7A4958	#40262E
blue_terracotta	#352B40	#41354F	#4C3E5C	#282030
brown_terracotta	#352318	#412B1E	#4C3223	#281A12
green_terracotta	#35391D	#414624	#4C522A	#282B16
red_terracotta	#642A20	#7A3327	#8E3C2E	#4B1F18
black_terracotta	#1A0F0B	#1F120D	#251610	#130B08
crimson_nylium	#852122	#A3292A	#BD3031	#641919
crimson_stem	#682C44	#7F3653	#943F61	#4E2133
crimson_hyphae	#401114	#4F1519	#5C191D	#300D0F
warped_nylium	#0F585E	#126C73	#167E86	#0B4246
warped_stem	#286462	#327A78	#3A8E8C	#1E4B4A
warped_hyphae	#3C1F2B	#4A2535	#562C3E	#2D1720
warped_wart_block	#0E7F5D	#119B72	#14B485	#0A5F46
deepslate	#464646	#565656	#646464	#343434
raw_iron_block	#987B67	#BA967E	#D8AF93	#725C4D
verdant_froglight	#597569	#6D9081	#7FA796	#43584F`
	// 读取datas中的每一行数据
	data_lines := strings.Split(datas, "\n")
	// 去除开头的 tab和结尾的回车
	for i, line := range data_lines {
		data_lines[i] = strings.Trim(line, "\n")
	}
	for _, line := range data_lines {
		// 将每一行数据按照空格分割
		line_split := strings.Split(line, "\t")
		if len(line_split) != 5 {
			continue
		}
		// 将分割后的数据存入MapColorMap
		LOW, err := colorful.Hex(line_split[1])
		if err != nil {
			panic(err)
		}
		NORMAL, err := colorful.Hex(line_split[2])
		if err != nil {
			panic(err)
		}
		HIGH, err := colorful.Hex(line_split[3])
		if err != nil {
			panic(err)
		}
		MapColorMap = append(MapColorMap, B{line_split[0], LOW, NORMAL, HIGH})
	}
	for _, i := range MapColorMap {
		MapColor = append(MapColor, i.LOW, i.NORMAL, i.HIGH)
	}

}
