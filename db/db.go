package db

import (
	"fmt"
	"time"

	"github.com/vulsio/go-cpe-dictionary/models"
	"golang.org/x/xerrors"
)

// DB is interface for a database driver
type DB interface {
	Name() string
	OpenDB(dbType, dbPath string, debugSQL bool, option Option) (bool, error)
	CloseDB() error
	MigrateDB() error

	IsGoCPEDictModelV1() (bool, error)
	GetFetchMeta() (*models.FetchMeta, error)
	UpsertFetchMeta(*models.FetchMeta) error

	GetVendorProducts() ([]models.VendorProduct, error)
	GetCpesByVendorProduct(string, string) ([]string, []string, error)
	InsertCpes(models.FetchType, []models.CategorizedCpe) error
	IsDeprecated(string) (bool, error)
}

type Option struct {
	RedisTimeout time.Duration
}

// NewDB returns db driver
func NewDB(dbType string, dbPath string, debugSQL bool, option Option) (driver DB, locked bool, err error) {
	if driver, err = newDB(dbType); err != nil {
		return driver, false, xerrors.Errorf("Failed to new db. err: %w", err)
	}

	if locked, err := driver.OpenDB(dbType, dbPath, debugSQL, option); err != nil {
		if locked {
			return nil, true, err
		}
		return nil, false, err
	}

	isV1, err := driver.IsGoCPEDictModelV1()
	if err != nil {
		return nil, false, xerrors.Errorf("Failed to IsGoCPEDictModelV1. err: %w", err)
	}
	if isV1 {
		return nil, false, xerrors.New("Failed to NewDB. Since SchemaVersion is incompatible, delete Database and fetch again.")
	}

	if err := driver.MigrateDB(); err != nil {
		return driver, false, xerrors.Errorf("Failed to migrate db. err: %w", err)
	}
	return driver, false, nil
}

func newDB(dbType string) (DB, error) {
	switch dbType {
	case dialectSqlite3, dialectMysql, dialectPostgreSQL:
		return &RDBDriver{name: dbType}, nil
	case dialectRedis:
		return &RedisDriver{name: dbType}, nil
	}
	return nil, fmt.Errorf("Invalid database dialect, %s", dbType)
}

// IndexChunk has a starting point and an ending point for Chunk
type IndexChunk struct {
	From, To int
}

func chunkSlice(length int, chunkSize int) <-chan IndexChunk {
	ch := make(chan IndexChunk)

	go func() {
		defer close(ch)

		for i := 0; i < length; i += chunkSize {
			idx := IndexChunk{i, i + chunkSize}
			if length < idx.To {
				idx.To = length
			}
			ch <- idx
		}
	}()

	return ch
}
