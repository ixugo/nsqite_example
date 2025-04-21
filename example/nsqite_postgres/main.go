package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	// 数据库连接字符串
	connStr := "postgres://username:password@localhost:5432/dbname?sslmode=disable"

	// 使用 pgx 驱动打开数据库连接
	db, err := sql.Open("pgx", connStr)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// 测试连接
	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("数据库连接成功！")
}
