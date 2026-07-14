package command

import (
	"encoding/binary"
	"io"
)

/*
type Command interface {
	ID() uint8 // Extra ID spaces (uint8) may be allocated in the future.
	Name() string
	Marshal(writer io.Writer) error
	Unmarshal(reader io.Reader) error
}

*/
/*
| 15 | PlaceBlock | u16(blockConstantStringID) u16(blockData) | 在画笔所在位置放置一个方块。同时指定欲放置方块的 `数据值(附加值)` 为 `blockData` ，且该方块在方块池中的 `ID` 为 `blockConstantStringID` |
| 16 | PlaceBlockWithBlockStates | u16(blockConstantStringID) string(blockStatesString) | 在画笔所在位置放置一个方块。同时指定欲放置方块的 `方块状态` 为 `blockStatesString` ，且该方块在方块池中的 `ID` 为 `blockConstantStringID`<br/> `方块状态` 的格式形如 `["color": "orange"]` |
*/

type PlaceBlock struct {
	BlockID   uint16
	BlockData uint16
}

func (p *PlaceBlock) ID() uint8 {
	return 15
}

func (p *PlaceBlock) Name() string {
	return "PlaceBlock"
}

func (p *PlaceBlock) Marshal(writer io.Writer) (err error) {
	err = binary.Write(writer, binary.BigEndian, p.BlockID)
	if err != nil {
		return
	}
	err = binary.Write(writer, binary.BigEndian, p.BlockData)
	if err != nil {
		return
	}
	return nil
}

func (p *PlaceBlock) Unmarshal(reader io.Reader) (err error) {
	err = binary.Read(reader, binary.BigEndian, &p.BlockID)
	if err != nil {
		return
	}
	err = binary.Read(reader, binary.BigEndian, &p.BlockData)
	if err != nil {
		return
	}
	return nil
}
