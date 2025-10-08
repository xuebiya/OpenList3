package middlewares

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/OpenListTeam/OpenList/v4/internal/conf"
	"github.com/OpenListTeam/OpenList/v4/internal/db"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

// MediaLogger 是一个专门记录媒体文件访问的日志中间件
// 它会完全替代原有的日志系统

// 初始化日志格式
func init() {
	// 设置日志格式为纯文本，不带颜色
	log.SetFormatter(&log.TextFormatter{
		DisableColors:    true,
		DisableTimestamp: true, // 禁用默认时间戳，我们将自己格式化
		FullTimestamp:    false,
	})
}

// 支持的媒体文件扩展名
var mediaExtensions = map[string]bool{
	// 图片格式
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".gif":  true,
	".bmp":  true,
	".webp": true,
	".svg":  true,
	".tiff": true,
	".ico":  true,
	".heic": true,

	// 视频格式
	".mp4":  true,
	".avi":  true,
	".mkv":  true,
	".mov":  true,
	".wmv":  true,
	".flv":  true,
	".webm": true,
	".m4v":  true,
	".mpg":  true,
	".mpeg": true,
	".3gp":  true,
	".rm":   true,
	".rmvb": true,
	".ts":   true,
	".m3u8": true,
}

// 要忽略的路径前缀
var ignoredPaths = []string{
	"/assets/",
	"/images/",
	"/favicon.ico",
	"/robots.txt",
	"/ping",
	"/manifest.json",
}

// 请求和响应的结构体，用于解析JSON
type fsRequest struct {
	Path     string `json:"path"`
	Password string `json:"password"`
}

type fsObject struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Type int    `json:"type"`
}

type fsListResponse struct {
	Code    int        `json:"code"`
	Content []fsObject `json:"content"`
}

type fsGetResponse struct {
	Code int      `json:"code"`
	Data fsObject `json:"data"`
}

// 共享信息结构
type sharingInfo struct {
	SharingID string
	Creator   string
	IsSharing bool
}

// 访问行为类型
type accessBehavior string

const (
	BehaviorDirectPlay   accessBehavior = "直接播放"
	BehaviorPlayerPlay   accessBehavior = "播放器播放"
	BehaviorDownload     accessBehavior = "下载"
	BehaviorBrowserView  accessBehavior = "浏览器查看"
)

// 常见的媒体播放器 User-Agent 标识
var playerIdentifiers = []string{
	"VLC",
	"MPlayer",
	"mpv",
	"PotPlayer",
	"KMPlayer",
	"IINA",
	"Kodi",
	"Plex",
	"Emby",
	"Jellyfin",
	"QuickTime",
	"Windows-Media-Player",
	"RealPlayer",
	"GStreamer",
	"lavf",      // FFmpeg/libavformat
	"NSPlayer",  // Windows Media Player
	"stagefright", // Android media player
	"ExoPlayer", // Android ExoPlayer
	"AppleCoreMedia", // Apple media framework
}

// 获取共享信息
func getSharingInfo(c *gin.Context) *sharingInfo {
	// 首先检查上下文中是否有共享ID（由 SharingIdParse 中间件设置）
	if sidVal, exists := c.Get(conf.SharingIDKey); exists {
		if sid, ok := sidVal.(string); ok && sid != "" {
			// 从数据库获取共享信息
			sharingDB := &model.SharingDB{}
			if err := db.GetByID(sharingDB, sid); err == nil {
				// 获取创建者信息
				creator := &model.User{}
				if err := db.GetByID(creator, sharingDB.CreatorId); err == nil {
					return &sharingInfo{
						SharingID: sid,
						Creator:   creator.Username,
						IsSharing: true,
					}
				}
				return &sharingInfo{
					SharingID: sid,
					Creator:   "未知创建者",
					IsSharing: true,
				}
			}
		}
	}

	// 检查请求路径是否是共享下载路径 /sd/:sid
	path := c.Request.URL.Path
	if strings.HasPrefix(path, "/sd/") {
		parts := strings.Split(strings.TrimPrefix(path, "/sd/"), "/")
		if len(parts) > 0 && parts[0] != "" {
			sid := parts[0]
			sharingDB := &model.SharingDB{}
			if err := db.GetByID(sharingDB, sid); err == nil {
				creator := &model.User{}
				if err := db.GetByID(creator, sharingDB.CreatorId); err == nil {
					return &sharingInfo{
						SharingID: sid,
						Creator:   creator.Username,
						IsSharing: true,
					}
				}
				return &sharingInfo{
					SharingID: sid,
					Creator:   "未知创建者",
					IsSharing: true,
				}
			}
		}
	}

	// 检查请求体中是否包含共享路径（用于API调用）
	if c.Request.Method == "POST" && c.Request.Body != nil {
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err == nil {
			c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			var req fsRequest
			if err := json.Unmarshal(bodyBytes, &req); err == nil {
				// 检查路径是否以共享ID开头（格式: /sharingID/path）
				if strings.HasPrefix(req.Path, "/") {
					parts := strings.Split(strings.TrimPrefix(req.Path, "/"), "/")
					if len(parts) > 0 && len(parts[0]) == 12 { // 共享ID长度为12
						sid := parts[0]
						sharingDB := &model.SharingDB{}
						if err := db.GetByID(sharingDB, sid); err == nil {
							creator := &model.User{}
							if err := db.GetByID(creator, sharingDB.CreatorId); err == nil {
								return &sharingInfo{
									SharingID: sid,
									Creator:   creator.Username,
									IsSharing: true,
								}
							}
							return &sharingInfo{
								SharingID: sid,
								Creator:   "未知创建者",
								IsSharing: true,
							}
						}
					}
				}
			}
		}
	}

	return &sharingInfo{IsSharing: false}
}

// 检测访问行为
func detectAccessBehavior(c *gin.Context) accessBehavior {
	userAgent := c.GetHeader("User-Agent")
	rangeHeader := c.GetHeader("Range")
	path := c.Request.URL.Path
	
	// 1. 检测是否是媒体播放器
	for _, identifier := range playerIdentifiers {
		if strings.Contains(userAgent, identifier) {
			return BehaviorPlayerPlay
		}
	}
	
	// 2. 检查路径判断访问类型
	// /p/* 路径通常用于代理播放
	if strings.HasPrefix(path, "/p/") {
		// 如果有 Range 请求头，说明是流式播放
		if rangeHeader != "" {
			return BehaviorDirectPlay
		}
		return BehaviorDirectPlay
	}
	
	// /d/* 路径通常用于下载
	if strings.HasPrefix(path, "/d/") {
		// 如果有 Range 请求头且是浏览器，可能是在线播放
		if rangeHeader != "" && isBrowser(userAgent) {
			return BehaviorDirectPlay
		}
		return BehaviorDownload
	}
	
	// /sd/* 共享下载路径
	if strings.HasPrefix(path, "/sd/") {
		// 如果有 Range 请求头且是浏览器，可能是在线播放
		if rangeHeader != "" && isBrowser(userAgent) {
			return BehaviorDirectPlay
		}
		return BehaviorDownload
	}
	
	// 3. 通过 Range 请求头判断
	if rangeHeader != "" {
		// 有 Range 请求通常表示流式播放
		if isBrowser(userAgent) {
			return BehaviorDirectPlay
		}
		return BehaviorPlayerPlay
	}
	
	// 4. 对于图片，通常是浏览器查看
	ext := strings.ToLower(filepath.Ext(path))
	imageExts := map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
		".bmp": true, ".webp": true, ".svg": true, ".ico": true, ".heic": true,
	}
	if imageExts[ext] {
		return BehaviorBrowserView
	}
	
	// 5. API 调用默认为浏览器查看
	if strings.HasPrefix(path, "/api/") {
		return BehaviorBrowserView
	}
	
	// 默认为下载
	return BehaviorDownload
}

// 判断是否为浏览器 User-Agent
func isBrowser(userAgent string) bool {
	browserIdentifiers := []string{
		"Mozilla", "Chrome", "Safari", "Firefox", "Edge", "Opera",
		"MSIE", "Trident", // Internet Explorer
	}
	
	for _, identifier := range browserIdentifiers {
		if strings.Contains(userAgent, identifier) {
			// 排除一些使用 Mozilla 标识但不是浏览器的客户端
			excludeIdentifiers := []string{
				"curl", "wget", "axios", "python", "java", "go-http-client",
			}
			for _, exclude := range excludeIdentifiers {
				if strings.Contains(strings.ToLower(userAgent), strings.ToLower(exclude)) {
					return false
				}
			}
			return true
		}
	}
	return false
}

// 获取用户名
func getUserName(c *gin.Context) string {
	// 尝试从上下文中获取用户对象
	userObj, exists := c.Get("user")
	if exists {
		// 检查是否可以转换为*model.User类型
		if user, ok := userObj.(*model.User); ok && user != nil {
			return user.Username
		}

		// 尝试从map中获取username
		if userMap, ok := userObj.(map[string]interface{}); ok {
			if username, exists := userMap["username"]; exists {
				if usernameStr, ok := username.(string); ok {
					return usernameStr
				}
			}
		}
	}

	// 尝试从Authorization头获取token并解析
	authHeader := c.GetHeader("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return "已认证用户"
	}

	// 如果无法获取用户名，返回访客
	return "访客"
}

// 格式化日志信息为标准格式（包含共享信息和访问行为）
func formatMediaLog(timestamp time.Time, clientIP string, filePath string, username string, behavior accessBehavior, sharing *sharingInfo) string {
	// 基本格式："时间：XXXX年X月X日 XX:XX:XX 访问IP：XXX.XXX.XXX.XXX 用户：XXX 行为：XXX 访问路径：XXX"
	if sharing != nil && sharing.IsSharing {
		// 共享访问格式："时间：XXXX年X月X日 XX:XX:XX 访问IP：XXX.XXX.XXX.XXX 用户：XXX 行为：XXX 共享ID：XXX 共享创建者：XXX 访问路径：XXX"
		return fmt.Sprintf("时间：%s 访问IP：%s 用户：%s 行为：%s 共享ID：%s 共享创建者：%s 访问路径：%s",
			timestamp.Format("2006年1月2日 15:04:05"),
			clientIP,
			username,
			behavior,
			sharing.SharingID,
			sharing.Creator,
			filePath)
	}
	// 普通访问格式
	return fmt.Sprintf("时间：%s 访问IP：%s 用户：%s 行为：%s 访问路径：%s",
		timestamp.Format("2006年1月2日 15:04:05"),
		clientIP,
		username,
		behavior,
		filePath)
}

// 输出日志到前台和日志文件
func logMediaAccess(timestamp time.Time, clientIP string, filePath string, username string, behavior accessBehavior, sharing *sharingInfo) {
	logMsg := formatMediaLog(timestamp, clientIP, filePath, username, behavior, sharing)

	// 输出到日志文件 - 使用纯文本格式，不带前缀
	log.Info(logMsg)

	// 输出到前台控制台
	fmt.Println(logMsg)
}

// MediaLoggerMiddleware 返回一个只记录媒体文件访问的日志中间件
func MediaLoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 如果是静态资源或其他忽略的路径，直接跳过
		path := c.Request.URL.Path
		for _, prefix := range ignoredPaths {
			if strings.HasPrefix(path, prefix) {
				c.Next()
				return
			}
		}

		// 获取共享信息
		sharing := getSharingInfo(c)

		// 检查是否是直接访问媒体文件的路径
		// 包括 /d/*path, /p/*path, /sd/:sid/*path 等
		if isMediaFilePath(path) || (strings.HasPrefix(path, "/sd/") && isMediaFileInPath(path)) {
			// 记录直接访问媒体文件的日志
			c.Next()

			clientIP := c.ClientIP()
			username := getUserName(c)
			behavior := detectAccessBehavior(c)

			// 使用新的日志格式记录
			logMediaAccess(time.Now(), clientIP, path, username, behavior, sharing)
			return
		}

		// 检查是否是API调用
		if strings.HasPrefix(path, "/api/") {
			// 如果是 /api/fs/list 或 /api/fs/get，需要特殊处理
			if path == "/api/fs/list" || strings.HasPrefix(path, "/api/fs/list?") {
				handleFSListRequest(c, sharing)
				return
			} else if path == "/api/fs/get" || strings.HasPrefix(path, "/api/fs/get?") {
				handleFSGetRequest(c, sharing)
				return
			}

			// 其他API调用不记录日志
			c.Next()
			return
		}

		// 默认情况下不记录日志
		c.Next()
	}
}

// 处理 /api/fs/list 请求
func handleFSListRequest(c *gin.Context, sharing *sharingInfo) {
	// 保存请求体
	var requestBody []byte
	if c.Request.Body != nil {
		requestBody, _ = io.ReadAll(c.Request.Body)
		// 恢复请求体，以便后续处理
		c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
	}

	// 创建响应体捕获器
	responseWriter := &responseBodyWriter{
		ResponseWriter: c.Writer,
		body:           &bytes.Buffer{},
	}
	c.Writer = responseWriter

	// 处理请求
	c.Next()

	// 检查请求体中是否包含媒体文件路径
	var req fsRequest
	if len(requestBody) > 0 {
		_ = json.Unmarshal(requestBody, &req)
	}

	// 检查响应体中是否包含媒体文件
	responseData := responseWriter.body.Bytes()
	var resp fsListResponse
	if len(responseData) > 0 {
		_ = json.Unmarshal(responseData, &resp)
	}

	// 检查响应中是否包含媒体文件
	hasMediaFile := false
	mediaFiles := []string{}

	if resp.Code == 200 && len(resp.Content) > 0 {
		for _, item := range resp.Content {
			if isMediaFileName(item.Name) {
				hasMediaFile = true
				// 使用响应中的完整路径
				mediaFiles = append(mediaFiles, item.Path)
			}
		}
	}

	// 如果包含媒体文件，记录日志
	if hasMediaFile {
		clientIP := c.ClientIP()
		username := getUserName(c)
		behavior := detectAccessBehavior(c)

		// 对每个媒体文件记录一条日志
		for _, mediaPath := range mediaFiles {
			logMediaAccess(time.Now(), clientIP, mediaPath, username, behavior, sharing)
		}
	}
}

// 处理 /api/fs/get 请求
func handleFSGetRequest(c *gin.Context, sharing *sharingInfo) {
	// 保存请求体
	var requestBody []byte
	if c.Request.Body != nil {
		requestBody, _ = io.ReadAll(c.Request.Body)
		// 恢复请求体，以便后续处理
		c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
	}

	// 创建响应体捕获器
	responseWriter := &responseBodyWriter{
		ResponseWriter: c.Writer,
		body:           &bytes.Buffer{},
	}
	c.Writer = responseWriter

	// 处理请求
	c.Next()

	// 检查请求体中是否包含媒体文件路径
	var req fsRequest
	if len(requestBody) > 0 {
		_ = json.Unmarshal(requestBody, &req)
	}

	// 检查响应体中是否包含媒体文件
	responseData := responseWriter.body.Bytes()
	var resp fsGetResponse
	if len(responseData) > 0 {
		_ = json.Unmarshal(responseData, &resp)
	}

	// 检查响应中是否包含媒体文件
	if resp.Code == 200 && isMediaFileName(resp.Data.Name) {
		clientIP := c.ClientIP()
		mediaPath := resp.Data.Path
		username := getUserName(c)
		behavior := detectAccessBehavior(c)

		// 使用新的日志格式记录
		logMediaAccess(time.Now(), clientIP, mediaPath, username, behavior, sharing)
	}
}

// 检查路径是否为媒体文件
func isMediaFilePath(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return mediaExtensions[ext]
}

// 检查路径中是否包含媒体文件（用于 /sd/:sid/path/file.mp4 这样的路径）
func isMediaFileInPath(path string) bool {
	// 从路径中提取文件名
	parts := strings.Split(path, "/")
	if len(parts) > 0 {
		filename := parts[len(parts)-1]
		ext := strings.ToLower(filepath.Ext(filename))
		return mediaExtensions[ext]
	}
	return false
}

// 检查文件名是否为媒体文件
func isMediaFileName(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return mediaExtensions[ext]
}

// responseBodyWriter 是一个用于捕获响应体的包装器
type responseBodyWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

// Write 实现 ResponseWriter 接口
func (w *responseBodyWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

// WriteString 实现 ResponseWriter 接口
func (w *responseBodyWriter) WriteString(s string) (int, error) {
	w.body.WriteString(s)
	return w.ResponseWriter.WriteString(s)
}

// Status 获取状态码
func (w *responseBodyWriter) Status() int {
	return w.ResponseWriter.Status()
}

// 启用调试模式的日志记录器
func MediaLoggerWithDebug() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 记录所有请求的开始信息
		path := c.Request.URL.Path

		// 获取共享信息
		sharing := getSharingInfo(c)

		// 获取请求体
		var requestBody []byte
		if c.Request.Body != nil && c.Request.Method != "GET" {
			requestBody, _ = io.ReadAll(c.Request.Body)
			// 恢复请求体，以便后续处理
			c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
		}

		// 创建响应体捕获器
		responseWriter := &responseBodyWriter{
			ResponseWriter: c.Writer,
			body:           &bytes.Buffer{},
		}
		c.Writer = responseWriter

		// 处理请求
		c.Next()

		// 检查是否为媒体文件访问
		isMedia := false
		mediaFilePath := path

		// 检查路径
		if isMediaFilePath(path) || (strings.HasPrefix(path, "/sd/") && isMediaFileInPath(path)) {
			isMedia = true
		}

		// 检查请求体
		if !isMedia && len(requestBody) > 0 {
			var req fsRequest
			if err := json.Unmarshal(requestBody, &req); err == nil && req.Path != "" {
				if strings.Contains(req.Path, ".") {
					ext := strings.ToLower(filepath.Ext(req.Path))
					if mediaExtensions[ext] {
						isMedia = true
						mediaFilePath = req.Path
					}
				}
			}
		}

		// 检查响应体
		responseData := responseWriter.body.Bytes()
		if !isMedia && len(responseData) > 0 {
			// 尝试解析为列表响应
			var listResp fsListResponse
			if err := json.Unmarshal(responseData, &listResp); err == nil && listResp.Code == 200 {
				for _, item := range listResp.Content {
					if isMediaFileName(item.Name) {
						isMedia = true
						mediaFilePath = item.Path
						break
					}
				}
			}

			// 尝试解析为单文件响应
			if !isMedia {
				var getResp fsGetResponse
				if err := json.Unmarshal(responseData, &getResp); err == nil && getResp.Code == 200 {
					if isMediaFileName(getResp.Data.Name) {
						isMedia = true
						mediaFilePath = getResp.Data.Path
					}
				}
			}
		}

		// 记录媒体文件访问日志
		if isMedia {
			clientIP := c.ClientIP()
			username := getUserName(c)
			behavior := detectAccessBehavior(c)
			logMediaAccess(time.Now(), clientIP, mediaFilePath, username, behavior, sharing)
		}
	}
}

