package middlewares

import (
	"fmt"
	"strings"
	"time"

	"github.com/OpenListTeam/OpenList/v4/internal/conf"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/OpenListTeam/OpenList/v4/pkg/utils"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
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

// isMediaFile 检查文件是否为图片或视频格式
func isMediaFile(filename string) bool {
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

// MediaAccessLog 记录图片和视频文件的访问日志
func MediaAccessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 先执行后续处理
		c.Next()

		// 获取访问路径
		rawPath, ok := c.Request.Context().Value(conf.PathKey).(string)
		if !ok || rawPath == "" {
			return
		}

		// 检查是否为媒体文件
		if !isMediaFile(rawPath) {
			return
		}

		// 获取用户信息
		username := "Guest"
		if user, ok := c.Request.Context().Value(conf.UserKey).(*model.User); ok && user != nil {
			username = user.Username
		}

		// 获取客户端IP
		clientIP := c.ClientIP()

		// 格式化时间
		now := time.Now()
		timeStr := fmt.Sprintf("%d年%d月%d日 %02d:%02d:%02d",
			now.Year(), now.Month(), now.Day(),
			now.Hour(), now.Minute(), now.Second())

		// 输出访问日志
		log.Infof("时间：%s 访问IP：%s 用户：%s 访问路径：%s",
			timeStr, clientIP, username, rawPath)
	}
}
