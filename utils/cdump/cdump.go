package cdump

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"nexus/utils/cdump/command"

	"github.com/df-mc/goleveldb/leveldb"
	"github.com/df-mc/goleveldb/leveldb/util"
)

/*
## 初始数据

| ID(u8类型) | 名称 | 类型 | 说明 |
| --- | --- | --- | --- |
| 0 | 保留 | 无 | 这是一个系统保留符号 |
| 1 | version | u32 | 版本号 |
| 2 | parameter | []parameter | 参数  |
| 3 | author | string | 作者  |
| 4 | description | string | 描述  |
| 5 | user id | string | 用户ID |
| 6 | File checksum | string | 文件校验码 |
| 7 | operand | u64 | 操作数(不包括可选参数中的操作数)(如果溢出,则取最大值) |
| 8 | build_xyz | [3]u64 | 建筑大小(3维,xyz) |
| 9 | end | 无 | 结束初始化符号 |
| 10 | isgzip | u8 | 是否压缩(0:不压缩,1:压缩)(版本2协议新内容) |
*/
type CDump struct {
	Version      uint32
	Parameter    []Parameter
	Author       string
	Description  string
	UserID       string
	FileChecksum string
	Operand      uint64
	BuildXYZ     [3]uint64
	IsGzip       bool
	Data         []command.Command

	Cache_Dir     string        // 缓存目录
	Cdump_leverdb *leveldb.DB   // cdump的leveldb接口
	str_word_list []string      // 字符串列表
	cache_Buffer  *bytes.Buffer // 区块缓存
	cache_Chain_x int           // 上一个区块x
	cache_Chain_z int           // 上一个区块z
	cache_last_x  int           // 上一个区块x
	cache_last_y  int           // 上一个区块y
	cache_last_z  int           // 上一个区块z
}

func (c *CDump) Write_Hander(w io.Writer) (err error) {
	switch c.Version {
	case 0:
		return c.Write_Hander_V1(w)
	case 1:
		return c.Write_Hander_V1(w)
	case 2:
		return c.Write_Hander_V2(w)
	default:
		return fmt.Errorf("unknow version %d", c.Version)
	}
}
func (c *CDump) Write_Hander_V1(w io.Writer) (err error) {
	_, err = w.Write([]byte{0})
	if err != nil {
		return
	}
	_, err = w.Write([]byte{1})
	if err != nil {
		return
	}
	binary.Write(w, binary.BigEndian, c.Version)
	_, err = w.Write([]byte{2})
	if err != nil {
		return
	}
	binary.Write(w, binary.BigEndian, uint32(len(c.Parameter)))
	for _, p := range c.Parameter {
		p.Write_Hander(w)
	}
	_, err = w.Write([]byte{3})
	if err != nil {
		return
	}
	byte_author := []byte(c.Author)
	binary.Write(w, binary.BigEndian, uint32(len(byte_author)))
	_, err = w.Write(byte_author)
	if err != nil {
		return
	}
	_, err = w.Write([]byte{4})
	if err != nil {
		return
	}
	byte_description := []byte(c.Description)
	binary.Write(w, binary.BigEndian, uint32(len(byte_description)))
	_, err = w.Write(byte_description)
	if err != nil {
		return
	}
	_, err = w.Write([]byte{5})
	if err != nil {
		return
	}
	byte_user_id := []byte(c.UserID)
	binary.Write(w, binary.BigEndian, uint32(len(byte_user_id)))
	_, err = w.Write(byte_user_id)
	if err != nil {
		return
	}
	_, err = w.Write([]byte{6})
	if err != nil {
		return
	}
	byte_file_checksum := []byte(c.FileChecksum)
	binary.Write(w, binary.BigEndian, uint32(len(byte_file_checksum)))
	_, err = w.Write(byte_file_checksum)
	if err != nil {
		return
	}
	_, err = w.Write([]byte{7})
	if err != nil {
		return
	}
	binary.Write(w, binary.BigEndian, c.Operand)
	_, err = w.Write([]byte{8})
	if err != nil {
		return
	}
	binary.Write(w, binary.BigEndian, c.BuildXYZ[0])
	binary.Write(w, binary.BigEndian, c.BuildXYZ[1])
	binary.Write(w, binary.BigEndian, c.BuildXYZ[2])
	_, err = w.Write([]byte{9})
	if err != nil {
		return
	}
	return nil
}
func (c *CDump) Write_Hander_V2(w io.Writer) (err error) {
	_, err = w.Write([]byte{0})
	if err != nil {
		return
	}
	_, err = w.Write([]byte{1})
	if err != nil {
		return
	}
	binary.Write(w, binary.BigEndian, c.Version)
	_, err = w.Write([]byte{2})
	if err != nil {
		return
	}
	binary.Write(w, binary.BigEndian, uint32(len(c.Parameter)))
	for _, p := range c.Parameter {
		p.Write_Hander(w)
	}
	_, err = w.Write([]byte{3})
	if err != nil {
		return
	}
	byte_author := []byte(c.Author)
	binary.Write(w, binary.BigEndian, uint32(len(byte_author)))
	_, err = w.Write(byte_author)
	if err != nil {
		return
	}
	_, err = w.Write([]byte{4})
	if err != nil {
		return
	}
	byte_description := []byte(c.Description)
	binary.Write(w, binary.BigEndian, uint32(len(byte_description)))
	_, err = w.Write(byte_description)
	if err != nil {
		return
	}
	_, err = w.Write([]byte{5})
	if err != nil {
		return
	}
	byte_user_id := []byte(c.UserID)
	binary.Write(w, binary.BigEndian, uint32(len(byte_user_id)))
	_, err = w.Write(byte_user_id)
	if err != nil {
		return
	}
	_, err = w.Write([]byte{6})
	if err != nil {
		return
	}
	byte_file_checksum := []byte(c.FileChecksum)
	binary.Write(w, binary.BigEndian, uint32(len(byte_file_checksum)))
	_, err = w.Write(byte_file_checksum)
	if err != nil {
		return
	}
	_, err = w.Write([]byte{7})
	if err != nil {
		return
	}
	binary.Write(w, binary.BigEndian, c.Operand)
	_, err = w.Write([]byte{8})
	if err != nil {
		return
	}
	binary.Write(w, binary.BigEndian, c.BuildXYZ[0])
	binary.Write(w, binary.BigEndian, c.BuildXYZ[1])
	binary.Write(w, binary.BigEndian, c.BuildXYZ[2])
	_, err = w.Write([]byte{10})
	if err != nil {
		return
	}
	binary.Write(w, binary.BigEndian, c.IsGzip)
	_, err = w.Write([]byte{9})
	if err != nil {
		return
	}
	return nil
}

func (c *CDump) Read_Hander(r io.Reader) error {
	return c.Read_Hander_V2(r)
}

func (c *CDump) Read_Hander_V1(r io.Reader) error {
	for {
		b := make([]byte, 1)
		_, err := r.Read(b)
		if err != nil {
			return err
		}
		// fmt.Println(b)
		switch b[0] {
		case 0:
			continue
		case 1:
			err = binary.Read(r, binary.BigEndian, &c.Version)
			if err != nil {
				return err
			}
		case 2:
			var Parameter_len uint32
			err = binary.Read(r, binary.BigEndian, &Parameter_len)
			if err != nil {
				return err
			}
			for i := 0; i < int(Parameter_len); i++ {
				p := Parameter{}
				err := p.Read_Hander(r)
				if err != nil {
					return err
				}
				c.Parameter = append(c.Parameter, p)
			}
		case 3:
			author_len := make([]byte, 4)
			_, err := r.Read(author_len)
			if err != nil {
				return err
			}
			author_len_int := binary.BigEndian.Uint32(author_len)
			author := make([]byte, author_len_int)
			_, err = r.Read(author)
			if err != nil {
				return err
			}
			c.Author = string(author)
		case 4:
			description_len := make([]byte, 4)
			_, err := r.Read(description_len)
			if err != nil {
				return err
			}
			description_len_int := binary.BigEndian.Uint32(description_len)
			description := make([]byte, description_len_int)
			_, err = r.Read(description)
			if err != nil {
				return err
			}
			c.Description = string(description)
		case 5:
			user_id_len := make([]byte, 4)
			_, err := r.Read(user_id_len)
			if err != nil {
				return err
			}
			user_id_len_int := binary.BigEndian.Uint32(user_id_len)
			user_id := make([]byte, user_id_len_int)
			_, err = r.Read(user_id)
			if err != nil {
				return err
			}
			c.UserID = string(user_id)
		case 6:
			file_checksum_len := make([]byte, 4)
			_, err := r.Read(file_checksum_len)
			if err != nil {
				return err
			}
			file_checksum_len_int := binary.BigEndian.Uint32(file_checksum_len)
			file_checksum := make([]byte, file_checksum_len_int)
			_, err = r.Read(file_checksum)
			if err != nil {
				return err
			}
			c.FileChecksum = string(file_checksum)
		case 7:
			operand := make([]byte, 8)
			_, err := r.Read(operand)
			if err != nil {
				return err
			}
			c.Operand = binary.BigEndian.Uint64(operand)
		case 8:
			build_xyz := make([]byte, 24)
			_, err := r.Read(build_xyz)
			if err != nil {
				return err
			}
			c.BuildXYZ[0] = binary.BigEndian.Uint64(build_xyz[0:8])
			c.BuildXYZ[1] = binary.BigEndian.Uint64(build_xyz[8:16])
			c.BuildXYZ[2] = binary.BigEndian.Uint64(build_xyz[16:24])
		case 9:
			return nil
		default:
			return fmt.Errorf("unknown type %d", b[0])
		}

	}
}
func (c *CDump) Read_Hander_V2(r io.Reader) error {
	for {
		b := make([]byte, 1)
		_, err := r.Read(b)
		if err != nil {
			return err
		}
		// fmt.Println(b)
		switch b[0] {
		case 0:
			continue
		case 1:
			err = binary.Read(r, binary.BigEndian, &c.Version)
			if err != nil {
				return err
			}
		case 2:
			var Parameter_len uint32
			err = binary.Read(r, binary.BigEndian, &Parameter_len)
			if err != nil {
				return err
			}
			for i := 0; i < int(Parameter_len); i++ {
				p := Parameter{}
				err := p.Read_Hander(r)
				if err != nil {
					return err
				}
				c.Parameter = append(c.Parameter, p)
			}
		case 3:
			author_len := make([]byte, 4)
			_, err := r.Read(author_len)
			if err != nil {
				return err
			}
			author_len_int := binary.BigEndian.Uint32(author_len)
			author := make([]byte, author_len_int)
			_, err = r.Read(author)
			if err != nil {
				return err
			}
			c.Author = string(author)
		case 4:
			description_len := make([]byte, 4)
			_, err := r.Read(description_len)
			if err != nil {
				return err
			}
			description_len_int := binary.BigEndian.Uint32(description_len)
			description := make([]byte, description_len_int)
			_, err = r.Read(description)
			if err != nil {
				return err
			}
			c.Description = string(description)
		case 5:
			user_id_len := make([]byte, 4)
			_, err := r.Read(user_id_len)
			if err != nil {
				return err
			}
			user_id_len_int := binary.BigEndian.Uint32(user_id_len)
			user_id := make([]byte, user_id_len_int)
			_, err = r.Read(user_id)
			if err != nil {
				return err
			}
			c.UserID = string(user_id)
		case 6:
			file_checksum_len := make([]byte, 4)
			_, err := r.Read(file_checksum_len)
			if err != nil {
				return err
			}
			file_checksum_len_int := binary.BigEndian.Uint32(file_checksum_len)
			file_checksum := make([]byte, file_checksum_len_int)
			_, err = r.Read(file_checksum)
			if err != nil {
				return err
			}
			c.FileChecksum = string(file_checksum)
		case 7:
			operand := make([]byte, 8)
			_, err := r.Read(operand)
			if err != nil {
				return err
			}
			c.Operand = binary.BigEndian.Uint64(operand)
		case 8:
			build_xyz := make([]byte, 24)
			_, err := r.Read(build_xyz)
			if err != nil {
				return err
			}
			c.BuildXYZ[0] = binary.BigEndian.Uint64(build_xyz[0:8])
			c.BuildXYZ[1] = binary.BigEndian.Uint64(build_xyz[8:16])
			c.BuildXYZ[2] = binary.BigEndian.Uint64(build_xyz[16:24])
		case 9:
			return nil
		case 10:
			binary.Read(r, binary.BigEndian, &c.IsGzip)
		default:
			return fmt.Errorf("unknown type %d", b[0])
		}

	}
}
func (c *CDump) Write_Data(w io.Writer, is_Body bool) (err error) {
	if is_Body {
		_, err = w.Write([]byte{0})
		if err != nil {
			return
		}
	}
	for _, d := range c.Data {
		_, err = w.Write([]byte{d.ID()})
		if err != nil {
			return
		}
		d.Marshal(w)
	}
	if is_Body {
		_, err = w.Write([]byte{22})
		if err != nil {
			return
		}
	}
	return nil
}

func (c *CDump) Read_Data(r io.Reader) error {
	for {
		b := make([]byte, 1)
		_, err := r.Read(b)
		if err != nil {
			return err
		}
		switch b[0] {
		case 0:
			continue
		case 22:
			return nil
		default:
			_, isHave := command.CDumpCommandPool_1[b[0]]

			if !isHave {
				return fmt.Errorf("unknown command type %d", b[0])
			}
			d := command.CDumpCommandPool_1[b[0]]()
			err := d.Unmarshal(r)
			if err != nil {
				return err
			}
			c.Data = append(c.Data, d)
		}

	}
}
func (c *CDump) Read_Data_One(r io.Reader, isSave bool) (command.Command, error) {
	for {
		b := make([]byte, 1)
		_, err := r.Read(b)
		if err == io.EOF {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}

		switch b[0] {
		case 0:
			continue
		case 22:
			return nil, nil
		default:
			_, isHave := command.CDumpCommandPool_1[b[0]]

			if !isHave {
				return nil, fmt.Errorf("unknown command type %d", b[0])
			}
			d := command.CDumpCommandPool_1[b[0]]()
			err := d.Unmarshal(r)
			if err != nil {
				return nil, err
			}
			if isSave {
				c.Data = append(c.Data, d)
			}

			return d, nil
		}

	}
}
func (c *CDump) Word_Put(word string) int {
	if c.Word_Has(word) {
		return c.Word_GetID(word)
	}
	len_w := c.Word_GetLen()
	c.str_word_list = append(c.str_word_list, word)
	return len_w
}
func (c *CDump) Word_Get(index int) string {
	return c.str_word_list[index]
}
func (c *CDump) Word_GetID(word string) int {
	for i, v := range c.str_word_list {
		if v == word {
			return i
		}
	}
	return -1
}
func (c *CDump) Word_GetLen() int {
	return len(c.str_word_list)
}
func (c *CDump) Word_Has(word string) bool {
	for _, v := range c.str_word_list {
		if v == word {
			return true
		}
	}
	return false
}

// 写入词组表
func (c *CDump) Word_Write() (err error) {
	is_true, err := c.Cdump_leverdb.Has([]byte{2}, nil)
	if err != nil {
		return
	}
	if !is_true {
		none_word := bytes.NewBuffer([]byte{})
		binary.Write(none_word, binary.BigEndian, uint32(0))
		c.Cdump_leverdb.Put([]byte{2}, none_word.Bytes(), nil)
		return
	}
	bytes_word := bytes.NewBuffer([]byte{})
	binary.Write(bytes_word, binary.BigEndian, uint32(len(c.str_word_list)))
	for _, v := range c.str_word_list {
		command.WriteString(bytes_word, v)
	}
	c.Cdump_leverdb.Put([]byte{2}, bytes_word.Bytes(), nil)
	// data, err = c.Cdump_leverdb.Get(q_c, nil)
	return
}

// 读取词组表
func (c *CDump) Word_Read() (err error) {
	data, err := c.Cdump_leverdb.Get([]byte{2}, nil)
	if err != nil {
		return
	}
	if len(data) == 0 {
		return
	}
	r := bytes.NewReader(data)
	var len_word uint32
	binary.Read(r, binary.BigEndian, &len_word)
	for i := uint32(0); i < len_word; i++ {
		word, err := command.ReadString(r)
		if err != nil {
			return err
		}
		c.str_word_list = append(c.str_word_list, word)
	}
	return
}

func (c *CDump) SetBlock(block command.Command, pos [3]int) (err error) {
	// 获取相对坐标数值
	// 计算区块位置.16*16为一个区块
	x_q := pos[0] / 16
	z_q := pos[2] / 16
	xyz, err := c.GetChainEndXYZ(x_q, z_q)
	if err != nil {
		return err
	}
	datas, err := c.GetChain(x_q, z_q)
	if err != nil {
		return err
	}
	io_wd := bytes.NewBuffer(datas)
	// 获取坐标在当前区块的xz
	x := pos[0] % 16
	z := pos[2] % 16
	// 计算与上一个坐标的距离
	x_d, y_d, z_d := x-int(xyz[0]), pos[1]-int(xyz[1]), z-int(xyz[2])

	if x_d < 0 {
		if x_d == -1 {
			cmd := command.XMinus{}
			_, err = io_wd.Write([]byte{cmd.ID()})
			if err != nil {
				return err
			}
		} else {
			cmd := command.XMinusN{
				N: uint32(-x_d),
			}
			_, err = io_wd.Write([]byte{cmd.ID()})
			if err != nil {
				return err
			}
			err = cmd.Marshal(io_wd)
			if err != nil {
				return err
			}
		}
	} else if x_d > 0 {
		if x_d == 1 {
			cmd := command.XPlus{}
			_, err = io_wd.Write([]byte{cmd.ID()})
			if err != nil {
				return err
			}
		} else {
			cmd := command.XPlusN{
				N: uint32(x_d),
			}
			_, err = io_wd.Write([]byte{cmd.ID()})
			if err != nil {
				return err
			}
			err = cmd.Marshal(io_wd)
			if err != nil {
				return err
			}

		}
	}
	// y
	if y_d < 0 {
		if y_d == -1 {
			cmd := command.YMinus{}
			_, err = io_wd.Write([]byte{cmd.ID()})
			if err != nil {
				return err
			}
		} else {
			cmd := command.YMinusN{
				N: uint32(-y_d),
			}
			_, err = io_wd.Write([]byte{cmd.ID()})
			if err != nil {
				return err
			}
			err = cmd.Marshal(io_wd)
			if err != nil {
				return err
			}
		}
	} else if y_d > 0 {
		if y_d == 1 {
			cmd := command.YPlus{}
			_, err = io_wd.Write([]byte{cmd.ID()})
			if err != nil {
				return err
			}
		} else {
			cmd := command.YPlusN{
				N: uint32(y_d),
			}
			_, err = io_wd.Write([]byte{cmd.ID()})
			if err != nil {
				return err
			}
		}
	}

	// z
	if z_d < 0 {
		if z_d == -1 {
			cmd := command.ZMinus{}
			_, err = io_wd.Write([]byte{cmd.ID()})
			if err != nil {
				return err
			}
		} else {
			cmd := command.ZMinusN{
				N: uint32(-z_d),
			}
			_, err = io_wd.Write([]byte{cmd.ID()})
			if err != nil {
				return err
			}
			err = cmd.Marshal(io_wd)
			if err != nil {
				return err
			}
		}
	} else if z_d > 0 {
		if z_d == 1 {
			cmd := command.ZPlus{}
			_, err = io_wd.Write([]byte{cmd.ID()})
			if err != nil {
				return err
			}
		} else {
			cmd := command.ZPlusN{
				N: uint32(z_d),
			}
			_, err = io_wd.Write([]byte{cmd.ID()})
			if err != nil {
				return err
			}
			err = cmd.Marshal(io_wd)
			if err != nil {
				return err
			}
		}
	}
	// 位移完成后放置方块
	_, err = io_wd.Write([]byte{block.ID()})
	if err != nil {
		return err
	}
	err = block.Marshal(io_wd)
	if err != nil {
		return err
	}
	// 更新区块最后位置
	c.SetChainEndXYZ(x_q, z_q, [3]int{x, pos[1], z})
	// 更新区块数据
	c.SetChain(x_q, z_q, io_wd.Bytes())

	return nil
}

// 获取这个区块记录的最后位置
func (c *CDump) GetChainEndXYZ(x_q int, z_q int) (xyz [3]int, err error) {
	q_c := []byte{4}
	write_io := bytes.NewBuffer(q_c)
	binary.Write(write_io, binary.BigEndian, uint32(x_q))
	binary.Write(write_io, binary.BigEndian, uint32(z_q))
	q_c = write_io.Bytes()
	is_true, err := c.Cdump_leverdb.Has(q_c, nil)
	if err != nil {
		return
	}
	if !is_true {
		xyz = [3]int{0, 0, 0}
		return
	}
	data, err := c.Cdump_leverdb.Get(q_c, nil)
	if err != nil {
		return
	}
	data_io := bytes.NewBuffer(data)
	var x_16, y_16, z_16 uint32 = 0, 0, 0
	err = binary.Read(data_io, binary.BigEndian, x_16)
	if err != nil {
		return
	}
	err = binary.Read(data_io, binary.BigEndian, y_16)
	if err != nil {
		return
	}
	err = binary.Read(data_io, binary.BigEndian, z_16)
	if err != nil {
		return
	}
	xyz = [3]int{int(x_16), int(y_16), int(z_16)}
	return

}

// 设置这个区块的相对坐标最后位置
func (c *CDump) SetChainEndXYZ(x_q int, z_q int, xyz [3]int) (err error) {
	q_c := []byte{4}
	write_io := bytes.NewBuffer(q_c)
	binary.Write(write_io, binary.BigEndian, uint32(x_q))
	binary.Write(write_io, binary.BigEndian, uint32(z_q))
	write_io2 := bytes.NewBuffer([]byte{})
	binary.Write(write_io, binary.BigEndian, uint32(xyz[0]))
	binary.Write(write_io, binary.BigEndian, uint32(xyz[1]))
	binary.Write(write_io, binary.BigEndian, uint32(xyz[2]))
	err = c.Cdump_leverdb.Put(write_io.Bytes(), write_io2.Bytes(), nil)
	return
}

// 读取这个区块的数据
func (c *CDump) GetChain(x_q int, z_q int) (data []byte, err error) {
	q_c := []byte{3}
	write_io := bytes.NewBuffer(q_c)
	binary.Write(write_io, binary.BigEndian, uint32(x_q))
	binary.Write(write_io, binary.BigEndian, uint32(z_q))
	q_c = write_io.Bytes()
	is_true, err := c.Cdump_leverdb.Has(q_c, nil)
	if err != nil {
		return
	}
	if !is_true {
		c.Cdump_leverdb.Put(q_c, []byte{0}, nil)
		return
	}
	data, err = c.Cdump_leverdb.Get(q_c, nil)
	return
}

// 更新这个区块的数据
func (c *CDump) SetChain(x_q int, z_q int, data []byte) (err error) {
	q_c := []byte{3}
	write_io := bytes.NewBuffer(q_c)
	binary.Write(write_io, binary.BigEndian, uint32(x_q))
	binary.Write(write_io, binary.BigEndian, uint32(z_q))
	err = c.Cdump_leverdb.Put(write_io.Bytes(), data, nil)
	return
}

// 获取全部区块列表
func (c *CDump) GetChainList(need_data bool) (map[[2]uint32][]byte, error) {
	ql := map[[2]uint32][]byte{}
	iter := c.Cdump_leverdb.NewIterator(util.BytesPrefix([]byte{3}), nil)
	for iter.Next() {
		key := iter.Key()
		data := iter.Value()
		key_io := bytes.NewBuffer(key)
		var x_q, z_q uint32
		key_io.Next(1)
		binary.Read(key_io, binary.BigEndian, &x_q)
		binary.Read(key_io, binary.BigEndian, &z_q)
		if need_data {
			ql[[2]uint32{x_q, z_q}] = data
		} else {
			ql[[2]uint32{x_q, z_q}] = []byte{}
		}
		// Use key/value.
		// ...
	}
	iter.Release()
	err := iter.Error()

	return ql, err
}

/*
## parameter 参数
| ID(u8) | 名称 | 类型 | 说明 |
| --- | --- | --- | --- |
| 0 | 保留 | 无 | 这是一个系统保留符号 |
| 1 | name | string | 参数名称 |
| 2 | value | bool | 参数默认值 |
| 3 | type | u8 | 参数类型 |
| 4 | description | string | 参数描述 |
| 5 | operand | u64 | 操作数(不包括可选参数中的操作数)(默认0) |
| 6 | end | 无 | 结束符号 |
> 参数类型级别如下:
> | 值 | 说明 |
> | --- | --- |
> | 0 | 默认 |
> | 1 | 纯优化 |
> | 2 | 普通 |
> | 3 | 危险 |
*/
type Parameter struct {
	Name        string
	Value       bool
	Type        uint8
	Description string
	Operand     uint64
}

func (c *Parameter) Write_Hander(w io.Writer) (err error) {
	_, err = w.Write([]byte{0})
	if err != nil {
		return
	}
	_, err = w.Write([]byte{1})
	if err != nil {
		return
	}
	byte_name := []byte(c.Name)
	binary.Write(w, binary.BigEndian, uint32(len(byte_name)))
	_, err = w.Write(byte_name)
	if err != nil {
		return
	}
	_, err = w.Write([]byte{2})
	if err != nil {
		return
	}
	if c.Value {
		_, err = w.Write([]byte{1})
		if err != nil {
			return
		}
	} else {
		_, err = w.Write([]byte{0})
		if err != nil {
			return
		}
	}
	_, err = w.Write([]byte{3})
	if err != nil {
		return
	}
	_, err = w.Write([]byte{c.Type})
	if err != nil {
		return
	}
	_, err = w.Write([]byte{4})
	if err != nil {
		return
	}
	byte_description := []byte(c.Description)
	binary.Write(w, binary.BigEndian, uint32(len(byte_description)))
	_, err = w.Write(byte_description)
	if err != nil {
		return
	}
	_, err = w.Write([]byte{5})
	if err != nil {
		return
	}
	binary.Write(w, binary.BigEndian, c.Operand)
	_, err = w.Write([]byte{6})
	if err != nil {
		return
	}
	return nil
}
func (c *Parameter) Read_Hander(r io.Reader) error {
	for {
		b := make([]byte, 1)
		_, err := r.Read(b)
		if err != nil {
			return err
		}
		switch b[0] {
		case 0:
			continue
		case 1:
			name_len := make([]byte, 4)
			_, err := r.Read(name_len)
			if err != nil {
				return err
			}
			name := make([]byte, binary.BigEndian.Uint32(name_len))
			_, err = r.Read(name)
			if err != nil {
				return err
			}
			c.Name = string(name)
		case 2:
			value := make([]byte, 1)
			_, err := r.Read(value)
			if err != nil {
				return err
			}
			if value[0] == 1 {
				c.Value = true
			} else {
				c.Value = false
			}
		case 3:
			type_ := make([]byte, 1)
			_, err := r.Read(type_)
			if err != nil {
				return err
			}
			c.Type = type_[0]
		case 4:
			description_len := make([]byte, 4)
			_, err := r.Read(description_len)
			if err != nil {
				return err
			}
			description := make([]byte, binary.BigEndian.Uint32(description_len))
			_, err = r.Read(description)
			if err != nil {
				return err
			}
			c.Description = string(description)
		case 5:
			operand := make([]byte, 8)
			_, err := r.Read(operand)
			if err != nil {
				return err
			}
			c.Operand = binary.BigEndian.Uint64(operand)
		case 6:
			return nil
		default:
			return fmt.Errorf("unknown parameter type %d", b[0])
		}

	}
	return nil
}
