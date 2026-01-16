package common

import (
	"fmt"
	"os"
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

// LogMediaAccess 记录媒体文件访问日志
func LogMediaAccess(c *gin.Context, rawPath string) {
	if !IsMediaFile(rawPath) {
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

	// 构建日志消息
	logMsg := fmt.Sprintf("时间：%s 访问IP：%s 用户：%s 访问路径：%s",
		timeStr, clientIP, username, rawPath)

	// 输出日志
	log.Info(logMsg)
	fmt.Fprintln(os.Stdout, logMsg)
}
