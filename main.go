package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	store, err := LoadStore("autogreen.json")
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	manager := NewManager(store)
	manager.Start()

	addr := os.Getenv("AUTOGREEN_LISTEN")
	if addr == "" {
		addr = ":8080"
	} else if !strings.Contains(addr, ":") {
		// 允许只填端口号，自动补全为 :端口
		addr = ":" + addr
	}

	handler := NewServer(store, manager)

	srv := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	go func() {
		log.Printf("autogreen 后台已启动，访问 http://localhost%s", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP 服务启动失败: %v", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	log.Println("正在停止...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	manager.StopAll()
	log.Println("autogreen 已停止")
}
