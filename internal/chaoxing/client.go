// 协议来源：CxStudy cxapi/session.py、cxapi/utils.py: 会话、mobile_ua_sign、get_ua；见 UPSTREAM.md。
package chaoxing

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"time"
)

const webUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36"

type Client struct {
	HTTP     *http.Client
	Cookies  string
	UA       string
	DeviceID string
}

func NewClient(path string) (*Client, error) {
	jar, _ := cookiejar.New(nil)
	device := make([]byte, 16)
	if _, err := rand.Read(device); err != nil {
		return nil, err
	}
	imei := hex.EncodeToString(device)
	suffix := "(device:MI12) Language/zh_CN com.chaoxing.mobile/ChaoXingStudy_3_6.3.9_android_phone_10824_250 (@Kalimdor)_" + imei
	sign := md5.Sum([]byte("(schild:ipL$TkeiEmfy1gTXb2XHrdLN0a@7c^vu) " + suffix))
	ua := "Dalvik/2.1.0 (Linux; U; Android 12; MI12 Build/SKQ1.211006.001) (schild:" + hex.EncodeToString(sign[:]) + ") " + suffix
	c := &Client{HTTP: &http.Client{Jar: jar, Timeout: 30 * time.Second}, Cookies: path, UA: ua, DeviceID: imei}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return c, nil
	}
	if err != nil {
		return nil, err
	}
	var cookies []*http.Cookie
	for _, p := range strings.Split(strings.TrimSpace(string(b)), ";") {
		name, value, ok := strings.Cut(strings.TrimSpace(p), "=")
		if ok {
			cookies = append(cookies, &http.Cookie{Name: name, Value: value, Domain: ".chaoxing.com", Path: "/"})
		}
	}
	u, _ := url.Parse("https://chaoxing.com")
	jar.SetCookies(u, cookies)
	return c, nil
}
func (c *Client) cookie(name string) string {
	u, _ := url.Parse("https://mooc1-api.chaoxing.com")
	for _, x := range c.HTTP.Jar.Cookies(u) {
		if x.Name == name {
			return x.Value
		}
	}
	return ""
}
func (c *Client) request(ctx context.Context, method, endpoint string, q, form url.Values) ([]byte, error) {
	u, _ := url.Parse(endpoint)
	if q != nil {
		u.RawQuery = q.Encode()
	}
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.UA)
	if strings.Contains(u.Host, "passport") {
		req.Header.Set("User-Agent", webUA)
	}
	req.Header.Set("X-Requested-With", "com.chaoxing.mobile")
	req.Header.Set("Referer", "https://mooc1.chaoxing.com/")
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	r, err := c.HTTP.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, Failure("NETWORK_ERROR", "网络请求失败："+u.Path, 6)
	}
	defer r.Body.Close()
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, Failure("NETWORK_ERROR", "响应读取失败", 6)
	}
	if strings.Contains(r.Request.URL.Host, "passport") && !strings.Contains(u.Host, "passport") {
		return nil, Failure("AUTH_REQUIRED", "登录会话已失效", 3)
	}
	if strings.Contains(r.Request.URL.Path, "antispiderShowVerify") || strings.Contains(string(b), "安全验证不通过") {
		return nil, Failure("CHALLENGE_REQUIRED", "需要先在学习通完成人机验证，再重新登录", 3)
	}
	if r.StatusCode == 401 {
		return nil, Failure("AUTH_REQUIRED", "请先使用 auth login 登录", 3)
	}
	if r.StatusCode == 403 {
		return nil, Failure("PERMISSION_DENIED", "平台拒绝请求："+u.Path, 4)
	}
	if r.StatusCode >= 400 {
		return nil, Failure("UPSTREAM_ERROR", "平台请求失败："+r.Status, 6)
	}
	return b, nil
}
