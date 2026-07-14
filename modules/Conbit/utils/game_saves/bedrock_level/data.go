package bedrock_level

import bedrock_level_provider "github.com/LangTuStudio/Conbit/utils/game_saves/bedrock_level/provider"

const defaultBaseGameVersion = "*"

type Data = bedrock_level_provider.Data
type Abilities = bedrock_level_provider.Abilities

func InitDefaultLevelDat() Data {
	return bedrock_level_provider.InitDefaultLevelDat()
}
