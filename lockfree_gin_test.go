package logrotatex

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"gitee.com/MM-Q/logrotatex/lockfree"
	"github.com/gin-gonic/gin"
)

// TestLockFreeWriterWithGin 使用无锁缓冲区写入器启动Gin服务器进行性能测试
func TestLockFreeWriterWithGin(t *testing.T) {
	// 确保日志目录存在
	logDir := "logs"
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		os.MkdirAll(logDir, 0755)
	}

	// 创建LogRotateX实例作为底层写入器
	rotator := Default()

	// 创建无锁缓冲区写入器
	lockfreeWriter := lockfree.NewLockFreeBufferedWriter(rotator, &lockfree.LockFreeWriterConfig{
		BufferSize:    1024 * 1024, // 1MB缓冲区
		FlushInterval: 1 * time.Second,
		BatchSize:     1024,
		MaxRetries:    3,
	})

	// 设置Gin为发布模式
	gin.SetMode(gin.ReleaseMode)

	// 将无锁缓冲区写入器设置为Gin的默认日志写入器
	gin.DefaultWriter = io.MultiWriter(os.Stdout, lockfreeWriter)

	// 创建Gin引擎
	r := gin.Default()

	// ping接口
	r.GET("/ping", func(c *gin.Context) {
		// 记录自定义日志
		lockfreeWriter.Write([]byte(fmt.Sprintf("[%s] 处理ping请求\n", time.Now().Format(time.RFC3339))))

		// 返回响应
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
			"time":    time.Now().Format(time.RFC3339),
		})
	})

	// 启动服务器
	t.Log("服务器启动在 http://localhost:8080")
	t.Log("可用接口:")
	t.Log("  GET /ping   - ping接口")
	t.Log("\n使用方法:")
	t.Log("1. 使用 curl 或浏览器访问 http://localhost:8080/ping 进行测试")
	t.Log("2. 使用压力测试工具 (如 wrk, ab) 测试性能")
	t.Log("   例如: go-wrk -c 10 -d 10  http://localhost:8080/ping")

	// 启动服务器
	go func() {
		if err := r.Run(":8080"); err != nil {
			t.Errorf("启动服务器失败: %v", err)
		}
	}()

	// 等待服务器启动
	time.Sleep(2 * time.Second)

	// 等待用户测试
	t.Log("服务器已启动，请进行性能测试...")
	t.Log("测试完成后，按任意键继续...")

	// 这里我们等待一段时间，让用户有足够的时间进行测试
	// 在实际使用中，用户可以手动终止测试
	time.Sleep(60 * time.Second) // 等待60秒

	// 关闭写入器
	lockfreeWriter.Close()
	rotator.Close()

	t.Log("测试完成")
}
