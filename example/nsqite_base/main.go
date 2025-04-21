package main

import (
	"fmt"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/ixugo/nsqite"
	"gorm.io/gorm"
)

func main() {
	gormDB, err := gorm.Open(sqlite.Open("test.db"))
	if err != nil {
		panic(err)
	}
	db, _ := gormDB.DB()

	const topic = "alert"

	nsqite.SetSQLite(db).AutoMigrate()

	p := nsqite.NewProducer()
	c := nsqite.NewConsumer(topic, "ch", nsqite.WithMaxAttempts(10))
	c2 := nsqite.NewConsumer(topic, "ch2", nsqite.WithMaxAttempts(3))

	var i int

	c.AddConcurrentHandlers(nsqite.ConsumerHandlerFunc(func(msg *nsqite.Message) error {
		i++
		fmt.Printf("c1 %d websocket >>> %s \n\n", i, string(msg.Body))
		if i >= 2 {
			fmt.Println("完成")
			return nil
		}
		return fmt.Errorf("err")
	}), 1)

	c2.AddConcurrentHandlers(nsqite.ConsumerHandlerFunc(func(msg *nsqite.Message) error {
		fmt.Printf("c2 websocket >>> %s \n\n", string(msg.Body))
		return nil
	}), 1)

	gormDB.Transaction(func(tx *gorm.DB) error {
		p.PublishTx(func(a *nsqite.Message) error {
			return tx.Create(a).Error
		}, topic, []byte("1111111111"))
		return nil
	})

	time.Sleep(10 * time.Second)
	fmt.Println("end")
}
