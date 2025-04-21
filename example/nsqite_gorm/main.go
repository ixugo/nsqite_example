package main

import (
	"github.com/glebarez/sqlite"
	"github.com/ixugo/nsqite"
	"gorm.io/gorm"
)

func main() {
	gormDB, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}
	db, _ := gormDB.DB()

	if err := gormDB.AutoMigrate(new(nsqite.Message)); err != nil {
		panic(err)
	}
	nsq := nsqite.SetDB("sqlite", db)
	_ = nsq

	// create table
	// if err := nsq.AutoMigrate(); err != nil {
	// panic(err)
	// }
}
