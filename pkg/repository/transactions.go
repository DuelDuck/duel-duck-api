package repository

import "github.com/uptrace/bun"

// DBConn is implemented by both *bun.DB and bun.Tx
type DBConn interface {
	bun.IDB
	bun.IConn
}

var _ DBConn = (*bun.DB)(nil)
var _ DBConn = bun.Tx{}

// DBWrapper implements DB interface and is used to
// create a new instance of itself, but with
// bun.Tx inside DBConn, so query executes inside a transaction
type DBWrapper struct {
	DBConn
}

func NewDBWrapper(db *bun.DB) DBWrapper {
	return DBWrapper{
		DBConn: db,
	}
}

func (w DBWrapper) WithTx(tx bun.Tx) DB {
	return &DBWrapper{
		DBConn: tx,
	}
}

type DB interface {
	DBConn
	WithTx(tx bun.Tx) DB
}

var _ DB = DBWrapper{}
