package level_db_trim

import (
	"fmt"

	"github.com/df-mc/goleveldb/leveldb"
	"github.com/df-mc/goleveldb/leveldb/iterator"
	"github.com/df-mc/goleveldb/leveldb/opt"
	"github.com/df-mc/goleveldb/leveldb/storage"
	"github.com/df-mc/goleveldb/leveldb/util"
)

// LevelDBTrimLockConfig 与 leveldb/opt.Options 共享布局。
type LevelDBTrimLockConfig opt.Options

func (c *LevelDBTrimLockConfig) options() *opt.Options {
	if c == nil {
		return nil
	}
	return (*opt.Options)(c)
}

// LevelDBTrimLock 封装 LevelDB 实例并管理底层存储。
type LevelDBTrimLock struct {
	DB      *leveldb.DB
	storage storage.Storage
}

func NewLevelDBTrimLock(path string, cfg *LevelDBTrimLockConfig) (*LevelDBTrimLock, error) {
	options := cfg.options()
	readOnly := false
	if options != nil {
		readOnly = options.ReadOnly
	}

	st, err := OpenFile(path, readOnly)
	if err != nil {
		return nil, err
	}
	db, err := leveldb.Open(st, options)
	if err != nil {
		_ = st.Close()
		st, err = OpenFile(path, readOnly)
		if err != nil {
			return nil, err
		}
		db, err = leveldb.Recover(st, options)
		if err != nil {
			_ = st.Close()
			return nil, err
		}
	}

	return &LevelDBTrimLock{DB: db, storage: st}, nil
}

func (l *LevelDBTrimLock) Close() error {
	if l == nil {
		return nil
	}
	var dbErr error
	if l.DB != nil {
		dbErr = l.DB.Close()
	}
	var storageErr error
	if l.storage != nil {
		storageErr = l.storage.Close()
	}
	if dbErr != nil && storageErr != nil {
		return fmt.Errorf("level db close err: %v storage close err: %v", dbErr, storageErr)
	}
	if dbErr != nil {
		return dbErr
	}
	return storageErr
}

func (l *LevelDBTrimLock) Get(key []byte, ro *opt.ReadOptions) ([]byte, error) {
	return l.DB.Get(key, ro)
}

func (l *LevelDBTrimLock) Has(key []byte, ro *opt.ReadOptions) (bool, error) {
	return l.DB.Has(key, ro)
}

func (l *LevelDBTrimLock) Put(key, value []byte, wo *opt.WriteOptions) error {
	return l.DB.Put(key, value, wo)
}

func (l *LevelDBTrimLock) Delete(key []byte, wo *opt.WriteOptions) error {
	return l.DB.Delete(key, wo)
}

func (l *LevelDBTrimLock) Write(batch *leveldb.Batch, wo *opt.WriteOptions) error {
	return l.DB.Write(batch, wo)
}

func (l *LevelDBTrimLock) NewIterator(slice *util.Range, ro *opt.ReadOptions) iterator.Iterator {
	return l.DB.NewIterator(slice, ro)
}

func (l *LevelDBTrimLock) GetSnapshot() (*leveldb.Snapshot, error) {
	return l.DB.GetSnapshot()
}

func (l *LevelDBTrimLock) GetProperty(name string) (string, bool) {
	value, err := l.DB.GetProperty(name)
	if err != nil {
		return "", false
	}
	return value, true
}

func (l *LevelDBTrimLock) OpenTransaction() (*leveldb.Transaction, error) {
	return l.DB.OpenTransaction()
}

func (l *LevelDBTrimLock) CompactRange(r util.Range) error {
	return l.DB.CompactRange(r)
}

func (l *LevelDBTrimLock) Stats(stats *leveldb.DBStats) error {
	return l.DB.Stats(stats)
}

func (l *LevelDBTrimLock) SizeOf(ranges []util.Range) ([]int64, error) {
	return l.DB.SizeOf(ranges)
}

func (l *LevelDBTrimLock) SetReadOnly() error {
	return l.DB.SetReadOnly()
}
