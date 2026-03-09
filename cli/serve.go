package cli

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sath/agent"
	"github.com/sath/config"
	"github.com/sath/errs"
	"github.com/sath/memory"
	"github.com/sath/middleware"
	"github.com/sath/model"
	"github.com/sath/obs"
	"github.com/sath/templates"
	"github.com/spf13/cobra"
)

// NewServeCommand 返回 sath serve 命令，启动 HTTP 服务形式的 Agent。
func NewServeCommand() *cobra.Command {
	var addr, configPath string
	var debug bool
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "启动 HTTP 服务形式的 Agent",
		Long:  "监听指定地址，提供 POST /chat、GET /health、GET /metrics。需设置 OPENAI_API_KEY。",
		RunE: func(cmd *cobra.Command, args []string) error {
			var cfg config.Config
			if configPath != "" {
				var err error
				cfg, err = config.LoadWithEnv(configPath)
				if err != nil {
					return fmt.Errorf("load config: %w", err)
				}
			} else {
				cfg = config.FromEnv()
			}

			var handler middleware.Handler
			var dataHandler middleware.Handler
			if cfg.ModelName != "" && len(cfg.Middlewares) >= 0 {
				mwMap := templates.DefaultMiddlewareMap()
				var err error
				handler, err = templates.NewChatAgentHandlerFromConfig(cfg, mwMap)
				if err != nil {
					return fmt.Errorf("build handler from config: %w", err)
				}
				// 若配置了数据源，则尝试构建数据查询 Handler。
				if len(cfg.DataSources) > 0 {
					dataHandler, err = templates.NewDataQueryHandlerFromConfig(cfg, mwMap)
					if err != nil {
						return fmt.Errorf("build data query handler from config: %w", err)
					}
				}
			} else {
				m, err := model.NewOpenAIClient()
				if err != nil {
					return fmt.Errorf("init model: %w", err)
				}
				mem := memory.NewBufferMemory(cfg.MaxHistory)
				mws := []middleware.Middleware{
					middleware.RecoveryMiddleware,
					middleware.LoggingMiddleware,
					middleware.MetricsMiddleware,
				}
				handler = templates.NewChatAgentHandler(m, mem, mws...)
			}
			if debug {
				handler = middleware.DebugMiddleware(true)(handler)
			}

			var healthModel model.Model
			if cfg.ModelName != "" {
				healthModel, _ = model.NewFromIdentifier(cfg.ModelName)
			}
			if healthModel == nil {
				healthModel, _ = model.NewOpenAIClient()
			}
			healthChecks := map[string]obs.HealthCheckFunc{}
			if healthModel != nil {
				healthChecks["model"] = func(ctx context.Context) error {
					ctx2, cancel := context.WithTimeout(ctx, 2*time.Second)
					defer cancel()
					_, err := healthModel.Generate(ctx2, "ping")
					return err
				}
			}
			http.Handle("/health", obs.HealthHandler(healthChecks))

			http.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
					return
				}
				requestID := r.Header.Get("X-Request-ID")
				if requestID == "" {
					b := make([]byte, 8)
					if _, err := rand.Read(b); err == nil {
						requestID = hex.EncodeToString(b)
					}
				}
				var body struct {
					Message string `json:"message"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					http.Error(w, errs.ErrBadRequest.Error(), http.StatusBadRequest)
					return
				}
				req := &agent.Request{
					Messages:  []model.Message{{Role: "user", Content: body.Message}},
					RequestID: requestID,
				}
				if debug {
					w.Header().Set("X-Request-ID", requestID)
				}
				resp, err := handler(context.Background(), req)
				if err != nil {
					code, msg := http.StatusInternalServerError, errs.ErrInternal.Error()
					if errors.Is(err, errs.ErrBadRequest) {
						code, msg = http.StatusBadRequest, err.Error()
					} else if errors.Is(err, errs.ErrRateLimited) {
						code, msg = http.StatusTooManyRequests, err.Error()
					} else if errors.Is(err, errs.ErrContentBlocked) {
						code, msg = http.StatusUnprocessableEntity, err.Error()
					} else {
						msg = err.Error()
					}
					http.Error(w, msg, code)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				out := map[string]string{"reply": resp.Text}
				if debug {
					out["request_id"] = requestID
				}
				_ = json.NewEncoder(w).Encode(out)
			})
			http.Handle("/metrics", obs.MetricsHandler())

			// 数据查询 API：POST /data/chat
			http.HandleFunc("/data/chat", func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
					return
				}
				if dataHandler == nil {
					http.Error(w, "data query handler not configured", http.StatusServiceUnavailable)
					return
				}
				requestID := r.Header.Get("X-Request-ID")
				if requestID == "" {
					b := make([]byte, 8)
					if _, err := rand.Read(b); err == nil {
						requestID = hex.EncodeToString(b)
					}
				}
				var body struct {
					Message      string `json:"message"`
					SessionID    string `json:"session_id"`
					UserID       string `json:"user_id"`
					DatasourceID string `json:"datasource_id"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					http.Error(w, errs.ErrBadRequest.Error(), http.StatusBadRequest)
					return
				}
				req := &agent.Request{
					Messages:  []model.Message{{Role: "user", Content: body.Message}},
					RequestID: requestID,
					Metadata: map[string]any{
						"session_id":    body.SessionID,
						"user_id":       body.UserID,
						"datasource_id": body.DatasourceID,
					},
				}
				if debug {
					w.Header().Set("X-Request-ID", requestID)
				}
				resp, err := dataHandler(context.Background(), req)
				if err != nil {
					code, msg := http.StatusInternalServerError, errs.ErrInternal.Error()
					if errors.Is(err, errs.ErrBadRequest) {
						code, msg = http.StatusBadRequest, err.Error()
					} else if errors.Is(err, errs.ErrRateLimited) {
						code, msg = http.StatusTooManyRequests, err.Error()
					} else if errors.Is(err, errs.ErrContentBlocked) {
						code, msg = http.StatusUnprocessableEntity, err.Error()
					} else {
						msg = err.Error()
					}
					http.Error(w, msg, code)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				out := map[string]any{"reply": resp.Text}
				if debug {
					out["request_id"] = requestID
				}
				if resp.Metadata != nil {
					if v, ok := resp.Metadata["confirm_required"]; ok {
						out["confirm_required"] = v
					}
					if v, ok := resp.Metadata["confirm_request"]; ok {
						out["confirm_request"] = v
					}
				}
				_ = json.NewEncoder(w).Encode(out)
			})

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			srv := &http.Server{Addr: addr}
			go func() {
				<-ctx.Done()
				_ = srv.Shutdown(context.Background())
			}()

			cmd.Printf("Listening on %s (POST /chat, POST /data/chat, GET /health, GET /metrics)\n", addr)
			if debug {
				cmd.Println("Debug mode: verbose logs and request_id in response")
			}
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				return err
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&addr, "addr", "a", ":8080", "监听地址")
	cmd.Flags().StringVarP(&configPath, "config", "c", "", "配置文件路径（可选）")
	cmd.Flags().BoolVarP(&debug, "debug", "d", false, "开启调试模式：详细日志与响应中的 request_id")
	return cmd
}
