package provider

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/LangTuStudio/Conbit/utils/game_saves/bedrock_level/operation"
	"github.com/LangTuStudio/Conbit/utils/level_db_trim"
	"github.com/df-mc/goleveldb/leveldb/opt"
)

type BedrockWorld struct {
	*operation.BedrockWorld
	dir  string
	ldat *Data
	db   *level_db_trim.LevelDBTrimLock
}

func OpenWorld(dir string, options *opt.Options) (World, error) {
	_ = os.MkdirAll(filepath.Join(dir, "db"), 0o777)
	if options == nil {
		options = new(opt.Options)
	}

	var ldat *Data
	if _, err := os.Stat(filepath.Join(dir, "level.dat")); err != nil {
		if os.IsNotExist(err) {
			defaultData := InitDefaultLevelDat()
			ldat = &defaultData
		} else {
			return nil, err
		}
	} else {
		ld, err := ReadLevelDatFile(dir)
		if err != nil {
			return nil, err
		}
		ldat = &Data{}
		if err := ld.Unmarshal(ldat); err != nil {
			return nil, err
		}
	}

	ldb, err := level_db_trim.NewLevelDBTrimLock(filepath.Join(dir, "db"), (*level_db_trim.LevelDBTrimLockConfig)(options))
	if err != nil {
		return nil, err
	}
	db := &trimDB{db: ldb}
	world := &operation.BedrockWorld{DB: db}
	return &BedrockWorld{
		BedrockWorld: world,
		dir:          dir,
		ldat:         ldat,
		db:           ldb,
	}, nil
}

func (db *BedrockWorld) LevelDat() *Data {
	return db.ldat
}

func (db *BedrockWorld) UpdateLevelDat() error {
	var ldat LevelDat
	if err := ldat.Marshal(*db.ldat); err != nil {
		return fmt.Errorf("error closing level.dat: %w", err)
	}
	if err := ldat.WriteFile(db.dir); err != nil {
		return fmt.Errorf("error closing level.dat: %w", err)
	}
	if err := os.WriteFile(filepath.Join(db.dir, "levelname.txt"), []byte(db.ldat.LevelName), 0o644); err != nil {
		return fmt.Errorf("error writing levelname.txt: %w", err)
	}
	return nil
}

func (db *BedrockWorld) CloseWorld() error {
	db.ldat.LastPlayed = time.Now().Unix()
	if err := db.UpdateLevelDat(); err != nil {
		return err
	}
	return db.Close()
}

type trimDB struct {
	db *level_db_trim.LevelDBTrimLock
}

func (t *trimDB) Close() error {
	return t.db.Close()
}

func (t *trimDB) Delete(key []byte) error {
	return t.db.Delete(key, nil)
}

func (t *trimDB) Get(key []byte) ([]byte, error) {
	value, err := t.db.Get(key, nil)
	if err != nil {
		return nil, err
	}
	return value, nil
}

func (t *trimDB) Has(key []byte) (bool, error) {
	return t.db.Has(key, nil)
}

func (t *trimDB) Put(key []byte, value []byte) error {
	return t.db.Put(key, value, nil)
}

func (t *trimDB) IterAll(fn IterAllFunc) error {
	iter := t.db.NewIterator(nil, nil)
	defer iter.Release()
	for iter.Next() {
		key := append([]byte(nil), iter.Key()...)
		value := append([]byte(nil), iter.Value()...)
		if !fn(key, value) {
			break
		}
	}
	if err := iter.Error(); err != nil {
		return err
	}
	return nil
}
