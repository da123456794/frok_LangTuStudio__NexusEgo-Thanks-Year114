package protocol

// Int describes the integer data types for __tag NBT network transmission.
type Int interface {
	uint16 | uint32 | uint64 | int16 | int32 | int64
}

// TAGNumber describes the integer types allowed in standard NBT.
type TAGNumber interface {
	uint8 | int16 | int32 | int64
}

// NBTInt reads/writes data from/to x using the T1 data type for __tag NBT network transmission.
func NBTInt[T1 Int, T2 TAGNumber](x *T2, f func(*T1)) {
	t2 := T1(*x)
	f(&t2)
	*x = T2(t2)
}

// NBTSlice reads/writes a []any using function f.
func NBTSlice[T any](r IO, x *[]any, f func(*[]T)) {
	if _, isReader := r.(*Reader); isReader {
		newVal := make([]T, 0)
		f(&newVal)
		*x = make([]any, len(newVal))
		for i, v := range newVal {
			(*x)[i] = v
		}
		return
	}

	newVal := make([]T, len(*x))
	for i, v := range *x {
		newVal[i] = v.(T)
	}
	f(&newVal)
}
