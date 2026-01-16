package sign

import (
	"sync"
	"time"

	"github.com/OpenListTeam/OpenList/v4/internal/conf"
	"github.com/OpenListTeam/OpenList/v4/internal/setting"
	"github.com/OpenListTeam/OpenList/v4/pkg/sign"
)

var once sync.Once
var instance sign.Sign

func Sign(data string) string {
	expire := setting.GetInt(conf.LinkExpiration, 0)
	if expire == 0 {
		return NotExpired(data)
	} else {
		return WithDuration(data, time.Duration(expire)*time.Hour)
	}
}

// SignWithUser 生成包含用户名的签名
// 签名数据格式: path|username
func SignWithUser(path string, username string) string {
	expire := setting.GetInt(conf.LinkExpiration, 0)
	dataWithUser := path + "|" + username
	if expire == 0 {
		return NotExpired(dataWithUser)
	} else {
		return WithDuration(dataWithUser, time.Duration(expire)*time.Hour)
	}
}

// VerifyWithUser 验证包含用户名的签名
// 返回错误，如果验证成功则返回nil
func VerifyWithUser(path string, username string, signStr string) error {
	once.Do(Instance)
	dataWithUser := path + "|" + username
	return instance.Verify(dataWithUser, signStr)
}

func WithDuration(data string, d time.Duration) string {
	once.Do(Instance)
	return instance.Sign(data, time.Now().Add(d).Unix())
}

func NotExpired(data string) string {
	once.Do(Instance)
	return instance.Sign(data, 0)
}

func Verify(data string, sign string) error {
	once.Do(Instance)
	return instance.Verify(data, sign)
}

func Instance() {
	instance = sign.NewHMACSign([]byte(setting.GetStr(conf.Token)))
}
