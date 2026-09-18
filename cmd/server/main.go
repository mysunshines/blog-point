package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	v1 "github.com/mysunshines/blog-point/internal/handler/v1"
	"github.com/mysunshines/blog-point/internal/repository"
	"github.com/mysunshines/blog-point/internal/service"
	pb "github.com/mysunshines/blog-point/proto/pb/v1"

	goconfig "github.com/mysunshines/gocommon/config"
	"github.com/mysunshines/gocommon/consul"
	"github.com/mysunshines/gocommon/database"
	"github.com/mysunshines/gocommon/log"
	"github.com/mysunshines/gocommon/metrics"
	"github.com/mysunshines/gocommon/middleware"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sony/gobreaker"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
)

// Version 由构建脚本通过 -ldflags "-X main.Version=xxx" 注入，未注入时默认 "dev"。
var Version = "dev"

// 进程级资源句柄，供退出时统一释放
var (
	metricsCancel context.CancelFunc
	deregister    func() error
)

// Server 积分服务进程（HTTP 探活 + gRPC 业务 + Metrics）
type Server struct {
	cfg        *goconfig.Config
	httpServer *http.Server
	grpcServer *grpc.Server
	cb         *gobreaker.CircuitBreaker
	pointSvc   service.PointService

	// quitCh 供监听 goroutine 在失败时通知 Run 走正常关闭路径
	quitCh chan struct{}
}

// initInfra 初始化外部基础设施（数据库）。
func initInfra(cfg *goconfig.Config) error {
	if err := database.Init(&cfg.Database, cfg.App.Env); err != nil {
		return fmt.Errorf("failed to init database: %v", err)
	}
	return nil
}

// NewServer 依赖装配（限流器 / JWT / 熔断器 / 服务 / 处理器），不做 I/O。
func NewServer(cfg *goconfig.Config) *Server {
	middleware.InitRateLimiter(&cfg.RateLimit)
	middleware.InitJWT(cfg.JWT.Secret)

	cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        cfg.App.Name,
		MaxRequests: 3,
		Interval:    10 * time.Second,
		Timeout:     30 * time.Second,
	})

	db := database.GetDB()
	repo := repository.NewPointRepository(db)
	svc := service.NewPointService(repo, db)

	return &Server{
		cfg:      cfg,
		cb:       cb,
		pointSvc: svc,
		quitCh:   make(chan struct{}),
	}
}

// Run 启动三组监听（HTTP 探活、gRPC、Metrics）并阻塞等待退出信号。
func (s *Server) Run() error {
	go s.runHTTPServer()
	go s.runGRPCServer()
	if goconfig.Get().Metrics.Enabled {
		go s.runMetricsServer()
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-quit:
	case <-s.quitCh:
		log.Errorf("server goroutine failed, initiating shutdown")
	}

	log.Info("Shutting down point-service...")
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if s.httpServer != nil {
		if err := s.httpServer.Shutdown(ctx); err != nil {
			log.Warnf("http server shutdown: %v", err)
		}
	}
	return nil
}

// runHTTPServer 探活端点（/health /ready /version），不承载业务路由。
func (s *Server) runHTTPServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ready"}`))
	})
	mux.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fmt.Sprintf(`{"service":"%s","version":"%s"}`, s.cfg.App.Name, Version)))
	})

	addr := fmt.Sprintf("%s:%d", s.cfg.HTTP.Host, s.cfg.HTTP.Port)
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
	log.Infof("HTTP probe server listening on %s", addr)
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Errorf("HTTP server error: %v", err)
		close(s.quitCh)
	}
}

// runGRPCServer 启动 gRPC 业务入口（唯一业务入口，经 Gateway 反射代理访问）。
func (s *Server) runGRPCServer() {
	addr := fmt.Sprintf("%s:%d", s.cfg.GRPC.Host, s.cfg.GRPC.Port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Errorf("failed to listen gRPC %s: %v", addr, err)
		close(s.quitCh)
		return
	}

	s.grpcServer = grpc.NewServer(
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle:     60 * time.Second,
			MaxConnectionAge:      120 * time.Second,
			MaxConnectionAgeGrace: 30 * time.Second,
			Time:                  30 * time.Second,
		}),
		grpc.ChainUnaryInterceptor(
			middleware.GRPCRecoveryInterceptor(s.cfg.App.Name),
			middleware.GRPCTimeoutInterceptor(s.cfg.App.Name),
			middleware.GRPCCircuitBreakerInterceptor(s.cb),
			middleware.GRPCAuthInterceptor(),
			middleware.GRPCMetricsInterceptor(s.cfg.App.Name),
			middleware.GRPCLoggingInterceptor(),
		),
	)

	pb.RegisterPointServiceServer(s.grpcServer, &v1.GrpcPointHandler{Svc: s.pointSvc, Cb: s.cb})
	// 注册反射服务：Gateway 动态代理依赖反射推导方法
	reflection.Register(s.grpcServer)

	log.Infof("gRPC server listening on %s", addr)
	if err := s.grpcServer.Serve(lis); err != nil {
		log.Errorf("gRPC server error: %v", err)
		close(s.quitCh)
	}
}

// runMetricsServer Prometheus 指标端点（内网抓取）
func (s *Server) runMetricsServer() {
	metricsCtx, cancel := context.WithCancel(context.Background())
	metricsCancel = cancel
	metrics.StartRuntimeMetrics(metricsCtx, 15*time.Second)

	mux := http.NewServeMux()
	mux.Handle(s.cfg.Metrics.Path, promhttp.Handler())
	addr := fmt.Sprintf(":%d", s.cfg.Metrics.Port)
	log.Infof("Metrics server listening on %s%s", addr, s.cfg.Metrics.Path)
	srv := &http.Server{Addr: addr, Handler: mux, ReadTimeout: 10 * time.Second}
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Warnf("metrics server error: %v", err)
	}
}

// shutdown 正常退出：先从 Consul 摘流量，再释放资源
func shutdown() {
	if deregister != nil {
		if err := deregister(); err != nil {
			log.Warnf("consul deregister: %v", err)
		}
	}
	releaseInfra()
}

// releaseInfra 释放全局资源（幂等）
func releaseInfra() {
	if metricsCancel != nil {
		metricsCancel()
	}
	if err := database.Close(); err != nil {
		log.Warnf("database close: %v", err)
	}
	log.StopRotation()
}

// registerToConsul 向 Consul 注册本服务实例，返回取消注册函数。
func registerToConsul(cfg *goconfig.Config) (func() error, error) {
	deregisterFn, err := consul.Register(consul.Registration{
		Name:               cfg.App.Name,
		ConsulAddress:      cfg.Consul.Address,
		GRPCPort:           cfg.GRPC.Port,
		HTTPPort:           cfg.HTTP.Port,
		CheckInterval:      cfg.Consul.CheckInterval,
		DeregisterCritical: cfg.Consul.DeregisterCritical,
		Version:            consul.VersionFromEnv(Version),
		Canary:             consul.CanaryFromEnv(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to register to consul: %v", err)
	}
	return deregisterFn, nil
}

func main() {
	// ① 加载配置（APP_ENV 解析 + 默认值兜底）
	cfg, err := goconfig.LoadByEnv()
	if err != nil {
		fmt.Printf("failed to load config: %v\n", err)
		os.Exit(1)
	}

	// ② 初始化基础设施
	if err := initInfra(cfg); err != nil {
		fmt.Printf("failed to init infra: %v\n", err)
		os.Exit(1)
	}

	// ③ 启用 Consul 服务发现（供调用下游时解析实例）
	consul.UseConsulDiscovery(cfg.Consul.Address)

	// ④ 注册本服务到 Consul（Gateway 由此自动同步 /api/v1/point 路由）
	deregister, err = registerToConsul(cfg)
	if err != nil {
		fmt.Printf("failed to register to consul: %v\n", err)
		releaseInfra()
		os.Exit(1)
	}

	srv := NewServer(cfg)
	if err := srv.Run(); err != nil {
		log.Errorf("server exit: %v", err)
	}
	shutdown()
}
