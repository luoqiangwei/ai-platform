package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ai-platform/core"
	"ai-platform/services"
	"ai-platform/utils"
	"ai-platform/web"
)

var Version = "dev" // Default version

// ExampleCustomService shows how to write a custom service.
type ExampleCustomService struct {
	status string
}

func (e *ExampleCustomService) Name() string {
	return "Data-Processing-Service"
}

func (e *ExampleCustomService) Start(ctx context.Context) error {
	e.status = "Initializing..."
	time.Sleep(1 * time.Second)
	e.status = "Processing data normally"

	// Keep running until context is canceled
	<-ctx.Done()
	return nil
}

func (e *ExampleCustomService) Stop() error {
	e.status = "Stopped"
	return nil
}

func (e *ExampleCustomService) Status() string {
	return e.status
}

func main() {
	// 1. Create a context that listens for system interruption signals (e.g., Ctrl+C)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 2. Initialize the service manager
	manager := core.NewManager()

	// 3. Register custom services
	customSrv := &ExampleCustomService{status: "Pending"}
	manager.Register(customSrv)

	// 4. Register a timer service (runs a task every 5 seconds)
	timerTask := func() {
		fmt.Printf("[%s] Timer ticked. Doing some background work...\n", time.Now().Format(time.RFC3339))
		// You can use utils.HashMD5("data") or DB connections here
	}
	timerSrv := utils.NewTimerService("Log-Cleanup-Timer", 5*time.Second, timerTask)
	manager.Register(timerSrv)

	appID := os.Getenv("LARK_APP_ID")
	appSecret := os.Getenv("LARK_APP_SECRET")

	if appID == "" || appSecret == "" {
		fmt.Println("错误: 请设置 LARK_APP_ID 和 LARK_APP_SECRET 环境变量")
		os.Exit(1)
	}

	aiSrv := services.NewAIService()
	manager.Register(aiSrv)

	// 2. 初始化 Dashboard，并把 larkSrv 的处理器传进去
	larkSrv := services.NewLarkService(appID, appSecret, aiSrv)
	manager.Register(larkSrv)

	dashboardSrv := web.NewDashboardService(":8080", Version, manager)
	manager.Register(dashboardSrv)

	// 6. Start all services concurrently
	fmt.Println("Starting all services...")
	manager.StartAll(ctx)

	// 7. Setup graceful shutdown listener
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Wait for interrupt signal
	<-sigCh
	fmt.Println("\nShutdown signal received. Stopping services...")

	// Cancel the context to signal all services to stop
	cancel()

	go func() {
		time.Sleep(10 * time.Second)
		fmt.Println("Shutdown timed out, forcing exit.")
		os.Exit(1)
	}()

	// Wait for all goroutines to finish
	manager.Wait()
	fmt.Println("All services stopped. Exiting.")
}
