package nsqiteexample

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/glebarez/go-sqlite"
	"github.com/ixugo/nsqite"
)

var nsqdb *nsqite.DB

func initDB() {
	// slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
	// 	Level:     slog.LevelDebug,
	// 	AddSource: true,
	// })))

	db, err := sql.Open("sqlite", "test.db")
	if err != nil {
		panic(err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	nsqdb = nsqite.SetDB(nsqite.DriverNameSQLite, db)
	if err := nsqdb.AutoMigrate(); err != nil {
		panic(err)
	}
}

// TestNSQite 测试消费消息
func TestNSQite(t *testing.T) {
	initDB()

	const topic = "test"
	const messageBody = "hello world"

	// 2. 创建生产者和消费者
	p := nsqite.NewProducer()
	c := nsqite.NewConsumer(topic, "test-channel")

	// 3. 设置消息处理函数
	done := make(chan bool)
	c.AddConcurrentHandlers(nsqite.ConsumerHandlerFunc(func(msg *nsqite.Message) error {
		if string(msg.Body) != messageBody {
			t.Errorf("expected message body %s, got %s", messageBody, string(msg.Body))
		}
		done <- true
		return nil
	}), 1)

	// 4. 发布消息
	if err := p.Publish(topic, []byte(messageBody)); err != nil {
		t.Fatal(err)
	}

	// 5. 等待消息处理完成
	select {
	case <-done:
		// 测试通过
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for message processing")
	}

	time.Sleep(time.Second)
}

// TestMaxAttempts 测试最大重试次数
func TestMaxAttempts(t *testing.T) {
	initDB()
	const topic = "test-max-attempts"
	const messageBody = "test message"

	p := nsqite.NewProducer()
	c := nsqite.NewConsumer(topic, "test-channel", nsqite.WithMaxAttempts(3))

	attempts := uint32(0)
	c.AddConcurrentHandlers(nsqite.ConsumerHandlerFunc(func(msg *nsqite.Message) error {
		msg.DisableAutoResponse()
		atomic.AddUint32(&attempts, 1)
		msg.Requeue(0)
		return fmt.Errorf("simulated error")
	}), 1)

	if err := p.Publish(topic, []byte(messageBody)); err != nil {
		t.Fatal(err)
	}

	<-c.WaitMessage()
	time.Sleep(10 * time.Second)

	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func BenchmarkNSQite(b *testing.B) {
	initDB()
	slog.SetLogLoggerLevel(slog.LevelError)

	const topic = "benchmark-topic"
	const messageBody = "test message"

	p := nsqite.NewProducer()
	c := nsqite.NewConsumer(topic, "benchmark-channel")
	c2 := nsqite.NewConsumer(topic, "benchmark-channel2")

	// 计数器和完成信号
	var counter int32
	done := make(chan struct{})
	var once sync.Once

	// 添加消息处理器
	c.AddConcurrentHandlers(nsqite.ConsumerHandlerFunc(func(msg *nsqite.Message) error {
		if atomic.AddInt32(&counter, 1) >= int32(b.N) {
			once.Do(func() {
				close(done)
			})
		}
		return nil
	}), int32(runtime.NumCPU()))

	c2.AddConcurrentHandlers(nsqite.ConsumerHandlerFunc(func(msg *nsqite.Message) error { return nil }), int32(runtime.NumCPU()))

	b.ResetTimer() // 重置计时器，不计算初始化时间

	// 发布消息
	for i := 0; i < b.N; i++ {
		if err := p.Publish(topic, []byte(messageBody)); err != nil {
			b.Fatal(err)
		}
	}

	// 等待所有消息处理完成
	select {
	case <-done:
		// 所有消息已处理
	case <-time.After(10 * time.Second):
		b.Fatal("基准测试超时")
	}
	b.StopTimer() // 停止计时器

	// 输出统计信息
	b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "msgs/sec")
}

func TestNSQiteClose(t *testing.T) {
	initDB()
	var wg sync.WaitGroup

	for i := range 1000 {
		topic := "test-topic-close" + strconv.Itoa(i)
		p := nsqite.NewProducer()
		s1 := nsqite.NewConsumer(topic, "limit-consumer-1", nsqite.WithQueueSize(3))
		s1.AddConcurrentHandlers(nsqite.ConsumerHandlerFunc(func(msg *nsqite.Message) error {
			fmt.Println(">>>", string(msg.Body))
			return nil
		}), 1)

		wg.Add(3)
		go func() {
			defer wg.Done()
			for range 3 {
				if err := p.Publish(topic, []byte("msg")); err != nil {
					fmt.Println(err)
				}
			}
		}()
		go func() {
			defer wg.Done()
			if err := p.Publish(topic, []byte("msg")); err != nil {
				fmt.Println(err)
			}
			s1.Stop()
		}()

		go func() {
			defer wg.Done()
			select {
			case <-s1.WaitMessage():
			case <-time.After(5 * time.Second):
				fmt.Println(">>> timeout")
			}
		}()
	}
	wg.Wait()
	time.Sleep(5 * time.Second)
}

func TestDeleteCompletedMessagesOlderThan(t *testing.T) {
	os.Remove("test.db")
	TestNSQite(t)

	count, err := nsqdb.Count()
	if err != nil {
		t.Fatalf("查询数据行数失败: %v", err)
	}
	if count != 1 {
		t.Fatalf("期望数据行数为1，但得到的是: %d", count)
	}
	nsqdb.DeleteCompletedMessagesOlderThan(0)

	count, err = nsqdb.Count()
	if err != nil {
		t.Fatalf("查询数据行数失败: %v", err)
	}
	if count != 0 {
		t.Fatalf("期望数据行数为0，但得到的是: %d", count)
	}
}
