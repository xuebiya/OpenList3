package middlewares

import (
	"strings"

	"github.com/OpenListTeam/OpenList/v4/internal/conf"
	"github.com/OpenListTeam/OpenList/v4/internal/op"
	"github.com/OpenListTeam/OpenList/v4/internal/setting"
	"github.com/OpenListTeam/OpenList/v4/internal/sign"

	"github.com/OpenListTeam/OpenList/v4/internal/errs"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/OpenListTeam/OpenList/v4/pkg/utils"
	"github.com/OpenListTeam/OpenList/v4/server/common"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

func PathParse(c *gin.Context) {
	rawPath := parsePath(c.Param("path"))
	common.GinWithValue(c, conf.PathKey, rawPath)
	c.Next()
}

func Down(verifyFunc func(string, string) error) func(c *gin.Context) {
	return func(c *gin.Context) {
		rawPath := c.Request.Context().Value(conf.PathKey).(string)
		meta, err := op.GetNearestMeta(rawPath)
		if err != nil {
			if !errors.Is(errors.Cause(err), errs.MetaNotFound) {
				common.ErrorPage(c, err, 500, true)
				return
			}
		}
		common.GinWithValue(c, conf.MetaKey, meta)
		
		// 获取URL中的签名
		signParam := strings.TrimSuffix(c.Query("sign"), "/")
		
		// 解析签名中的用户名（格式: sign:user:username 或 sign&user=username）
		var signStr, username string
		if strings.Contains(signParam, ":user:") {
			parts := strings.SplitN(signParam, ":user:", 2)
			signStr = parts[0]
			if len(parts) > 1 {
				username = parts[1]
			}
		} else {
			signStr = signParam
			username = c.Query("user")
		}
		
		// verify sign
		if needSign(meta, rawPath) {
			// 如果有用户名，尝试使用带用户名的签名验证
			if username != "" && signStr != "" {
				err = sign.VerifyWithUser(rawPath, username, signStr)
				if err == nil {
					// 签名验证成功，设置用户到context
					user, userErr := op.GetUserByName(username)
					if userErr == nil && user != nil {
						common.GinWithValue(c, conf.UserKey, user)
					}
					c.Next()
					return
				}
			}
			
			// 如果带用户名的验证失败或没有用户名，尝试普通验证
			err = verifyFunc(rawPath, signStr)
			if err != nil {
				common.ErrorPage(c, err, 401)
				c.Abort()
				return
			}
		} else {
			// 即使不需要签名验证，也尝试从签名参数恢复用户信息
			if username != "" && signStr != "" {
				err = sign.VerifyWithUser(rawPath, username, signStr)
				if err == nil {
					user, userErr := op.GetUserByName(username)
					if userErr == nil && user != nil {
						common.GinWithValue(c, conf.UserKey, user)
					}
				}
			}
		}
		c.Next()
	}
}

// TODO: implement
// path maybe contains # ? etc.
func parsePath(path string) string {
	return utils.FixAndCleanPath(path)
}

func needSign(meta *model.Meta, path string) bool {
	if setting.GetBool(conf.SignAll) {
		return true
	}
	if common.IsStorageSignEnabled(path) {
		return true
	}
	if meta == nil || meta.Password == "" {
		return false
	}
	if !meta.PSub && path != meta.Path {
		return false
	}
	return true
}
