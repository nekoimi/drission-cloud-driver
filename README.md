# drission-cloud-driver

Browser-based Cloud Storage Driver Middleware — 基于浏览器自动化的云盘驱动中间件。

## 项目简介

drission-cloud-driver 通过 CDP（Chrome DevTools Protocol）连接真实浏览器，直接操作云盘网页完成各种云存储操作。核心理念：**Browser is the API**。

### 支持的云盘平台

| 平台 | 状态 | 说明 |
|---|---|---|
| 115 网盘 | ✅ 已实现 | 离线下载、文件管理、搜索 |
| PikPak | 🔜 计划中 | - |
| 夸克网盘 | 🔜 计划中 | - |
| 迅雷云盘 | 🔜 计划中 | - |

## 技术栈

| 组件 | 说明 |
|---|---|
| [Gin](https://github.com/gin-gonic/gin) | HTTP 框架 |
| [playwright-go](https://github.com/playwright-community/playwright-go) | CDP 浏览器自动化 |
| [115driver](https://github.com/SheltonZhu/115driver) | 115 网盘 SDK |
| [Zap](https://github.com/uber-go/zap) | 结构化日志 |
| [Viper](https://github.com/spf13/viper) | 配置管理 |
| [CloakBrowser-Manager](https://github.com/CloakHQ/CloakBrowser-Manager) | 浏览器实例管理 |

## 项目结构

```
├── cmd/
│   └── server/main.go              # HTTP 服务入口
├── configs/                        # 配置文件
├── deployments/                    # Dockerfile & docker-compose
├── internal/
│   ├── app/                        # 应用初始化
│   ├── browser/                    # CDP 浏览器连接层
│   │   ├── connection.go           # 单个 CDP 连接封装
│   │   ├── manager.go              # 浏览器实例管理
│   │   └── types.go                # 类型定义
│   ├── cloak/                      # CloakBrowser-Manager 客户端
│   │   ├── client.go               # HTTP 客户端
│   │   └── types.go                # API 响应结构
│   ├── config/                     # 配置结构与加载
│   ├── drivers/                    # Driver 抽象层
│   │   ├── driver.go               # Driver 接口定义
│   │   ├── types.go                # 统一数据类型
│   │   ├── registry.go             # Driver 注册表
│   │   ├── base/                   # 基础 Driver 实现
│   │   └── pan115/                 # 115 网盘 Driver
│   ├── handler/
│   │   ├── middleware/             # 中间件 (CORS, 限流, 日志, Recovery, RequestID)
│   │   └── v1/                     # API Handler
│   │       ├── driver.go           # Driver API
│   │       ├── profile.go          # 浏览器 Profile 管理
│   │       └── system.go           # 系统信息
│   ├── infrastructure/
│   │   └── logger/                 # 日志
│   ├── offline/                    # 离线任务仓库
│   └── pkg/
│       ├── errcode/                # 业务错误码
│       ├── response/               # 统一响应格式
│       └── timeutil/               # 时区与时间类型
├── Makefile
└── go.mod
```

## 快速开始

### 1. 部署 CloakBrowser-Manager

CloakBrowser-Manager 负责管理浏览器实例，请参考 [官方文档](https://github.com/CloakHQ/CloakBrowser-Manager) 进行部署。

### 2. 配置

编辑 `configs/config.dev.yaml`：

```yaml
server:
  port: "8091"
  mode: debug
  timezone: "Asia/Shanghai"

cloak:
  base_url: "http://localhost:3000"  # CloakBrowser-Manager 地址

drivers:
  default_timeout: 30
  platforms:
    - "115"  # 启用 115 网盘

offline:
  store:
    driver: "postgres"
    dsn: "postgres://drission:drission@localhost:5432/drission_cloud_driver?sslmode=disable"
```

### 3. 启动服务

```bash
make run
```

### 4. 使用流程

1. 在 CloakBrowser-Manager 中创建浏览器 Profile
2. 启动 Profile，在浏览器中登录 115 账号
3. 调用 API 时通过 `X-Profile-ID` Header 指定 Profile

## API

### 系统接口

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/health` | 健康检查 |
| GET | `/drivers` | 列出所有 Driver |

### 浏览器 Profile 管理

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/profiles` | 列出所有 Profile |
| GET | `/profiles/:id` | 获取 Profile 详情 |
| POST | `/profiles/:id/start` | 启动 Profile |
| POST | `/profiles/:id/stop` | 停止 Profile |

### Driver API

所有 Driver API 都需要通过 `X-Profile-ID` Header 指定浏览器 Profile。

#### 能力查询

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/drivers/:platform/capabilities` | 获取 Driver 能力 |

#### 离线下载

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/drivers/:platform/offline/add` | 提交离线下载任务 |
| GET | `/drivers/:platform/offline/tasks` | 列出所有任务 |
| GET | `/drivers/:platform/offline/tasks/:id` | 查询任务状态 |
| DELETE | `/drivers/:platform/offline/tasks/:id` | 删除任务 |

#### 文件系统

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/drivers/:platform/fs/mkdir` | 创建目录 |
| DELETE | `/drivers/:platform/fs/remove` | 删除文件/目录 |
| POST | `/drivers/:platform/fs/move` | 移动文件/目录 |
| POST | `/drivers/:platform/fs/rename` | 重命名 |
| GET | `/drivers/:platform/fs/list` | 列出目录内容 |
| GET | `/drivers/:platform/fs/search` | 搜索文件 |

#### 媒体

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/drivers/:platform/media/url` | 获取下载链接 |

### 请求示例

```bash
# 列出 115 网盘根目录
curl -H "X-Profile-ID: your-profile-id" \
  "http://localhost:8091/drivers/115/fs/list?path=/"

# 提交离线下载任务
curl -X POST \
  -H "X-Profile-ID: your-profile-id" \
  -H "Content-Type: application/json" \
  -d '{"url": "https://example.com/file.zip"}' \
  "http://localhost:8091/drivers/115/offline/add"

# 搜索文件
curl -H "X-Profile-ID: your-profile-id" \
  "http://localhost:8091/drivers/115/fs/search?keyword=movie"
```

### 统一响应格式

```json
{
  "code": 0,
  "message": "success",
  "data": {},
  "error": null
}
```

错误时 `code` 为业务错误码，`error` 包含错误详情。

## 配置

配置文件位于 `configs/`，通过 `--config` 参数指定。支持环境变量覆盖：

| 环境变量 | 对应配置 | 说明 |
|---|---|---|
| `CLOAK_BASE_URL` | cloak.base_url | CloakBrowser-Manager 地址 |
| `CLOAK_API_KEY` | cloak.api_key | CloakBrowser API Key (可选) |
| `OFFLINE_STORE_DRIVER` | offline.store.driver | 离线任务仓库类型，默认 postgres |
| `OFFLINE_STORE_DSN` / `DATABASE_URL` | offline.store.dsn | PostgreSQL 连接串 |
| `OFFLINE_SYNC_ENABLED` | offline.sync.enabled | 启用后台离线任务状态同步 |
| `OFFLINE_SYNC_INTERVAL_SECONDS` | offline.sync.interval_seconds | 同步间隔，默认 15 秒 |
| `OFFLINE_SYNC_CLEANUP_COMPLETED` | offline.sync.cleanup_completed | 完成后清理平台离线任务记录 |
| `OFFLINE_SYNC_CLEANUP_GRACE_SECONDS` | offline.sync.cleanup_grace_seconds | 完成到清理之间的宽限时间 |
| `TZ` | server.timezone | 时区 |

完整配置项见 `configs/config.dev.yaml`。

## 构建与部署

```bash
make build         # 编译到 bin/
make test          # 运行测试
make lint          # golangci-lint

make docker-build  # 构建 Docker 镜像
make docker-up     # 启动服务
make docker-down   # 停止服务
```

## License

[MIT](LICENSE)
