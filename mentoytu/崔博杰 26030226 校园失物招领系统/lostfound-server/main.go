// 校园失物招领系统 · 极简后端服务
// 技术栈：Go + Gin，数据用内存切片模拟（不接数据库）
// 运行：go run main.go     默认监听 http://127.0.0.1:8080
package main

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

/* ===================== 数据模型 ===================== */

// ImageRef 一张图片：thumb 列表缩略图、large 详情高清图（base64 data URL）
type ImageRef struct {
	Thumb string `json:"thumb"`
	Large string `json:"large"`
}

// Item 一条失物 / 招领信息
// 前 6 个是核心字段；后面几个是前端页面用到的可选字段（用不到可直接删掉）
type Item struct {
	ID        int    `json:"id"`
	Title     string `json:"title"`  // 标题
	Desc      string `json:"desc"`   // 描述
	Type      string `json:"type"`   // lost = 失物(寻物) / found = 招领
	Status    string `json:"status"` // searching = 寻找中/认领中 / done = 已找到/已归还
	Owner     string `json:"owner"`  // 发布人
	CreatedAt string `json:"created_at"`

	Campus    string     `json:"campus,omitempty"`     // 校区
	Place     string     `json:"place,omitempty"`      // 地点标签
	PlaceInfo string     `json:"place_info,omitempty"` // 地点补充信息
	Phone     string     `json:"phone,omitempty"`      // 联系方式
	Date      string     `json:"date,omitempty"`       // 丢失 / 拾取日期 (2006-01-02)
	Imgs      []ImageRef `json:"imgs,omitempty"`       // 图片（可多张）
}

// 内存数据：全部用切片 + map 模拟，进程重启即清空
var (
	mu     sync.Mutex            // 并发保护
	items  = []Item{}            // 物品列表
	nextID = 1                   // 自增 ID
	users  = map[string]string{} // 学号 -> 密码（注册后写入）
)

/* ===================== CORS 中间件 ===================== */
// 前端用浏览器直接打开（file:// 或本地静态服务）时必须开启
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Header("Access-Control-Max-Age", "86400")

		// 浏览器的预检请求直接放行
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

/* ===================== 工具函数 ===================== */

// 假 Token：fake-token-<学号>-<时间戳>
func makeToken(studentID string) string {
	return "fake-token-" + studentID + "-" + strconv.FormatInt(time.Now().Unix(), 10)
}

// 从 Authorization: Bearer <token> 里取出学号（可选，用于自动填发布人）
func studentIDFromToken(c *gin.Context) string {
	auth := strings.TrimSpace(c.GetHeader("Authorization"))
	auth = strings.TrimPrefix(auth, "Bearer ")
	const prefix = "fake-token-"
	if !strings.HasPrefix(auth, prefix) {
		return ""
	}
	rest := strings.TrimPrefix(auth, prefix)
	if i := strings.LastIndex(rest, "-"); i > 0 {
		return rest[:i]
	}
	return rest
}

// 按 ID 查找，返回下标；找不到返回 -1
func indexOf(id int) int {
	for i := range items {
		if items[i].ID == id {
			return i
		}
	}
	return -1
}

// 解析路径参数 :id
func parseID(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id 不合法"})
		return 0, false
	}
	return id, true
}

/* ===================== 注册 / 登录 ===================== */

type authReq struct {
	StudentID string `json:"student_id"`
	Password  string `json:"password"`
	Nickname  string `json:"nickname"`
}

// POST /api/register 注册
func register(c *gin.Context) {
	var req authReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体不是合法 JSON"})
		return
	}
	req.StudentID = strings.TrimSpace(req.StudentID)
	if req.StudentID == "" || len(req.Password) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "学号不能为空，密码至少 6 位"})
		return
	}

	mu.Lock()
	defer mu.Unlock()
	if _, ok := users[req.StudentID]; ok {
		c.JSON(http.StatusConflict, gin.H{"error": "该学号已注册"})
		return
	}
	users[req.StudentID] = req.Password

	c.JSON(http.StatusOK, gin.H{
		"message":    "注册成功",
		"student_id": req.StudentID,
		"nickname":   defaultStr(req.Nickname, "同学 "+lastN(req.StudentID, 4)),
	})
}

// POST /api/login 登录，返回假 Token
func login(c *gin.Context) {
	var req authReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体不是合法 JSON"})
		return
	}

	mu.Lock()
	pwd, ok := users[req.StudentID]
	mu.Unlock()

	if !ok || pwd != req.Password {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "学号或密码错误"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "登录成功",
		"token":      makeToken(req.StudentID),
		"student_id": req.StudentID,
	})
}

/* ===================== 物品 CRUD ===================== */

type itemReq struct {
	Title  string `json:"title"`
	Desc   string `json:"desc"`
	Type   string `json:"type"`
	Status string `json:"status"`
	Owner  string `json:"owner"`

	Campus    string     `json:"campus"`
	Place     string     `json:"place"`
	PlaceInfo string     `json:"place_info"`
	Phone     string     `json:"phone"`
	Date      string     `json:"date"`
	Imgs      []ImageRef `json:"imgs"`
}

// GET /api/items?keyword=耳机&status=searching&type=lost
func listItems(c *gin.Context) {
	keyword := strings.ToLower(strings.TrimSpace(c.Query("keyword")))
	if keyword == "" {
		keyword = strings.ToLower(strings.TrimSpace(c.Query("q"))) // 兼容 q
	}
	status := strings.TrimSpace(c.Query("status"))
	typ := strings.TrimSpace(c.Query("type"))

	mu.Lock()
	result := make([]Item, 0, len(items))
	for i := len(items) - 1; i >= 0; i-- { // 倒序：最新发布在前
		it := items[i]
		if keyword != "" &&
			!strings.Contains(strings.ToLower(it.Title), keyword) &&
			!strings.Contains(strings.ToLower(it.Desc), keyword) {
			continue
		}
		if status != "" && status != "all" && it.Status != status {
			continue
		}
		if typ != "" && typ != "all" && it.Type != typ {
			continue
		}
		result = append(result, it)
	}
	mu.Unlock()

	c.JSON(http.StatusOK, gin.H{"total": len(result), "items": result})
}

// POST /api/items 发布
func createItem(c *gin.Context) {
	var req itemReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体不是合法 JSON"})
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "标题不能为空"})
		return
	}

	owner := strings.TrimSpace(req.Owner)
	if owner == "" {
		owner = defaultStr(studentIDFromToken(c), "匿名同学") // 带上 Token 就自动填发布人
	}

	mu.Lock()
	item := Item{
		ID:        nextID,
		Title:     req.Title,
		Desc:      strings.TrimSpace(req.Desc),
		Type:      defaultStr(req.Type, "lost"),
		Status:    defaultStr(req.Status, "searching"),
		Owner:     owner,
		CreatedAt: time.Now().Format("2006-01-02 15:04:05"),

		Campus:    req.Campus,
		Place:     defaultStr(req.Place, "其它"),
		PlaceInfo: req.PlaceInfo,
		Phone:     req.Phone,
		Date:      req.Date,
		Imgs:      req.Imgs,
	}
	nextID++
	items = append(items, item)
	mu.Unlock()

	c.JSON(http.StatusOK, gin.H{"message": "发布成功", "item": item})
}

// PUT /api/items/:id 修改（只传要改的字段即可，常用于改状态）
func updateItem(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	// 用指针接收，未传的字段保持原值
	var req struct {
		Title  *string `json:"title"`
		Desc   *string `json:"desc"`
		Type   *string `json:"type"`
		Status *string `json:"status"`
		Owner  *string `json:"owner"`

		Campus    *string     `json:"campus"`
		Place     *string     `json:"place"`
		PlaceInfo *string     `json:"place_info"`
		Phone     *string     `json:"phone"`
		Date      *string     `json:"date"`
		Imgs      *[]ImageRef `json:"imgs"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体不是合法 JSON"})
		return
	}

	mu.Lock()
	defer mu.Unlock()
	i := indexOf(id)
	if i < 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "物品不存在"})
		return
	}
	if req.Title != nil {
		items[i].Title = strings.TrimSpace(*req.Title)
	}
	if req.Desc != nil {
		items[i].Desc = strings.TrimSpace(*req.Desc)
	}
	if req.Type != nil {
		items[i].Type = *req.Type
	}
	if req.Status != nil {
		items[i].Status = *req.Status
	}
	if req.Owner != nil {
		items[i].Owner = *req.Owner
	}
	if req.Campus != nil {
		items[i].Campus = *req.Campus
	}
	if req.Place != nil {
		items[i].Place = *req.Place
	}
	if req.PlaceInfo != nil {
		items[i].PlaceInfo = *req.PlaceInfo
	}
	if req.Phone != nil {
		items[i].Phone = *req.Phone
	}
	if req.Date != nil {
		items[i].Date = *req.Date
	}
	if req.Imgs != nil {
		items[i].Imgs = *req.Imgs
	}

	c.JSON(http.StatusOK, gin.H{"message": "修改成功", "item": items[i]})
}

// DELETE /api/items/:id 删除
func deleteItem(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	mu.Lock()
	defer mu.Unlock()
	i := indexOf(id)
	if i < 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "物品不存在"})
		return
	}
	items = append(items[:i], items[i+1:]...)

	c.JSON(http.StatusOK, gin.H{"message": "删除成功", "id": id})
}

/* ===================== 小工具 ===================== */

func defaultStr(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

func lastN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

/* ===================== 启动 ===================== */

func main() {
	// 预置两条演示数据，方便前端一打开就能看到列表
	items = append(items,
		Item{ID: nextID, Title: "校园卡", Desc: "卡套是浅蓝色的，背面贴有小熊贴纸",
			Type: "lost", Status: "searching", Owner: "林同学",
			Campus: "东校区", Place: "图书馆", PlaceInfo: "一楼大厅 · 自助借还机旁",
			CreatedAt: time.Now().Format("2006-01-02 15:04:05")},
	)
	nextID++
	items = append(items,
		Item{ID: nextID, Title: "黑色折叠雨伞", Desc: "伞柄上有一条银色装饰环",
			Type: "found", Status: "searching", Owner: "陈同学",
			Campus: "东校区", Place: "食堂", PlaceInfo: "正门伞架 · 伞柄有银环",
			CreatedAt: time.Now().Format("2006-01-02 15:04:05")},
	)
	nextID++

	r := gin.Default()
	r.Use(CORS()) // 全局 CORS

	api := r.Group("/api")
	{
		api.POST("/register", register)
		api.POST("/login", login)
		api.GET("/items", listItems)
		api.POST("/items", createItem)
		api.PUT("/items/:id", updateItem)
		api.DELETE("/items/:id", deleteItem)
		api.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	}

	port := defaultStr(os.Getenv("PORT"), "8080") // 默认 8080，可用环境变量 PORT 改端口

	log.Println("服务已启动： http://127.0.0.1:" + port)
	log.Println("接口前缀： /api    （Ctrl+C 停止）")
	if err := r.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}
