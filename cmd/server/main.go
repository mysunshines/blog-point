package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/mysunshines/blog-point/internal/client"
	v0 "github.com/mysunshines/blog-point/internal/handler/v0"
	v1 "github.com/mysunshines/blog-point/internal/handler/v1"
	"github.com/mysunshines/blog-point/internal/repository"
	"github.com/mysunshines/blog-point/internal/service"
	v0pb "github.com/mysunshines/blog-point/proto/pb/v0"
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
	serviceName   string
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
	// 注册 v0 摄入服务（point.v0.PointIngestService）：仅内网服务经 Consul 直连调用，
	// 公网网关 DeriveGRPCService 仅硬编码 v1，故 v0 天然不进公网入口（纵深防御）。
	v0pb.RegisterPointIngestServiceServer(s.grpcServer, &v0.GrpcPointHandler{Svc: s.pointSvc})
	// 注册反射服务：Gateway 动态代理依赖反射推导方法（仅枚举 point.v1.PointService）
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
	// 顶层兜底：panic 与 run 返回 err 两条路径收敛到同一个出口，
	// 自然走到 defer 统一释放资源（避免中途 log.Fatalf/os.Exit 跳过 defer）。
	var runErr error
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("panic recovered in main: %v\n%s", r, debug.Stack())
			runErr = fmt.Errorf("panic: %v", r)
		}
		if runErr != nil {
			log.Errorf("%s exited: %v", serviceName, runErr)
		}
		releaseInfra()
		if runErr != nil {
			os.Exit(1)
		}
	}()

	runErr = run()
}

// run 承载全部启动逻辑，任一环节失败返回 error，由 main 的 defer 兜底统一收口
// （保证无论 panic 还是启动失败，都会走到 releaseInfra 释放资源并统一退出码）。
func run() error {
	// ① 加载配置（APP_ENV 解析 + 默认值兜底）
	cfg, err := goconfig.LoadByEnv()
	if err != nil {
		return fmt.Errorf("failed to load config: %v", err)
	}
	serviceName = cfg.App.Name

	// ② 初始化日志
	log.Init(cfg.App.LogDir, cfg.App.LogLevel, serviceName)

	// ③ 初始化基础设施
	if err := initInfra(cfg); err != nil {
		return fmt.Errorf("failed to init infra: %v", err)
	}

	// ④ 启用 Consul 服务发现（供调用下游时解析实例）
	consul.UseConsulDiscovery(cfg.Consul.Address)

	// ⑤ 注册本服务到 Consul（Gateway 由此自动同步 /api/v1/point 路由）
	deregister, err = registerToConsul(cfg)
	if err != nil {
		return fmt.Errorf("failed to register to consul: %v", err)
	}

	// ⑥ 向 ranking-service 注册「用户积分榜」（best-effort，失败仅告警，不影响启动）
	if err := client.RegisterUserPointsBoard(context.Background()); err != nil {
		log.Warnf("register user points board failed: %v", err)
	}

	srv := NewServer(cfg)

	// ⑥' 全量重建用户积分榜：积分只在「变动时」同步到 ranking-service，若 Redis
	//     重启/数据丢失，榜单会为空且不自愈。启动时按 DB 余额覆盖重建（best-effort）。
	if n, err := srv.pointSvc.RebuildUserPointsBoard(context.Background()); err != nil {
		log.Warnf("rebuild user points board failed: %v", err)
	} else {
		log.Infof("user points board rebuilt from DB: %d users", n)
	}

	if err := srv.Run(); err != nil {
		return fmt.Errorf("server exit: %v", err)
	}

	// ⑦ 正常退出：先从 Consul 摘流量，再释放全局资源
	shutdown()
	return nil
}
