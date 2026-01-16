package middlewares

import (
	"crypto/subtle"

	"github.com/OpenListTeam/OpenList/v4/internal/conf"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/OpenListTeam/OpenList/v4/internal/op"
	"github.com/OpenListTeam/OpenList/v4/internal/setting"
	"github.com/OpenListTeam/OpenList/v4/server/common"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

// Auth is a middleware that checks if the user is logged in.
// if token is empty, set user to guest
func Auth(allowDisabledGuest bool) func(c *gin.Context) {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if subtle.ConstantTimeCompare([]byte(token), []byte(setting.GetStr(conf.Token))) == 1 {
			admin, err := op.GetAdmin()
			if err != nil {
				common.ErrorResp(c, err, 500)
				c.Abort()
				return
			}
			common.GinWithValue(c, conf.UserKey, admin)
			log.Debugf("use admin token: %+v", admin)
			c.Next()
			return
		}
		if token == "" {
			guest, err := op.GetGuest()
			if err != nil {
				common.ErrorResp(c, err, 500)
				c.Abort()
				return
			}
			if !allowDisabledGuest && guest.Disabled {
				common.ErrorStrResp(c, "Guest user is disabled, login please", 401)
				c.Abort()
				return
			}
			common.GinWithValue(c, conf.UserKey, guest)
			log.Debugf("use empty token: %+v", guest)
			c.Next()
			return
		}
		userClaims, err := common.ParseToken(token)
		if err != nil {
			common.ErrorResp(c, err, 401)
			c.Abort()
			return
		}
		user, err := op.GetUserByName(userClaims.Username)
		if err != nil {
			common.ErrorResp(c, err, 401)
			c.Abort()
			return
		}
		// validate password timestamp
		if userClaims.PwdTS != user.PwdTS {
			common.ErrorStrResp(c, "Password has been changed, login please", 401)
			c.Abort()
			return
		}
		if user.Disabled {
			common.ErrorStrResp(c, "Current user is disabled, replace please", 401)
			c.Abort()
			return
		}
		common.GinWithValue(c, conf.UserKey, user)
		log.Debugf("use login token: %+v", user)
		c.Next()
	}
}

// AuthOptional 可选认证中间件，尝试解析用户但不强制要求
// 用于下载路由，尝试从 Authorization header 或 token query 参数获取用户信息
func AuthOptional(c *gin.Context) {
	// 尝试从 header 获取 token
	token := c.GetHeader("Authorization")
	// 如果 header 没有，尝试从 query 参数获取
	if token == "" {
		token = c.Query("token")
	}
	
	// 检查是否是管理员 token
	if token != "" && subtle.ConstantTimeCompare([]byte(token), []byte(setting.GetStr(conf.Token))) == 1 {
		admin, err := op.GetAdmin()
		if err == nil {
			common.GinWithValue(c, conf.UserKey, admin)
			log.Debugf("auth optional: use admin token")
			c.Next()
			return
		}
	}
	
	// 尝试解析用户 token
	if token != "" {
		userClaims, err := common.ParseToken(token)
		if err == nil {
			user, err := op.GetUserByName(userClaims.Username)
			if err == nil && userClaims.PwdTS == user.PwdTS && !user.Disabled {
				common.GinWithValue(c, conf.UserKey, user)
				log.Debugf("auth optional: use user token: %s", user.Username)
				c.Next()
				return
			}
		}
	}
	
	// 如果没有有效 token，设置为 guest（但不阻止请求）
	guest, err := op.GetGuest()
	if err == nil {
		common.GinWithValue(c, conf.UserKey, guest)
	}
	log.Debugf("auth optional: use guest")
	c.Next()
}

func Authn(c *gin.Context) {
	token := c.GetHeader("Authorization")
	if subtle.ConstantTimeCompare([]byte(token), []byte(setting.GetStr(conf.Token))) == 1 {
		admin, err := op.GetAdmin()
		if err != nil {
			common.ErrorResp(c, err, 500)
			c.Abort()
			return
		}
		common.GinWithValue(c, conf.UserKey, admin)
		log.Debugf("use admin token: %+v", admin)
		c.Next()
		return
	}
	if token == "" {
		guest, err := op.GetGuest()
		if err != nil {
			common.ErrorResp(c, err, 500)
			c.Abort()
			return
		}
		common.GinWithValue(c, conf.UserKey, guest)
		log.Debugf("use empty token: %+v", guest)
		c.Next()
		return
	}
	userClaims, err := common.ParseToken(token)
	if err != nil {
		common.ErrorResp(c, err, 401)
		c.Abort()
		return
	}
	user, err := op.GetUserByName(userClaims.Username)
	if err != nil {
		common.ErrorResp(c, err, 401)
		c.Abort()
		return
	}
	// validate password timestamp
	if userClaims.PwdTS != user.PwdTS {
		common.ErrorStrResp(c, "Password has been changed, login please", 401)
		c.Abort()
		return
	}
	if user.Disabled {
		common.ErrorStrResp(c, "Current user is disabled, replace please", 401)
		c.Abort()
		return
	}
	common.GinWithValue(c, conf.UserKey, user)
	log.Debugf("use login token: %+v", user)
	c.Next()
}

func AuthNotGuest(c *gin.Context) {
	user := c.Request.Context().Value(conf.UserKey).(*model.User)
	if user.IsGuest() {
		common.ErrorStrResp(c, "You are a guest", 403)
		c.Abort()
	} else {
		c.Next()
	}
}

func AuthAdmin(c *gin.Context) {
	user := c.Request.Context().Value(conf.UserKey).(*model.User)
	if !user.IsAdmin() {
		common.ErrorStrResp(c, "You are not an admin", 403)
		c.Abort()
	} else {
		c.Next()
	}
}
