package level_db_trim

import (
	"fmt"

	"github.com/df-mc/goleveldb/leveldb/storage"
)

type ErrCorrupted struct {
	Fd  FileDesc
	Err error
}

func (e *ErrCorrupted) Error() string {
	if !e.Fd.Zero() {
		return fmt.Sprintf("%v [file=%v]", e.Err, e.Fd)
	}
	return e.Err.Error()
}

func isCorrupted(err error) bool {
	_, ok := err.(*ErrCorrupted)
	return ok
}

type Reader = storage.Reader
type Writer = storage.Writer
