package common

import (
	stdpath "path"

	"github.com/OpenListTeam/OpenList/v4/internal/conf"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/OpenListTeam/OpenList/v4/internal/setting"
	"github.com/OpenListTeam/OpenList/v4/internal/sign"
)

// Sign 生成签名（兼容旧版本，不包含用户名）
func Sign(obj model.Obj, parent string, encrypt bool) string {
	if obj.IsDir() || (!encrypt && !setting.GetBool(conf.SignAll)) {
		return ""
	}
	return sign.Sign(stdpath.Join(parent, obj.GetName()))
}

// SignWithUser 生成包含用户名的签名
func SignWithUser(obj model.Obj, parent string, encrypt bool, username string) string {
	if obj.IsDir() || (!encrypt && !setting.GetBool(conf.SignAll)) {
		return ""
	}
	return sign.SignWithUser(stdpath.Join(parent, obj.GetName()), username)
}

// SignPathWithUser 为路径生成包含用户名的签名
func SignPathWithUser(path string, username string) string {
	return sign.SignWithUser(path, username)
}
