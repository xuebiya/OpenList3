package common

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/OpenListTeam/OpenList/v4/internal/conf"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/OpenListTeam/OpenList/v4/pkg/utils"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

// 访问行为类型
const (
	AccessTypePreview = "在线预览"
	AccessTypeDownload = "下载"
	AccessTypePlayer  = "播放器"
)

// 访问记录去重
var (
	accessCache     = make(map[string]time.Time)
	accessCacheLock sync.RWMutex
	dedupeWindow    = 5 * time.Second // 5秒内同一IP访问同一文件只记录一次
)

// 常见的图片格式
var imageExtensions = []string{
	"jpg", "jpeg", "png", "gif", "bmp", "webp", "svg", "ico", "tiff", "tif",
	"raw", "cr2", "nef", "arw", "dng", "heic", "heif", "avif",
}

// 常见的视频格式
var videoExtensions = []string{
	"mp4", "mkv", "avi", "mov", "wmv", "flv", "webm", "m4v", "mpeg", "mpg",
	"3gp", "3g2", "ts", "mts", "m2ts", "vob", "ogv", "rm", "rmvb", "asf",
	"f4v", "divx", "xvid",
}

// IsMediaFile 检查文件是否为图片或视频格式
func IsMediaFile(filename string) bool {
	ext := strings.ToLower(utils.Ext(filename))
	for _, e := range imageExtensions {
		if ext == e {
			return true
		}
	}
	for _, e := range videoExtensions {
		if ext == e {
			return true
		}
	}
	return false
}

// shouldLogAccess 检查是否应该记录此次访问（去重）
func shouldLogAccess(clientIP, rawPath string) bool {
	key := clientIP + "|" + rawPath
	now := time.Now()
	
	accessCacheLock.RLock()
	lastAccess, exists := accessCache[key]
	accessCacheLock.RUnlock()
	
	if exists && now.Sub(lastAccess) < dedupeWindow {
		return false // 在去重窗口内，不记录
	}
	
	accessCacheLock.Lock()
	accessCache[key] = now
	// 清理过期的缓存条目（简单清理，避免内存泄漏）
	if len(accessCache) > 1000 {
		for k, v := range accessCache {
			if now.Sub(v) > dedupeWindow*2 {
				delete(accessCache, k)
			}
		}
	}
	accessCacheLock.Unlock()
	
	return true
}

// detectAccessType 检测访问类型
func detectAccessType(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return AccessTypeDownload
	}
	
	userAgent := strings.ToLower(c.Request.UserAgent())
	path := c.Request.URL.Path
	
	// 常见播放器的 User-Agent 特征
	playerKeywords := []string{
		"vlc", "mpv", "potplayer", "mpc-hc", "mpc-be", "kodi", "plex",
		"infuse", "iina", "nplayer", "oplayer", "avplayer", "kmplayer",
		"gom", "daum", "lavf", "ffmpeg", "libmpv", "exoplayer",
		"stagefright", "android.media", "quicktime", "windows-media",
	}
	
	for _, keyword := range playerKeywords {
		if strings.Contains(userAgent, keyword) {
			return AccessTypePlayer
		}
	}
	
	// 根据请求路径判断
	// /d/ 路径是下载
	if strings.HasPrefix(path, "/d/") {
		return AccessTypeDownload
	}
	
	// /p/ 路径是代理/预览
	if strings.HasPrefix(path, "/p/") {
		return AccessTypePreview
	}
	
	return AccessTypeDownload
}

// LogMediaAccess 记录媒体文件访问日志（用于前端预览）
func LogMediaAccess(c *gin.Context, rawPath string) {
	LogMediaAccessWithType(c, rawPath, AccessTypePreview)
}

// LogMediaAccessWithType 记录媒体文件访问日志（指定类型）
func LogMediaAccessWithType(c *gin.Context, rawPath string, accessType string) {
	if !IsMediaFile(rawPath) {
		return
	}

	// 获取客户端IP
	clientIP := "unknown"
	if c != nil {
		clientIP = c.ClientIP()
	}

	// 去重检查
	if !shouldLogAccess(clientIP, rawPath) {
		return
	}

	// 获取用户信息
	username := "Guest"
	if c != nil && c.Request != nil && c.Request.Context() != nil {
		if user, ok := c.Request.Context().Value(conf.UserKey).(*model.User); ok && user != nil {
			username = user.Username
		}
	}

	// 格式化时间
	now := time.Now()
	timeStr := fmt.Sprintf("%d年%d月%d日 %02d:%02d:%02d",
		now.Year(), now.Month(), now.Day(),
		now.Hour(), now.Minute(), now.Second())

	// 构建日志消息
	logMsg := fmt.Sprintf("时间：%s 访问IP：%s 用户：%s 行为：%s 访问路径：%s",
		timeStr, clientIP, username, accessType, rawPath)

	// 使用logrus输出（会根据配置输出到文件或控制台）
	log.WithFields(log.Fields{
		"type":        "media_access",
		"ip":          clientIP,
		"user":        username,
		"access_type": accessType,
		"path":        rawPath,
	}).Info(logMsg)
	
	// 强制输出到标准错误（stderr通常不会被缓冲）
	fmt.Fprintln(os.Stderr, "[媒体访问] "+logMsg)
}

// LogMediaAccessAuto 自动检测访问类型并记录日志
func LogMediaAccessAuto(c *gin.Context, rawPath string) {
	accessType := detectAccessType(c)
	LogMediaAccessWithType(c, rawPath, accessType)
}
