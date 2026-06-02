# drission-cloud-driver 接口文档

本文档按当前代码整理，适合手动逐个接口测试。

## 基础信息

- 默认服务地址：`http://localhost:8091`
- 默认已注册平台：`115`
- Content-Type：有请求体的接口使用 `application/json`
- Driver 类接口需要指定浏览器 Profile：
  - 推荐 Header：`X-Profile-ID: <profile_id>`
  - 也支持 Query：`?profile_id=<profile_id>`

使用 Driver 类接口前，建议先：

1. 在 CloakBrowser-Manager 中创建 Profile。
2. 启动 Profile。
3. 在浏览器中登录对应网盘账号，例如 115。
4. 后续请求带上 `X-Profile-ID`。

## 统一响应格式

成功响应：

```json
{
  "code": 0,
  "message": "success",
  "data": {}
}
```

失败响应：

```json
{
  "code": 40020,
  "message": "invalid request",
  "error": "keyword is required"
}
```

常见业务错误码：

| code | message | 说明 |
|---:|---|---|
| 0 | success | 成功 |
| 40000 | bad request | 请求错误 |
| 40010 | platform not supported | 平台不支持 |
| 40011 | browser profile not running | Profile 未运行 |
| 40020 | invalid request | 请求参数错误 |
| 40110 | profile is not logged in | Profile 未登录对应平台 |
| 40410 | driver not found | Driver 不存在 |
| 40411 | browser profile not found | Profile 不存在 |
| 40412 | offline task not found | 离线任务不存在 |
| 42200 | validation error | JSON 绑定或必填字段校验失败 |
| 42900 | too many requests | 触发限流 |
| 50012 | driver operation failed | Driver 操作失败 |
| 50013 | platform state cannot be parsed | 平台页面或状态解析失败 |

## 数据结构

### BrowserProfile

```json
{
  "id": "profile-id",
  "name": "profile name",
  "status": "running",
  "cdp_url": "http://127.0.0.1:9222",
  "platform": "",
  "headless": false,
  "screen_width": 1920,
  "screen_height": 1080,
  "vnc_ws_port": 0,
  "user_data_dir": "",
  "created_at": "2026-06-01T12:00:00+08:00",
  "updated_at": "2026-06-01T12:00:00+08:00"
}
```

### FileInfo

```json
{
  "id": "file-id",
  "name": "file.txt",
  "path": "/folder/file.txt",
  "is_dir": false,
  "size": 12345,
  "mime_type": "text/plain",
  "created_at": "2026-06-01T12:00:00+08:00",
  "updated_at": "2026-06-01T12:00:00+08:00",
  "extra": {}
}
```

### OfflineTask

```json
{
  "task_id": "115:xxxx",
  "provider_task_id": "xxxx",
  "status": "running",
  "name": "file.zip",
  "progress": 50,
  "save_path": "/downloads",
  "error_code": "",
  "error_message": "",
  "files": [],
  "created_at": "2026-06-01T12:00:00+08:00",
  "updated_at": "2026-06-01T12:00:00+08:00"
}
```

`status` 可能值：`pending`、`running`、`completed`、`failed`、`canceled`、`unknown`。

## 系统接口

### 健康检查

`GET /health`

请求示例：

```bash
curl "http://localhost:8091/health"
```

成功响应：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "status": "ok"
  }
}
```

### 获取 Driver 列表

`GET /drivers`

请求示例：

```bash
curl "http://localhost:8091/drivers"
```

成功响应：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "drivers": ["115"]
  }
}
```

## Profile 管理

### 获取 Profile 列表

`GET /profiles`

请求示例：

```bash
curl "http://localhost:8091/profiles"
```

成功响应：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "profiles": []
  }
}
```

### 获取 Profile 详情

`GET /profiles/:id`

路径参数：

| 参数 | 必填 | 说明 |
|---|---|---|
| id | 是 | Profile ID |

请求示例：

```bash
curl "http://localhost:8091/profiles/your-profile-id"
```

成功响应：`data` 为 `BrowserProfile`。

### 启动 Profile

`POST /profiles/:id/start`

路径参数：

| 参数 | 必填 | 说明 |
|---|---|---|
| id | 是 | Profile ID |

请求示例：

```bash
curl -X POST "http://localhost:8091/profiles/your-profile-id/start"
```

成功响应：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "status": "started"
  }
}
```

### 停止 Profile

`POST /profiles/:id/stop`

请求示例：

```bash
curl -X POST "http://localhost:8091/profiles/your-profile-id/stop"
```

成功响应：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "status": "stopped"
  }
}
```

## Driver 能力

### 获取平台能力

`GET /drivers/:platform/capabilities`

路径参数：

| 参数 | 必填 | 说明 |
|---|---|---|
| platform | 是 | 平台标识，当前为 `115` |

请求示例：

```bash
curl "http://localhost:8091/drivers/115/capabilities"
```

成功响应：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "platform": "115",
    "offline_download": true,
    "fs_list": true,
    "fs_search": true,
    "media_url": true,
    "capabilities": {
      "offline_download": true,
      "file_manage": true,
      "search": true,
      "direct_link": true,
      "media_info": false
    }
  }
}
```

## 离线下载

以下接口均需要 `X-Profile-ID`。

### 添加离线任务

`POST /drivers/:platform/offline/add`

请求体：

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| url | string | 是 | 下载链接、磁力链接等 |
| category | string | 否 | 分类 |
| save_path | string | 否 | 保存目录 |
| client_task_id | string | 否 | 客户端幂等 ID；相同 Profile、平台、ID 会返回已有任务 |
| metadata | object | 否 | 自定义字符串键值 |

请求示例：

```bash
curl -X POST "http://localhost:8091/drivers/115/offline/add" \
  -H "X-Profile-ID: your-profile-id" \
  -H "Content-Type: application/json" \
  -d '{
    "url": "magnet:?xt=urn:btih:example",
    "save_path": "/downloads",
    "client_task_id": "manual-test-001",
    "metadata": {
      "source": "manual"
    }
  }'
```

成功响应：`data` 为 `OfflineTask`。

### 获取离线任务列表

`GET /drivers/:platform/offline/tasks`

请求示例：

```bash
curl "http://localhost:8091/drivers/115/offline/tasks" \
  -H "X-Profile-ID: your-profile-id"
```

成功响应：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "items": [],
    "total": 0
  }
}
```

### 查询离线任务

`GET /drivers/:platform/offline/tasks/:id`

路径参数：

| 参数 | 必填 | 说明 |
|---|---|---|
| id | 是 | 接口返回的 `task_id` |

请求示例：

```bash
curl "http://localhost:8091/drivers/115/offline/tasks/115:xxxx" \
  -H "X-Profile-ID: your-profile-id"
```

成功响应：`data` 为 `OfflineTask`。

### 删除离线任务

`DELETE /drivers/:platform/offline/tasks/:id`

请求示例：

```bash
curl -X DELETE "http://localhost:8091/drivers/115/offline/tasks/115:xxxx" \
  -H "X-Profile-ID: your-profile-id"
```

成功响应：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "task_id": "115:xxxx",
    "deleted": true
  }
}
```

## 文件系统

以下接口均需要 `X-Profile-ID`。

### 创建目录

`POST /drivers/:platform/fs/mkdir`

支持两种传参方式。

方式一：直接传完整目录路径：

```json
{
  "path": "/parent/new-folder"
}
```

方式二：传父目录和目录名：

```json
{
  "parent_path": "/parent",
  "name": "new-folder"
}
```

请求示例：

```bash
curl -X POST "http://localhost:8091/drivers/115/fs/mkdir" \
  -H "X-Profile-ID: your-profile-id" \
  -H "Content-Type: application/json" \
  -d '{"path": "/manual-test/new-folder"}'
```

成功响应：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "status": "created"
  }
}
```

### 删除文件或目录

`DELETE /drivers/:platform/fs/remove`

请求体：

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| path | string | 是 | 要删除的文件或目录路径 |

请求示例：

```bash
curl -X DELETE "http://localhost:8091/drivers/115/fs/remove" \
  -H "X-Profile-ID: your-profile-id" \
  -H "Content-Type: application/json" \
  -d '{"path": "/manual-test/new-folder"}'
```

成功响应：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "status": "removed"
  }
}
```

### 移动文件或目录

`POST /drivers/:platform/fs/move`

请求体：

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| src | string | 是 | 源路径 |
| dst | string | 是 | 目标路径 |

请求示例：

```bash
curl -X POST "http://localhost:8091/drivers/115/fs/move" \
  -H "X-Profile-ID: your-profile-id" \
  -H "Content-Type: application/json" \
  -d '{"src": "/manual-test/a.txt", "dst": "/manual-test/sub/a.txt"}'
```

成功响应：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "status": "moved"
  }
}
```

### 重命名文件或目录

`POST /drivers/:platform/fs/rename`

请求体：

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| path | string | 是 | 原路径 |
| new_name | string | 是 | 新名称，不是完整路径 |

请求示例：

```bash
curl -X POST "http://localhost:8091/drivers/115/fs/rename" \
  -H "X-Profile-ID: your-profile-id" \
  -H "Content-Type: application/json" \
  -d '{"path": "/manual-test/old-name", "new_name": "new-name"}'
```

成功响应：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "status": "renamed"
  }
}
```

### 列出目录内容

`GET /drivers/:platform/fs/list`

Query 参数：

| 参数 | 必填 | 说明 |
|---|---|---|
| path | 否 | 目录路径；为空时由 Driver 使用默认目录 |

请求示例：

```bash
curl "http://localhost:8091/drivers/115/fs/list?path=/" \
  -H "X-Profile-ID: your-profile-id"
```

成功响应：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "files": []
  }
}
```

`files` 数组元素为 `FileInfo`。

### 搜索文件

`GET /drivers/:platform/fs/search`

Query 参数：

| 参数 | 必填 | 说明 |
|---|---|---|
| keyword | 是 | 搜索关键词 |

请求示例：

```bash
curl "http://localhost:8091/drivers/115/fs/search?keyword=movie" \
  -H "X-Profile-ID: your-profile-id"
```

成功响应：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "files": []
  }
}
```

## 媒体

以下接口均需要 `X-Profile-ID`。

### 获取下载链接

`GET /drivers/:platform/media/url`

Query 参数：

| 参数 | 必填 | 说明 |
|---|---|---|
| file_id | 否 | 文件 ID；`file_id` 和 `path` 至少传一个 |
| path | 否 | 文件路径；`file_id` 和 `path` 至少传一个 |

同时传 `file_id` 和 `path` 时，优先使用 `file_id`。

按文件 ID 请求：

```bash
curl "http://localhost:8091/drivers/115/media/url?file_id=your-file-id" \
  -H "X-Profile-ID: your-profile-id"
```

按路径请求：

```bash
curl "http://localhost:8091/drivers/115/media/url?path=/manual-test/a.txt" \
  -H "X-Profile-ID: your-profile-id"
```

成功响应：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "url": "https://example.com/download"
  }
}
```

## 推荐手测顺序

1. `GET /health`
2. `GET /drivers`
3. `GET /profiles`
4. `POST /profiles/:id/start`
5. 在浏览器中确认 115 已登录
6. `GET /drivers/115/capabilities`
7. `GET /drivers/115/fs/list?path=/`
8. `POST /drivers/115/fs/mkdir`
9. `GET /drivers/115/fs/search?keyword=<关键词>`
10. `POST /drivers/115/offline/add`
11. `GET /drivers/115/offline/tasks`
12. `GET /drivers/115/offline/tasks/:id`
13. `GET /drivers/115/media/url?file_id=<文件ID>` 或 `?path=<文件路径>`
