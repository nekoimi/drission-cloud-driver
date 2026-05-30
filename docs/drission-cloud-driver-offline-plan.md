# drission-cloud-driver 离线下载能力落地计划

## 目标

为 `drission-cloud-driver` 补齐稳定的网盘离线下载 API，使 `get-magnet` 可以把磁力链接提交到中间服务，由中间服务完成：

- 选择目标网盘 Driver。
- 使用指定浏览器 Profile 的登录态。
- 创建或确认保存目录。
- 提交离线下载任务。
- 查询任务状态。
- 将各网盘差异状态统一成固定状态模型。
- 返回最终网盘文件信息。

`get-magnet` 不直接理解 115、PikPak、夸克、迅雷等平台差异，只依赖统一 HTTP 契约。

## 对接原则

- `get-magnet` 只通过 HTTP 调用中间服务。
- 所有 Driver API 继续使用 `X-Profile-ID` Header 指定浏览器 Profile。
- 平台差异由 `drission-cloud-driver` 吸收。
- 接口响应保持统一格式：

```json
{
  "code": 0,
  "message": "success",
  "data": {},
  "error": null
}
```

- `code = 0` 表示成功。
- 非 0 表示业务失败，`message` 给出可读错误，`error` 放调试信息。
- HTTP 状态码只表达协议层结果，业务错误放在响应体中。

## 必须落地的接口

### 健康检查

```http
GET /health
```

用途：

- `get-magnet` 启动时检查中间服务是否可用。
- 中间服务部署后快速确认运行状态。

成功响应：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "status": "ok"
  },
  "error": null
}
```

### Driver 能力查询

```http
GET /drivers/:platform/capabilities
Header: X-Profile-ID: <profile_id>
```

用途：

- 告诉调用方当前平台是否支持离线下载、文件搜索、媒体链接获取等能力。
- 便于后续多平台扩展。

建议响应：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "platform": "115",
    "offline_download": true,
    "fs_list": true,
    "fs_search": true,
    "media_url": true
  },
  "error": null
}
```

### 提交离线下载任务

```http
POST /drivers/:platform/offline/add
Header: X-Profile-ID: <profile_id>
Content-Type: application/json
```

请求体：

```json
{
  "url": "magnet:?xt=urn:btih:xxx",
  "category": "JavDB",
  "save_path": "/get-magnet/JavDB/2026-05-30",
  "client_task_id": "optional-idempotency-key",
  "metadata": {
    "origin": "JavDB"
  }
}
```

字段说明：

- `url`: 待离线下载链接，第一阶段主要是磁力链接。
- `category`: 上游分类，例如 `JavDB`。
- `save_path`: 网盘目标保存目录。
- `client_task_id`: 调用方提供的幂等 ID，可为空。
- `metadata`: 透传元数据，中间服务可以记录，但不应强依赖。

成功响应：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "task_id": "cloud-task-id",
    "provider_task_id": "115-task-id",
    "status": "pending"
  },
  "error": null
}
```

字段说明：

- `task_id`: 中间服务统一任务 ID，`get-magnet` 会保存到 `magnets.followed_by`。
- `provider_task_id`: 网盘平台原始任务 ID。
- `status`: 统一状态，见“任务状态模型”。

### 查询离线任务状态

```http
GET /drivers/:platform/offline/tasks/:id
Header: X-Profile-ID: <profile_id>
```

成功响应：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "task_id": "cloud-task-id",
    "provider_task_id": "115-task-id",
    "status": "completed",
    "name": "example",
    "progress": 100,
    "save_path": "/get-magnet/JavDB/2026-05-30",
    "error_code": "",
    "error_message": "",
    "files": [
      {
        "file_id": "file-id",
        "name": "example.mp4",
        "path": "/get-magnet/JavDB/2026-05-30/example.mp4",
        "size": 123456789
      }
    ]
  },
  "error": null
}
```

要求：

- 即使平台返回的是复杂状态，也必须转成统一状态。
- `completed` 时建议返回 `files`。
- 如果平台无法直接返回文件列表，中间服务可以用 `save_path` 和任务名搜索补齐。

### 查询离线任务列表

```http
GET /drivers/:platform/offline/tasks
Header: X-Profile-ID: <profile_id>
```

建议查询参数：

```text
status=pending,running,completed,failed
limit=100
offset=0
```

成功响应：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "items": [
      {
        "task_id": "cloud-task-id",
        "provider_task_id": "115-task-id",
        "status": "running",
        "name": "example",
        "progress": 42,
        "save_path": "/get-magnet/JavDB/2026-05-30",
        "error_code": "",
        "error_message": "",
        "files": []
      }
    ],
    "total": 1
  },
  "error": null
}
```

用途：

- 运维排查。
- 后续支持 `get-magnet` 批量恢复任务。

### 删除离线任务

```http
DELETE /drivers/:platform/offline/tasks/:id
Header: X-Profile-ID: <profile_id>
```

用途：

- 清理失败任务。
- 人工取消任务。

成功响应：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "task_id": "cloud-task-id",
    "deleted": true
  },
  "error": null
}
```

## 建议保留的文件接口

这些接口当前 README 中已有，建议保持并保证字段稳定。

### 创建目录

```http
POST /drivers/:platform/fs/mkdir
Header: X-Profile-ID: <profile_id>
Content-Type: application/json
```

请求：

```json
{
  "path": "/get-magnet/JavDB/2026-05-30"
}
```

### 列出目录

```http
GET /drivers/:platform/fs/list?path=/get-magnet/JavDB
Header: X-Profile-ID: <profile_id>
```

### 搜索文件

```http
GET /drivers/:platform/fs/search?keyword=example
Header: X-Profile-ID: <profile_id>
```

### 获取媒体链接

```http
GET /drivers/:platform/media/url?file_id=file-id
Header: X-Profile-ID: <profile_id>
```

`get-magnet` 第一阶段不强依赖这些接口，但离线任务完成后的文件定位会用到文件搜索能力。

## 任务状态模型

中间服务对外只暴露以下状态：

| 状态 | 含义 | get-magnet 行为 |
|---|---|---|
| `pending` | 任务已提交，等待平台处理 | 保持等待 |
| `running` | 平台正在离线下载 | 保持等待 |
| `completed` | 离线完成，文件已进入网盘 | 标记 `post_process_done = true` |
| `failed` | 平台确认失败 | 触发错误回调并记录日志 |
| `canceled` | 任务被取消 | 触发错误回调并记录日志 |
| `unknown` | 暂时无法确认状态 | 保持等待，下次轮询 |

状态转换建议：

```text
pending -> running -> completed
pending -> failed
running -> failed
pending -> canceled
running -> canceled
any -> unknown -> 原状态或新状态
```

`unknown` 不应该覆盖中间服务内部已知的最终状态，只表示本次查询无法确认。

## 内部模块建议

结合当前 README 中的结构，建议按以下方式落地。

### Driver 抽象层

位置：

```text
internal/drivers/
```

建议接口：

```go
type Driver interface {
    Platform() string
    Capabilities(ctx context.Context, profileID string) (Capabilities, error)

    AddOfflineTask(ctx context.Context, profileID string, req AddOfflineTaskRequest) (OfflineTask, error)
    GetOfflineTask(ctx context.Context, profileID string, taskID string) (OfflineTask, error)
    ListOfflineTasks(ctx context.Context, profileID string, query OfflineTaskQuery) (OfflineTaskList, error)
    DeleteOfflineTask(ctx context.Context, profileID string, taskID string) error

    Mkdir(ctx context.Context, profileID string, path string) error
    ListFiles(ctx context.Context, profileID string, path string) ([]FileEntry, error)
    SearchFiles(ctx context.Context, profileID string, keyword string) ([]FileEntry, error)
    MediaURL(ctx context.Context, profileID string, fileID string) (string, error)
}
```

如果现有 Driver 接口已经存在，不必完全替换，可以增量补齐离线下载方法。

### 统一类型

位置：

```text
internal/drivers/types.go
```

建议类型：

```go
type AddOfflineTaskRequest struct {
    URL          string            `json:"url"`
    Category     string            `json:"category,omitempty"`
    SavePath     string            `json:"save_path,omitempty"`
    ClientTaskID string            `json:"client_task_id,omitempty"`
    Metadata     map[string]string `json:"metadata,omitempty"`
}

type OfflineTask struct {
    TaskID         string      `json:"task_id"`
    ProviderTaskID string      `json:"provider_task_id,omitempty"`
    Status         TaskStatus  `json:"status"`
    Name           string      `json:"name,omitempty"`
    Progress       float64     `json:"progress,omitempty"`
    SavePath       string      `json:"save_path,omitempty"`
    ErrorCode      string      `json:"error_code,omitempty"`
    ErrorMessage   string      `json:"error_message,omitempty"`
    Files          []FileEntry `json:"files,omitempty"`
}

type TaskStatus string

const (
    TaskPending   TaskStatus = "pending"
    TaskRunning   TaskStatus = "running"
    TaskCompleted TaskStatus = "completed"
    TaskFailed    TaskStatus = "failed"
    TaskCanceled  TaskStatus = "canceled"
    TaskUnknown   TaskStatus = "unknown"
)
```

### Handler 层

位置：

```text
internal/handler/v1/driver.go
```

建议拆分：

```text
internal/handler/v1/offline.go
internal/handler/v1/fs.go
internal/handler/v1/media.go
```

如果暂时不想拆文件，也可以先保留在 `driver.go`，但要把请求解析、Driver 调用和响应包装分清。

### 任务 ID 映射

建议中间服务生成自己的 `task_id`，不要直接把平台原始 ID 暴露为唯一 ID。

原因：

- 不同平台任务 ID 规则不同。
- 有些平台可能没有稳定 ID。
- 后续可以支持幂等、任务恢复、状态缓存。

第一阶段可以使用：

```text
task_id = platform + ":" + provider_task_id
```

后续再演进成持久化任务表。

## 持久化建议

第一阶段如果 115 Driver 能稳定通过平台原始任务 ID 查询状态，可以不加数据库。

但建议尽早预留任务记录能力：

```text
offline_tasks
```

字段建议：

| 字段 | 说明 |
|---|---|
| `id` | 中间服务统一任务 ID |
| `platform` | 平台，例如 `115` |
| `profile_id` | 浏览器 Profile |
| `provider_task_id` | 平台任务 ID |
| `url` | 原始链接 |
| `save_path` | 保存目录 |
| `status` | 统一状态 |
| `name` | 任务名称 |
| `progress` | 进度 |
| `error_code` | 错误码 |
| `error_message` | 错误信息 |
| `files_json` | 完成后的文件列表 |
| `client_task_id` | 幂等键 |
| `created_at` | 创建时间 |
| `updated_at` | 更新时间 |

第一阶段可以先用内存加平台查询兜底，但服务重启后可能丢失映射。若 `get-magnet` 要长期稳定运行，建议落库。

## 115 Driver 落地要求

115 网盘作为第一阶段目标，需要完成：

1. 根据 `X-Profile-ID` 获取或启动浏览器连接。
2. 确认用户已登录 115。
3. 确认或创建 `save_path`。
4. 调用 115 离线下载能力提交磁力链接。
5. 获取平台任务 ID。
6. 将平台状态映射为统一状态。
7. 完成后定位文件列表。

### 目录处理

`offline/add` 收到 `save_path` 后，中间服务应尽量保证目录存在。

建议行为：

- 目录存在：直接使用。
- 目录不存在：自动创建。
- 创建失败：返回业务错误，不提交离线任务。

### 完成文件定位

115 离线任务完成后，如果平台任务接口直接返回文件信息，优先使用平台返回。

如果没有直接返回，使用以下方式补齐：

1. 根据任务名在 `save_path` 下搜索。
2. 根据磁力 hash 或文件名搜索。
3. 返回匹配到的文件列表。

如果任务已完成但暂时定位不到文件：

- 状态可以先返回 `completed`。
- `files` 可为空。
- `error_message` 不应标记为失败。

## 错误码建议

建议保留统一业务错误码：

| code | 含义 |
|---|---|
| `0` | 成功 |
| `40001` | 请求参数错误 |
| `40101` | Profile 未登录或登录态失效 |
| `40401` | Driver 不存在 |
| `40402` | 任务不存在 |
| `40901` | 幂等任务已存在 |
| `50001` | Driver 执行失败 |
| `50002` | 浏览器连接失败 |
| `50003` | 平台状态无法解析 |

错误响应示例：

```json
{
  "code": 40101,
  "message": "profile is not logged in",
  "data": null,
  "error": {
    "profile_id": "xxx",
    "platform": "115"
  }
}
```

## 幂等策略

`client_task_id` 第一阶段可以不强制，但建议支持。

规则：

- 同一 `platform + profile_id + client_task_id` 重复提交时，不重复创建平台任务。
- 如果历史任务仍在进行，直接返回原 `task_id` 和当前状态。
- 如果历史任务已完成，返回原 `task_id` 和 `completed`。
- 如果历史任务失败，是否重试由后续 `retry` 接口决定。

如果 `client_task_id` 为空，中间服务正常创建新任务。

## 路由落地清单

需要确认这些路由已注册：

```text
GET    /health
GET    /drivers
GET    /drivers/:platform/capabilities

POST   /drivers/:platform/offline/add
GET    /drivers/:platform/offline/tasks
GET    /drivers/:platform/offline/tasks/:id
DELETE /drivers/:platform/offline/tasks/:id

POST   /drivers/:platform/fs/mkdir
GET    /drivers/:platform/fs/list
GET    /drivers/:platform/fs/search
DELETE /drivers/:platform/fs/remove
POST   /drivers/:platform/fs/move
POST   /drivers/:platform/fs/rename

GET    /drivers/:platform/media/url
```

## 测试计划

### 单元测试

覆盖：

- 平台状态到统一状态的映射。
- `task_id` 生成和解析。
- `client_task_id` 幂等逻辑。
- 请求参数校验。
- 统一响应结构。

### 集成测试

使用测试 Profile 验证：

1. Profile 可启动。
2. 115 已登录。
3. `offline/add` 可以提交磁力链接。
4. `offline/tasks/:id` 可以查询状态。
5. 任务完成后能返回文件列表或至少返回 `completed`。

### get-magnet 对接测试

启动顺序：

1. 启动 CloakBrowser-Manager。
2. 启动并登录 115 Profile。
3. 启动 `drission-cloud-driver`。
4. 设置 `get-magnet` 环境变量：

```text
CLOUD_DRIVER_BASE_URL=http://localhost:8080
CLOUD_DRIVER_PLATFORM=115
CLOUD_DRIVER_PROFILE_ID=<profile-id>
CLOUD_DRIVER_SAVE_ROOT=/get-magnet
```

5. 启动 `get-magnet`。
6. 触发一次爬虫采集。
7. 确认 `magnets.followed_by` 写入中间服务 `task_id`。
8. 离线任务完成后确认 `post_process_done = true`。

## 实施阶段

### 阶段 1：接口固定

目标：

- 固定请求体和响应体。
- 固定状态枚举。
- 确认路由注册。

验收：

- `curl /health` 成功。
- `curl /drivers/115/capabilities` 成功。

### 阶段 2：115 离线提交

目标：

- `POST /drivers/115/offline/add` 可提交磁力链接。
- 返回统一 `task_id`。

验收：

- 115 网盘后台出现离线任务。
- API 返回 `pending` 或 `running`。

### 阶段 3：状态查询

目标：

- `GET /drivers/115/offline/tasks/:id` 可查询统一状态。
- 支持 `pending/running/completed/failed/canceled/unknown`。

验收：

- 任务进行中返回 `running`。
- 任务完成后返回 `completed`。

### 阶段 4：文件定位

目标：

- 完成任务返回 `files`。
- 至少包含 `file_id`、`name`、`path`、`size`。

验收：

- `get-magnet` 能在完成轮询中拿到文件路径列表。

### 阶段 5：幂等和持久化

目标：

- 支持 `client_task_id`。
- 服务重启后仍能查询历史任务。

验收：

- 同一幂等键重复提交不会创建重复离线任务。
- 重启中间服务后，旧 `task_id` 仍可查询。

## 与 get-magnet 当前实现的契约

当前 `get-magnet` 云盘下载器依赖以下行为：

- `POST /drivers/:platform/offline/add` 成功时必须返回 `data.task_id`。
- 响应格式支持 `message`，不要求 `msg`。
- `GET /drivers/:platform/offline/tasks/:id` 成功时必须返回 `data.status`。
- 状态为 `completed` 时，`get-magnet` 会标记本地任务完成。
- 状态为 `failed` 或 `canceled` 时，`get-magnet` 会触发错误回调。
- 状态为 `pending`、`running`、`unknown` 时，`get-magnet` 会等待下一次轮询。

因此中间服务首版最小可用标准是：

```text
health 正常
offline/add 返回 task_id
offline/tasks/:id 返回统一 status
```

