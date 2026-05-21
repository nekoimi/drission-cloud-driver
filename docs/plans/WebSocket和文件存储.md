对于 Gin 项目，**推荐使用 `github.com/gorilla/websocket`**（2026 年依然是最成熟、社区支持最好、与 Gin 结合最自然的库）。它稳定、功能完整，且 Gin 官方示例也使用它。

### 更新后的项目目录结构（新增 WebSocket 部分）

```
goscaffold/
├── cmd/
│   └── server/
│       └── main.go                  # 主入口（同时支持 HTTP + WebSocket）
├── internal/
│   ├── config/                      # 配置（新增 websocket 配置）
│   ├── handler/
│   │   ├── v1/
│   │   │   ├── auth.go
│   │   │   ├── user.go
│   │   │   └── ws.go                # 新增：WebSocket 路由与升级处理
│   │   └── middleware/
│   ├── model/                       # 可复用现有模型
│   ├── repository/
│   ├── service/
│   │   └── ws_service.go            # 新增：WebSocket 业务逻辑（消息处理、广播等）
│   ├── websocket/                   # 新增：WebSocket 核心模块（推荐独立目录）
│   │   ├── manager.go               # 连接管理器（Hub）：维护客户端列表、广播消息
│   │   ├── client.go                # 单个客户端连接封装（读写协程）
│   │   ├── handler.go               # WebSocket 升级 + 消息分发
│   │   └── message.go               # 消息结构体（通用消息格式）
│   ├── infrastructure/
│   │   ├── database/
│   │   └── logger/
│   └── pkg/
│       └── response/                # HTTP 响应（WebSocket 可复用部分逻辑）
├── configs/
│   └── config.dev.yaml              # 新增 websocket 配置段
├── deployments/
│   ├── docker-compose.yml           # 可选暴露 ws 端口
│   └── Dockerfile
├── .env.example
├── go.mod
├── Makefile
└── README.md
```

### 新增依赖

执行以下命令添加：

```bash
go get github.com/gorilla/websocket
```

### 配置新增部分（configs/config.dev.yaml）

```yaml
server:
  port: 8080
  mode: debug

# ... 原有 database、jwt 配置

websocket:
  enabled: true
  read_buffer_size: 1024
  write_buffer_size: 1024
  ping_period: 60s          # 心跳间隔
  write_wait: 10s           # 写超时
  read_wait: 60s            # 读超时
  max_message_size: 5120    # 最大消息大小（字节）
```

### WebSocket 设计要点（小项目友好）

- **连接管理器（Hub）**：单例或注入，维护所有活跃客户端，支持广播（Broadcast）、群发、单发。
- **Client**：每个连接对应一个 Client，包含读/写 goroutine + channel。
- **消息格式**：统一使用 JSON，例如：
  ```go
  type Message struct {
      Type    string      `json:"type"`    // "chat"、"notify"、"system" 等
      From    string      `json:"from,omitempty"`
      To      string      `json:"to,omitempty"`
      Content interface{} `json:"content"`
      Time    time.Time   `json:"time"`
  }
  ```
- **鉴权**：支持在升级时通过 Query 参数或 Header 携带 JWT，在 middleware 中验证后注入用户 ID。
- **心跳**：定期 Ping/Pong，自动清理断开连接。
- **并发安全**：使用 mutex 保护客户端 map。

### 使用示例

- 客户端连接地址：`ws://localhost:8080/ws/v1/chat?token=your-jwt-token`
- 支持房间（Room）或全局广播（根据需求扩展）。
- 在 `internal/websocket/manager.go` 中实现广播：`hub.Broadcast(msg)`

### 集成方式

在 `cmd/server/main.go` 中：
- 加载配置后初始化 `websocket.NewManager(logger, config.Websocket)`
- 在 Gin 路由中注册：`router.GET("/ws/v1/chat", wsHandler.UpgradeHandler)`

**推荐路由结构**：
- `/ws/v1/chat` — 通用聊天/通知
- `/ws/v1/notifications` — 用户专属通知
- 支持 URL 参数区分房间：`/ws/v1/room/:roomID`


# 文件存储支持

要求完全满足：
- 支持**本地存储**（开发/小项目最方便）
- 支持**MinIO**（S3 兼容，生产推荐）
- **高度可扩展**：以后加 AWS S3、阿里云 OSS、腾讯 COS 等，只需新增一个实现类即可，无需改动业务代码

### 1. 更新后的项目目录结构（新增 storage 模块）

```
goscaffold/
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── config/                      # 配置新增 storage 段
│   ├── handler/
│   │   ├── v1/
│   │   │   ├── upload.go            # 新增：统一文件上传 Handler
│   │   │   ├── auth.go
│   │   │   └── user.go
│   │   └── middleware/
│   ├── storage/                     # 新增：文件存储核心（策略模式）
│   │   ├── storage.go               # 接口定义 FileStorage
│   │   ├── factory.go               # 工厂 + 根据配置自动选择驱动
│   │   ├── local/                   # 本地文件系统实现
│   │   │   └── local.go
│   │   ├── minio/                   # MinIO 实现
│   │   │   └── minio.go
│   │   └── types.go                 # 公共类型（UploadResult、FileHeader 等）
│   ├── service/                     # 可新增 file_service.go（业务层包装）
│   ├── websocket/                   # （已有）
│   ├── scheduler/                   # （已有）
│   └── infrastructure/
│       ├── database/
│       └── logger/
├── configs/
│   └── config.dev.yaml              # 新增 storage 配置
├── migrations/                      # 可选：新增 files 表记录文件元数据
├── deployments/
│   ├── docker-compose.yml           # MinIO 服务（开发用）
│   └── Dockerfile
├── .env.example
├── go.mod
├── Makefile
└── README.md
```

### 2. 新增依赖（执行一次）

```bash
go get github.com/minio/minio-go/v7
go get github.com/minio/minio-go/v7/pkg/credentials
```

（`gorilla/websocket` 和其他已有依赖保持不变）

### 3. 配置新增部分（configs/config.dev.yaml）

```yaml
server:
  port: 8080
  mode: debug

# ... 原有 database、jwt、websocket、scheduler 配置

storage:
  driver: "local"          # 可选值：local | minio
  base_url: "http://localhost:8080/uploads"   # 公开访问前缀（本地用）
  
  # 本地驱动专用
  local:
    upload_dir: "./uploads"   # 项目根目录下的 uploads 文件夹（.gitignore 已忽略）
    max_file_size: 10         # MB
  
  # MinIO 驱动专用
  minio:
    endpoint: "localhost:9000"
    access_key: "minioadmin"
    secret_key: "minioadmin"
    bucket: "goscaffold"
    use_ssl: false
    public_url: "http://localhost:9000"   # 用于生成公开访问 URL
```

### 4. 核心设计（策略模式 + 工厂）

- `internal/storage/storage.go` 定义统一接口：
  ```go
  type FileStorage interface {
      Upload(ctx context.Context, file *types.FileHeader, folder string) (*types.UploadResult, error)
      Delete(ctx context.Context, path string) error
      GetURL(path string) string
      Exists(ctx context.Context, path string) (bool, error)
  }
  ```

- `factory.go` 根据 `config.Storage.Driver` 自动返回对应实现（`NewStorage`）。
- 业务代码永远只依赖 `storage.FileStorage` 接口，完全解耦。
- 统一返回 `UploadResult`（包含 `Path`、`URL`、`Size`、`MimeType` 等）。

### 5. 使用示例（未来你写业务时这样用）

```go
// 在 Handler 中
uploadResult, err := h.storage.Upload(c.Request.Context(), fileHeader, "avatars")

// 在 Service 中（推荐）
func (s *UserService) UpdateAvatar(ctx context.Context, userID uint, file *types.FileHeader) error {
    result, err := s.storage.Upload(ctx, file, "avatars")
    // 保存 result.Path 到数据库
}
```

### 6. 推荐路由（已规划在 handler/v1/upload.go）

- `POST /v1/upload/single` → 单文件上传（支持 folder 参数）
- `POST /v1/upload/multiple` → 多文件上传
- 可选：图片压缩、类型校验（jpg/png/pdf 等）、防重名（UUID + 原始后缀）

### 7. Docker 开发环境（docker-compose.yml 已支持）

新增 MinIO 服务（开发时一键启动）：
```yaml
services:
  minio:
    image: minio/minio:latest
    command: server /data --console-address ":9001"
    ports:
      - "9000:9000"
      - "9001:9001"
    environment:
      MINIO_ROOT_USER: minioadmin
      MINIO_ROOT_PASSWORD: minioadmin
    volumes:
      - minio-data:/data
```



