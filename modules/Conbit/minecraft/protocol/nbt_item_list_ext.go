package protocol

import "bytes"

// EnchantList reads a length-prefixed enchant list.
func (r *Reader) EnchantList(m *[]Enchant) {
	var bufferLength uint16
	r.Varuint16(&bufferLength)
	if bufferLength == 0 {
		return
	}
	SliceVarint16Length(r, m)
}

// ItemList reads a byte-slice wrapped item list.
func (r *Reader) ItemList(m *[]ItemWithSlot) {
	var newBytes []byte
	r.ByteSlice(&newBytes)

	buffer := bytes.NewBuffer(newBytes)
	reader := NewReader(buffer, 0, false)

	if len(buffer.Bytes()) == 0 {
		return
	}
	*m = make([]ItemWithSlot, 0)

	for len(buffer.Bytes()) > 0 {
		newItem := ItemWithSlot{}
		newItem.Marshal(reader)
		*m = append(*m, newItem)
	}
}

// EnchantList writes a length-prefixed enchant list.
func (w *Writer) EnchantList(x *[]Enchant) {
	var bufferLength uint16

	if x == nil || len(*x) == 0 {
		w.Varuint16(&bufferLength)
		return
	}

	buffer := bytes.NewBuffer([]byte{})
	SliceVarint16Length(NewWriter(buffer, 0), x)

	bufferLength = uint16(buffer.Len())
	w.Varuint16(&bufferLength)
	_, _ = w.w.Write(buffer.Bytes())
}

// ItemList writes a byte-slice wrapped item list.
func (w *Writer) ItemList(x *[]ItemWithSlot) {
	var length uint32
	if x == nil || len(*x) == 0 {
		w.Varuint32(&length)
		return
	}

	buffer := bytes.NewBuffer([]byte{})
	SliceOfLen(NewWriter(buffer, 0), uint32(len(*x)), x)

	length = uint32(buffer.Len())
	w.Varuint32(&length)
	_, _ = w.w.Write(buffer.Bytes())
}
