package cdump

import (
	"encoding/binary"
	"io"
)

/*
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
*/
func (c *CDump) writeHeader(w io.Writer) error {
	err := c.writeVersion(w)
	if err != nil {
		return err
	}
	err = c.writeParameter(w)
	if err != nil {
		return err
	}
	err = c.writeAuthor(w)
	if err != nil {
		return err
	}
	err = c.writeDescription(w)
	if err != nil {
		return err
	}
	err = c.writeUserID(w)
	if err != nil {
		return err
	}
	err = c.writeFileChecksum(w)
	if err != nil {
		return err
	}
	err = c.writeOperand(w)
	if err != nil {
		return err
	}
	err = c.writeBuildXYZ(w)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte{9})
	if err != nil {
		return err
	}
	return nil
}
func (c *CDump) writeVersion(w io.Writer) error {
	_, err := w.Write([]byte{1})
	if err != nil {
		return err
	}
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, c.Version)
	_, err = w.Write(buf)
	if err != nil {
		return err
	}
	return nil
}
func (c *CDump) writeParameter(w io.Writer) error {
	_, err := w.Write([]byte{2})
	if err != nil {
		return err
	}
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, uint32(len(c.Parameter)))
	_, err = w.Write(buf)
	if err != nil {
		return err
	}
	for _, p := range c.Parameter {
		err = p.Write(w)
		if err != nil {
			return err
		}
	}
	return nil
}
func (c *CDump) writeAuthor(w io.Writer) error {
	_, err := w.Write([]byte{3})
	if err != nil {
		return err
	}
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, uint32(len(c.Author)))
	_, err = w.Write(buf)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(c.Author))
	if err != nil {
		return err
	}
	return nil
}
func (c *CDump) writeDescription(w io.Writer) error {
	_, err := w.Write([]byte{4})
	if err != nil {
		return err
	}
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, uint32(len(c.Description)))
	_, err = w.Write(buf)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(c.Description))
	if err != nil {
		return err
	}
	return nil
}
func (c *CDump) writeUserID(w io.Writer) error {
	_, err := w.Write([]byte{5})
	if err != nil {
		return err
	}
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, uint32(len(c.UserID)))
	_, err = w.Write(buf)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(c.UserID))
	if err != nil {
		return err
	}
	return nil
}
func (c *CDump) writeFileChecksum(w io.Writer) error {
	_, err := w.Write([]byte{6})
	if err != nil {
		return err
	}
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, uint32(len(c.FileChecksum)))
	_, err = w.Write(buf)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(c.FileChecksum))
	if err != nil {
		return err
	}
	return nil
}
func (c *CDump) writeOperand(w io.Writer) error {
	_, err := w.Write([]byte{7})
	if err != nil {
		return err
	}
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, c.Operand)
	_, err = w.Write(buf)
	if err != nil {
		return err
	}
	return nil
}
func (c *CDump) writeBuildXYZ(w io.Writer) error {
	_, err := w.Write([]byte{8})
	if err != nil {
		return err
	}
	buf := make([]byte, 24)
	binary.BigEndian.PutUint64(buf[0:8], c.BuildXYZ[0])
	binary.BigEndian.PutUint64(buf[8:16], c.BuildXYZ[1])
	binary.BigEndian.PutUint64(buf[16:24], c.BuildXYZ[2])
	_, err = w.Write(buf)
	if err != nil {
		return err
	}
	return nil
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
*/
func (p *Parameter) Write(w io.Writer) error {
	_, err := w.Write([]byte{1})
	if err != nil {
		return err
	}
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, uint32(len(p.Name)))
	_, err = w.Write(buf)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(p.Name))
	if err != nil {
		return err
	}
	_, err = w.Write([]byte{2})
	if err != nil {
		return err
	}
	if p.Value {
		_, err = w.Write([]byte{1})
	} else {
		_, err = w.Write([]byte{0})
	}
	if err != nil {
		return err
	}
	_, err = w.Write([]byte{3})
	if err != nil {
		return err
	}
	_, err = w.Write([]byte{p.Type})
	if err != nil {
		return err
	}
	_, err = w.Write([]byte{4})
	if err != nil {
		return err
	}
	buf = make([]byte, 4)
	binary.BigEndian.PutUint32(buf, uint32(len(p.Description)))
	_, err = w.Write(buf)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(p.Description))
	if err != nil {
		return err
	}
	_, err = w.Write([]byte{5})
	if err != nil {
		return err
	}
	buf = make([]byte, 8)
	binary.BigEndian.PutUint64(buf, p.Operand)
	_, err = w.Write(buf)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte{6})
	if err != nil {
		return err
	}
	return nil
}
