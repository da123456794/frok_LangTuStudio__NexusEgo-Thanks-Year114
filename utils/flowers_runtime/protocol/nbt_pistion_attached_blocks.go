package protocol

func (r *Reader) PistonAttachedBlocks(m *[]int32) {
	blocks := make([]BlockPos, 0)
	FuncSliceVarint16Length(r, &blocks, r.BlockPos)
	*m = make([]int32, 0, len(blocks)*3)
	for _, value := range blocks {
		*m = append(*m, value[0], value[1], value[2])
	}
}

func (w *Writer) PistonAttachedBlocks(m *[]int32) {
	blocks := make([]BlockPos, 0, len(*m)/3)
	for i := 0; i+2 < len(*m); i += 3 {
		blocks = append(blocks, BlockPos{(*m)[i], (*m)[i+1], (*m)[i+2]})
	}
	FuncSliceVarint16Length(w, &blocks, w.BlockPos)
}
