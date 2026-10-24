// 协议来源：CxStudy cxapi/api.py: login_passwd、accinfo；固定版本及适配说明见 UPSTREAM.md。
package chaoxing

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func (c *Client) Auth(ctx context.Context) (any, error) {
	b, err := c.request(ctx, "GET", "https://sso.chaoxing.com/apis/login/userLogin4Uname.do", nil, nil)
	if err != nil {
		return nil, err
	}
	var j map[string]any
	if err = decodeJSON(b, &j); err != nil {
		return nil, Failure("UPSTREAM_FORMAT", "登录状态响应不是 JSON", 6)
	}
	if !truth(j["result"]) {
		return nil, Failure("AUTH_REQUIRED", "登录会话已失效", 3)
	}
	return map[string]any{"authenticated": true}, nil
}
func encryptPassword(s string) string {
	key := []byte("u2oh6Vu^HWe4_AES")
	b := []byte(s)
	pad := aes.BlockSize - len(b)%aes.BlockSize
	for i := 0; i < pad; i++ {
		b = append(b, byte(pad))
	}
	block, _ := aes.NewCipher(key)
	cipher.NewCBCEncrypter(block, key).CryptBlocks(b, b)
	return base64.StdEncoding.EncodeToString(b)
}
func (c *Client) Login(ctx context.Context, phone, password string) (any, error) {
	b, err := c.request(ctx, "POST", "https://passport2.chaoxing.com/fanyalogin", nil, url.Values{"fid": {"-1"}, "uname": {encryptPassword(phone)}, "password": {encryptPassword(password)}, "t": {"true"}, "refer": {"https://i.chaoxing.com"}})
	if err != nil {
		return nil, err
	}
	var j map[string]any
	if decodeJSON(b, &j) != nil {
		return nil, Failure("UPSTREAM_FORMAT", "登录响应不是 JSON", 6)
	}
	if !truth(j["status"]) {
		return nil, Failure("AUTH_REQUIRED", first(j["msg2"], j["msg"], "登录失败，请使用网页登录并导入 Cookie"), 3)
	}
	if jump := str(j["url"]); jump != "" {
		jump, _ = url.QueryUnescape(jump)
		if _, err = c.request(ctx, "GET", jump, nil, nil); err != nil {
			return nil, err
		}
	}
	u, _ := url.Parse("https://passport2.chaoxing.com")
	var saved []string
	for _, x := range c.HTTP.Jar.Cookies(u) {
		saved = append(saved, x.Name+"="+x.Value)
	}
	if len(saved) == 0 {
		return nil, Failure("AUTH_REQUIRED", "平台没有返回登录 Cookie", 3)
	}
	if err = os.MkdirAll(filepath.Dir(c.Cookies), 0700); err != nil {
		return nil, err
	}
	if err = os.WriteFile(c.Cookies, []byte(strings.Join(saved, ";")), 0600); err != nil {
		return nil, err
	}
	return map[string]any{"authenticated": true, "cookie_file": c.Cookies}, nil
}
