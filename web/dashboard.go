package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"ai-platform/core"

	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
)

type DashboardService struct {
	addr        string
	version     string
	manager     *core.Manager
	server      *http.Server
	larkHandler *dispatcher.EventDispatcher // 存一下 Lark 处理器
}

// 修改构造函数，增加 larkHandler 参数
func NewDashboardService(addr string, version string, manager *core.Manager) *DashboardService {
	return &DashboardService{
		addr:    addr,
		version: version,
		manager: manager,
	}
}

func (s *DashboardService) Name() string {
	return "Web-Dashboard"
}

// 这里的参数回归标准接口，内部使用 s.larkHandler
func (s *DashboardService) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	// 注册路由，指向下面的方法
	mux.HandleFunc("/api/status", s.statusHandler)
	mux.HandleFunc("/", s.uiHandler)

	s.server = &http.Server{
		Addr:    s.addr,
		Handler: mux,
	}

	go func() {
		fmt.Printf("[Web] Dashboard starting on %s\n", s.addr)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Web server error: %v\n", err)
		}
	}()

	<-ctx.Done()
	return s.Stop()
}

// 提取出来的 Status 处理方法
func (s *DashboardService) statusHandler(w http.ResponseWriter, r *http.Request) {
	statuses := s.manager.GetStatuses()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(statuses)
}

// 提取出来的 UI 处理方法
func (s *DashboardService) uiHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, htmlTemplate, s.version)
}

func (s *DashboardService) Stop() error {
	if s.server != nil {
		fmt.Printf("[Web] Shutting down dashboard on %s\n", s.addr)
		return s.server.Shutdown(context.Background())
	}
	return nil
}

func (s *DashboardService) Status() string {
	return fmt.Sprintf("Online on %s", s.addr)
}

// Added %s placeholder for version and improved styling
const htmlTemplate = `
<!DOCTYPE html>
<html>
<head>
    <title>AI Platform Monitor</title>
    <style>
        body { font-family: 'Segoe UI', Tahoma, sans-serif; background-color: #f0f2f5; padding: 20px; color: #333; }
        .header { display: flex; justify-content: space-between; align-items: center; background: #fff; padding: 10px 20px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); margin-bottom: 20px; }
        .version { background: #1a73e8; color: white; padding: 2px 8px; border-radius: 4px; font-size: 0.8em; }
        .card { background: white; padding: 15px 20px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.05); margin-bottom: 10px; display: flex; justify-content: space-between; align-items: center; }
        .service-name { font-weight: 600; color: #1a73e8; }
        .service-status { font-family: 'Courier New', monospace; background: #e6f4ea; color: #1e8e3e; padding: 4px 8px; border-radius: 4px; font-size: 0.9em; }
    </style>
</head>
<body>
    <div class="header">
        <h1>Service Monitor</h1>
        <span class="version">Version: %s</span>
    </div>
    <div id="services-container">Connecting to backend...</div>

    <script>
        function fetchStatus() {
            fetch('/api/status')
                .then(res => res.json())
                .then(data => {
                    const container = document.getElementById('services-container');
                    container.innerHTML = '';
                    for (const [name, status] of Object.entries(data)) {
                        const div = document.createElement('div');
                        div.className = 'card';
                        div.innerHTML = '<span class="service-name">' + name + '</span>' + 
                                        '<span class="service-status">' + status + '</span>';
                        container.appendChild(div);
                    }
                })
                .catch(err => console.error('Fetch error:', err));
        }
        setInterval(fetchStatus, 2000);
        fetchStatus();
    </script>
</body>
</html>
`
