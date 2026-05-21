package websocket

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	wslib "github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/nekoimi/go-project-template/internal/config"
	"github.com/nekoimi/go-project-template/internal/pkg/jwtutil"
)

type WSHandler struct {
	manager        *Manager
	jwtSecret      string
	logger         *zap.Logger
	allowedOrigins []string
	params         connParams
}

func NewWSHandler(manager *Manager, jwtSecret string, logger *zap.Logger, allowedOrigins []string, wsCfg config.WebsocketConfig) *WSHandler {
	return &WSHandler{
		manager:        manager,
		jwtSecret:      jwtSecret,
		logger:         logger,
		allowedOrigins: allowedOrigins,
		params:         newConnParams(wsCfg),
	}
}

// Upgrade handles WebSocket upgrade requests.
// 优先从 Sec-WebSocket-Protocol 解析 JWT（推荐：new WebSocket(url, ['access_token', token])），否则使用 ?token= 查询参数。
// GET /ws/v1/chat?token=<jwt>
func (h *WSHandler) Upgrade(c *gin.Context) {
	tokenStr := tokenFromWSRequest(c.Request)
	if tokenStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
		return
	}

	userID, err := jwtutil.ValidateToken(tokenStr, h.jwtSecret)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}

	upgrader := wslib.Upgrader{
		ReadBufferSize:  h.params.readBufferSize,
		WriteBufferSize: h.params.writeBufferSize,
		CheckOrigin: func(r *http.Request) bool {
			return h.checkOrigin(r.Header.Get("Origin"))
		},
	}

	respHdr := http.Header{}
	if p := wsSubprotocolToEcho(c.Request); p != "" {
		respHdr.Set("Sec-WebSocket-Protocol", p)
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, respHdr)
	if err != nil {
		h.logger.Error("websocket upgrade failed", zap.Error(err))
		return
	}

	client := newClient(h.manager, conn, userID, h.logger, h.params)
	h.manager.register <- client

	go client.WritePump()
	go client.ReadPump()
}

func jwtLooksLike(s string) bool {
	return strings.Count(s, ".") == 2
}

func tokenFromWSRequest(r *http.Request) string {
	if hdr := r.Header.Get("Sec-WebSocket-Protocol"); hdr != "" {
		for _, part := range strings.Split(hdr, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if strings.EqualFold(part, "bearer") || strings.EqualFold(part, "access_token") {
				continue
			}
			if jwtLooksLike(part) {
				return part
			}
		}
	}
	return r.URL.Query().Get("token")
}

// 若客户端通过子协议列表协商 token，需在响应中回显其中一个协议名（如 access_token）。
func wsSubprotocolToEcho(r *http.Request) string {
	hdr := r.Header.Get("Sec-WebSocket-Protocol")
	if hdr == "" {
		return ""
	}
	for _, part := range strings.Split(hdr, ",") {
		part = strings.TrimSpace(part)
		if part == "" || jwtLooksLike(part) {
			continue
		}
		return part
	}
	return ""
}

func (h *WSHandler) checkOrigin(origin string) bool {
	// 如果未配置允许的来源，则允许所有（开发模式）
	if len(h.allowedOrigins) == 0 {
		return true
	}
	// 检查来源是否在允许列表中
	for _, allowed := range h.allowedOrigins {
		if origin == allowed {
			return true
		}
	}
	return false
}
