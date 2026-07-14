package cdump

var Low_to_high_Block map[string]bool = map[string]bool{}
var High_to_low_Block map[string]bool = map[string]bool{}

func init() {
	low_to_high()
	high_to_low()
}
func low_to_high() {
	/*
		羊毛地毯
	*/
	Low_to_high_Block["white_carpet"] = true
	Low_to_high_Block["orange_carpet"] = true
	Low_to_high_Block["magenta_carpet"] = true
	Low_to_high_Block["light_blue_carpet"] = true
	Low_to_high_Block["yellow_carpet"] = true
	Low_to_high_Block["lime_carpet"] = true
	Low_to_high_Block["pink_carpet"] = true
	Low_to_high_Block["gray_carpet"] = true
	Low_to_high_Block["light_gray_carpet"] = true
	Low_to_high_Block["cyan_carpet"] = true
	Low_to_high_Block["purple_carpet"] = true
	Low_to_high_Block["blue_carpet"] = true
	Low_to_high_Block["brown_carpet"] = true
	Low_to_high_Block["green_carpet"] = true
	Low_to_high_Block["red_carpet"] = true
	Low_to_high_Block["black_carpet"] = true

}
func high_to_low() {
	High_to_low_Block["vine"] = true

}
