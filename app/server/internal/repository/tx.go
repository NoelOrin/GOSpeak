package repository

import "gorm.io/gorm"

// TxRunner 封装事务边界：service 层通过它开启事务，不直持 *gorm.DB。
// 事务回调内用 repo 的 WithTx(tx) 副本操作，保证写入与事务绑定。
type TxRunner interface {
	WithinTx(fn func(tx *gorm.DB) error) error
}

type dbTxRunner struct{ db *gorm.DB }

// NewDBTxRunner 返回基于 GORM Transaction 的默认 TxRunner 实现。
func NewDBTxRunner(db *gorm.DB) TxRunner {
	return &dbTxRunner{db: db}
}

func (r *dbTxRunner) WithinTx(fn func(tx *gorm.DB) error) error {
	return r.db.Transaction(fn)
}
