package command

import (
	"encoding/binary"
	"io"
)

/*
| ID(u8类型) | 名称 | 类型 | 说明 |
| --- | --- | --- | --- |
| 1 | x+ | 无 | x轴正方向+1 |
| 2 | x- | 无 | x轴负方向+1 |
| 3 | y+ | 无 | y轴正方向+1 |
| 4 | y- | 无 | y轴负方向+1 |
| 5 | z+ | 无 | z轴正方向+1 |
| 6 | z- | 无 | z轴负方向+1 |
| 7 | x++ | u32 | x轴正方向+n |
| 8 | x-- | u32 | x轴负方向+n |
| 9 | y++ | u32 | y轴正方向+n |
| 10 | y-- | u32 | y轴负方向+n |
| 11 | z++ | u32 | z轴正方向+n |
| 12 | z-- | u32 | z轴负方向+n |
*/
/*
type Command interface {
	ID() uint8 // Extra ID spaces (uint8) may be allocated in the future.
	Name() string
	Marshal(writer io.Writer) error
	Unmarshal(reader io.Reader) error
}
*/

type XPlus struct {
}

func (c *XPlus) ID() uint8 {
	return 1
}

func (c *XPlus) Name() string {
	return "x+"
}

func (c *XPlus) Marshal(writer io.Writer) error {
	return nil
}

func (c *XPlus) Unmarshal(reader io.Reader) error {
	return nil
}

type XMinus struct {
}

func (c *XMinus) ID() uint8 {
	return 2
}

func (c *XMinus) Name() string {
	return "x-"
}

func (c *XMinus) Marshal(writer io.Writer) error {
	return nil
}

func (c *XMinus) Unmarshal(reader io.Reader) error {
	return nil
}

type YPlus struct {
}

func (c *YPlus) ID() uint8 {
	return 3
}

func (c *YPlus) Name() string {
	return "y+"
}

func (c *YPlus) Marshal(writer io.Writer) error {
	return nil
}

func (c *YPlus) Unmarshal(reader io.Reader) error {
	return nil
}

type YMinus struct {
}

func (c *YMinus) ID() uint8 {
	return 4
}

func (c *YMinus) Name() string {
	return "y-"
}

func (c *YMinus) Marshal(writer io.Writer) error {
	return nil
}

func (c *YMinus) Unmarshal(reader io.Reader) error {
	return nil
}

type ZPlus struct {
}

func (c *ZPlus) ID() uint8 {
	return 5
}

func (c *ZPlus) Name() string {
	return "z+"
}

func (c *ZPlus) Marshal(writer io.Writer) error {
	return nil
}

func (c *ZPlus) Unmarshal(reader io.Reader) error {
	return nil
}

type ZMinus struct {
}

func (c *ZMinus) ID() uint8 {
	return 6
}

func (c *ZMinus) Name() string {
	return "z-"
}

func (c *ZMinus) Marshal(writer io.Writer) error {
	return nil
}

func (c *ZMinus) Unmarshal(reader io.Reader) error {
	return nil
}

type XPlusN struct {
	N uint32
}

func (c *XPlusN) ID() uint8 {
	return 7
}

func (c *XPlusN) Name() string {
	return "x++"
}

func (c *XPlusN) Marshal(writer io.Writer) error {
	return binary.Write(writer, binary.BigEndian, &c.N)
}

func (c *XPlusN) Unmarshal(reader io.Reader) error {
	return binary.Read(reader, binary.BigEndian, &c.N)
}

type XMinusN struct {
	N uint32
}

func (c *XMinusN) ID() uint8 {
	return 8
}

func (c *XMinusN) Name() string {
	return "x--"
}

func (c *XMinusN) Marshal(writer io.Writer) error {
	return binary.Write(writer, binary.BigEndian, &c.N)
}

func (c *XMinusN) Unmarshal(reader io.Reader) error {
	return binary.Read(reader, binary.BigEndian, &c.N)
}

type YPlusN struct {
	N uint32
}

func (c *YPlusN) ID() uint8 {
	return 9
}

func (c *YPlusN) Name() string {
	return "y++"
}

func (c *YPlusN) Marshal(writer io.Writer) error {
	return binary.Write(writer, binary.BigEndian, &c.N)
}

func (c *YPlusN) Unmarshal(reader io.Reader) error {
	return binary.Read(reader, binary.BigEndian, &c.N)
}

type YMinusN struct {
	N uint32
}

func (c *YMinusN) ID() uint8 {
	return 10
}

func (c *YMinusN) Name() string {
	return "y--"
}

func (c *YMinusN) Marshal(writer io.Writer) error {
	return binary.Write(writer, binary.BigEndian, &c.N)
}

func (c *YMinusN) Unmarshal(reader io.Reader) error {
	return binary.Read(reader, binary.BigEndian, &c.N)
}

type ZPlusN struct {
	N uint32
}

func (c *ZPlusN) ID() uint8 {
	return 11
}

func (c *ZPlusN) Name() string {
	return "z++"
}

func (c *ZPlusN) Marshal(writer io.Writer) error {
	return binary.Write(writer, binary.BigEndian, &c.N)
}

func (c *ZPlusN) Unmarshal(reader io.Reader) error {
	return binary.Read(reader, binary.BigEndian, &c.N)
}

type ZMinusN struct {
	N uint32
}

func (c *ZMinusN) ID() uint8 {
	return 12
}

func (c *ZMinusN) Name() string {
	return "z--"
}

func (c *ZMinusN) Marshal(writer io.Writer) error {
	return binary.Write(writer, binary.BigEndian, &c.N)
}

func (c *ZMinusN) Unmarshal(reader io.Reader) error {
	return binary.Read(reader, binary.BigEndian, &c.N)
}
