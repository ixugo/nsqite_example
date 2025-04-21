package nsqiteexample

import (
	"database/sql"
	"fmt"
	"log/slog"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/glebarez/go-sqlite"
	"github.com/ixugo/nsqite"
)

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

	if err := nsqite.SetDB(nsqite.DriverNameSQLite, db).AutoMigrate(); err != nil {
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

	c.WaitMessage()
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
	for i := range 10000 {
		topic := "test-topic-close" + strconv.Itoa(i)
		p := nsqite.NewProducer()
		s1 := nsqite.NewConsumer(topic, "limit-consumer-1", nsqite.WithQueueSize(2))
		s1.AddConcurrentHandlers(nsqite.ConsumerHandlerFunc(func(msg *nsqite.Message) error { return nil }), 1)

		wg.Add(2)
		go func() {
			defer wg.Done()
			for range 3 {
				p.Publish(topic, []byte("msg"))
			}
		}()
		go func() {
			defer wg.Done()
			s1.Stop()
		}()
	}
	time.Sleep(time.Second)
}
