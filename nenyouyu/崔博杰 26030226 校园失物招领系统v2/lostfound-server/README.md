# 校园失物招领 · 极简后端（Go + Gin）+ 前端联调

数据用内存切片模拟，**不接数据库**，进程重启数据清空。内置 CORS，前端用浏览器直接打开（`file://`）即可调用。

```
default-workspace/
├─ campus-lost-found.html        ← 前端（单文件，已接入后端）
└─ lostfound-server/
   ├─ main.go                    ← 后端服务
   ├─ go.mod / go.sum
   ├─ api-test.mjs               ← 接口自检脚本（19 项断言）
   ├─ watch.mjs                  ← 后端实时监视器（第 4 步全链路回归用）
   └─ README.md
```

---

## 一、启动步骤

### 1. 启动后端

```bash
cd lostfound-server
go mod tidy            # 首次会下载 Gin
go run main.go         # 默认 http://127.0.0.1:8080
```

- 换端口：`set PORT=8081 && go run main.go`（Windows）/ `PORT=8081 go run main.go`（macOS、Linux）
- 安静日志：`set GIN_MODE=release`
- 编译：`go build -o server.exe main.go && .\server.exe`

### 2. 打开前端

**双击 `campus-lost-found.html` 即可**（`file://` 打开也支持，后端已放开 CORS）。
首页搜索框下方会显示连接状态：`· 已连接后端` / `· 本地演示数据`。

> 后端没启动时前端**不会白屏**：自动降级为本地演示数据（10 条示例 + 演示聊天），所有交互照常可用，只是不落库。

---

## 二、接口一览

| 方法 | 路径 | 说明 | 请求体 / 参数 |
|---|---|---|---|
| POST | `/api/register` | 注册 | `{"student_id":"20230012345","password":"123456","nickname":"测试同学"}` |
| POST | `/api/login` | 登录，返回假 Token | `{"student_id":"...","password":"..."}` |
| GET | `/api/items` | 列表（关键词 + 状态 + 类型筛选） | `?keyword=耳机&status=searching&type=lost` |
| POST | `/api/items` | 发布 | 见下方字段说明 |
| PUT | `/api/items/:id` | 修改（可只传要改的字段，如改状态） | `{"status":"done"}` |
| DELETE | `/api/items/:id` | 删除 | — |
| GET | `/api/health` | 健康检查 | — |

核心字段：`id`、`title`、`desc`、`type`（`lost` 失物 / `found` 招领）、`status`（`searching` 未结束 / `done` 已结束）、`owner`、`created_at`。

可选扩展字段（前端渲染用，用不到可删）：`campus`、`place`、`place_info`、`phone`、`date`、`imgs[{thumb,large}]`。

---

## 三、前端是怎么接的

前端只在原有逻辑上加了一层「API 优先、本地兜底」，改动集中在 4 个地方：

| 位置 | 行为 |
|---|---|
| `syncFromServer()` | 打开页面拉 `GET /api/items`，成功则以服务端为准，失败保留本地演示数据 |
| 发布 | `POST /api/items` → 成功后重新拉列表 |
| 修改信息 / 改状态 | `PUT /api/items/:id` → 改完回读 |
| 删除这条动态（详情页，仅自己） | `DELETE /api/items/:id` |
| 统一认证登录 | `POST /api/login`；若 401 说明该学号还没注册 → 自动 `POST /api/register` 再登录（页面上不出现注册入口） |

```js
const API_BASE = 'http://127.0.0.1:8080/api';

async function api(path, options) {
  const headers = { 'Content-Type': 'application/json' };
  if (authToken) headers.Authorization = 'Bearer ' + authToken;   // 假 Token
  const res = await fetch(API_BASE + path, { method: options?.method || 'GET', headers,
    body: options?.body ? JSON.stringify(options.body) : undefined });
  const data = await res.json().catch(() => null);
  if (!res.ok) throw new Error(data?.error || ('HTTP ' + res.status));
  return data;
}
```

Token 存在 `localStorage.lf_token`，用户信息存在 `localStorage.lf_user`。

---

## 四、如何验证整个程序能正常运行（5 步）

### 第 1 步：后端活着吗

```bash
curl http://127.0.0.1:8080/api/health
# 期望： {"status":"ok"}
```

### 第 2 步：接口/契约自检（19 条断言，一条命令）

```bash
cd lostfound-server
node api-test.mjs          # 换端口： node api-test.mjs 8081
```

脚本按**前端发出的完全相同的请求体**打后端，覆盖：列表 → 登录（含 401 自动注册）→ 发布（含图片/地点/联系方式）→ 回读字段不丢 → 改状态 → 关键词与状态筛选 → 改发布人 → 删除 → 404/400 错误分支。

末尾会打印 `结果：19 通过 / 0 失败`，全部通过即后端 OK。

### 第 3 步：前端连上没

1. 刷新 `campus-lost-found.html`，看搜索框下方提示是否为 **`· 已连接后端`**；
2. 看列表是否变成后端那 2 条种子数据（「校园卡」「黑色折叠雨伞」）而不是 10 条本地演示数据。

### 第 4 步：全链路回归（前端操作 → 后端数据）

这一步要证明「浏览器里的操作真的写进了后端」。开 **两个终端窗口** + 浏览器：

```bash
# 窗口 A：后端
cd lostfound-server && go run main.go

# 窗口 B：后端实时监视器（每 1.5 秒拉一次列表，把增/改/删实时打印出来）
cd lostfound-server && node watch.mjs        # 换端口： node watch.mjs 8081
```

窗口 B 启动后会先打印当前后端数据，例如：

```
已连接后端 http://127.0.0.1:8080/api（Ctrl+C 退出）
当前后端数据 2 条：
   #2  |  黑色折叠雨伞  |  招领  |  认领中  |  发布人=陈同学  |  地点=食堂  |  电话=-  |  图片=0张
   #1  |  校园卡  |  失物  |  寻找中  |  发布人=林同学  |  地点=图书馆  |  电话=-  |  图片=0张

现在去浏览器操作前端，这里会实时打印变化：
```

然后用浏览器打开 `campus-lost-found.html`，按下表操作，**眼睛盯着窗口 B**：

| 浏览器里的操作 | 窗口 B 应出现 |
|---|---|
| 个人中心 → 一键使用测试账号 | 无数据变化（提示「统一认证成功」即可） |
| 点右下角 + → 选「我丢失的物品」→ 填名称/地点/手机号 → 发布 | `+ 21:04:57 新增 #3 … 发布人=测试同学 …` |
| 点开该条 → 修改物品状态 → 选「已找到」→ 更改 → 确定 | `~ 修改 #3 xxx` → `status: searching → done` |
| 点开该条 → 修改信息 → 改名称/补充信息 → 保存 | `~ 修改 #3 xxx` → `title: 旧 → 新 ; place_info: 旧 → 新` |
| 详情页 → 删除这条动态 → 确定 | `- 删除 #3 xxx` |
| 详情页点头像 / 聊天页点头像 | 打开对应个人中心，不产生数据变化 |

实测样例输出：

```
+ 21:04:57 新增   #3 | 天青色测试水杯 | 失物 | 寻找中 | 发布人=测试同学 | 地点=图书馆 | 电话=13800001234 | 图片=0张
~ 21:05:00 修改   #3 天青色测试水杯
            status: searching → done
- 21:05:01 删除   #3 天青色测试水杯
```

**交叉验证（可选，二选一）**

1. 命令行查：
   ```bash
   curl "http://127.0.0.1:8080/api/items?keyword=水杯"
   ```
2. 浏览器 F12 → Network：每做一次操作都应看到一条 `/api/items` 请求（POST / PUT / DELETE）且状态码 200。

**如果窗口 B 没有任何 + / ~ / -**：说明操作没写到后端。检查搜索框下方是否为「· 已连接后端」；若显示「本地演示数据」，就是前端没连上（后端没开或端口不一致），此时操作只改了本地内存。

### 第 5 步：重启后端（验证数据是内存的）

`Ctrl+C` 停掉再 `go run main.go`：发布的数据会**消失**、种子数据恢复 —— 这是预期行为（内存存储，重启即清空）。前端刷新后会退回 2 条种子数据。

---

## 五、常见问题

**Q：前端显示「本地演示数据」？**
A：后端没起来或端口不对。确认 `go run main.go` 还在运行、`API_BASE` 与实际端口一致；打开浏览器 F12 Console 看是否有 `Failed to fetch`。

**Q：`listen tcp :8080: bind: Only one usage of each socket address`？**
A：端口被占用（可能上次的服务还在跑）。换端口 `set PORT=8081 && go run main.go`，或 `netstat -ano | findstr :8080` 找到 PID 后 `taskkill /F /PID <PID>`。

**Q：CORS 报错？**
A：中间件已对所有响应加 `Access-Control-Allow-Origin: *`，并对 `OPTIONS` 预检直接返回 204。若仍报错，检查是否有代理/浏览器插件拦截。

**Q：图片能存吗？**
A：可以，图片以 base64 data URL 存在内存里（前端上传时已压缩：缩略图 320px + 详情图 1024px）。图片较大时列表返回体会变长，生产环境请改存对象存储。

---

## 六、说明与限制（演示项目）

- 用户与物品都在内存里，**无数据库、无真实鉴权**：Token 只是 `fake-token-<学号>-<时间戳>` 字符串，服务端不校验；除注册/登录外任何人可改可删。
- 密码明文存在内存 map 中，仅供演示。
- 生产化需要：bcrypt 密码哈希、JWT/session 鉴权、数据库持久化、分页、图片对象存储。
