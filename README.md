# nsqite_example
event bus, mq
[nsqite](https://github.com/ixugo/nsqite)



## pgx

[example](./example/postgres/main.go)

```go
import "database/sql"
import "github.com/ixugo/nsqite"
import 	_ "github.com/jackc/pgx/v5/stdlib"

func main(){
	connStr := "postgres://username:password@localhost:5432/dbname?sslmode=disable"
	db, err := sql.Open("pgx", connStr)
	if err != nil {
		log.Fatal(err)
	}

	// init database table
	nsqite.SetDB(db).AutoMigrate()
}
```


## gorm

[example](./example/gorm/main.go)

```go
import "github.com/ixugo/nsqite"

func main(){
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
	// nsq.AutoMigrate or gormDB.AutoMigrate
	// if err := nsq.AutoMigrate(); err != nil {
	// panic(err)
	// }
}

```

## sqlite

```go
import "database/sql"
import "github.com/ixugo/nsqite"
import _ "github.com/mattn/go-sqlite3"


func main(){
	db, err := sql.Open("sqlite3", "./test.db")
	if err != nil {
		log.Fatal(err)
	}
	// init database table
	nsqite.SetDB(db).AutoMigrate()
}
```
