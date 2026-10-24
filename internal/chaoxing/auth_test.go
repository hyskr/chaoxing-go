package chaoxing

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestLoginOnlySavesSession(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.PostForm.Get("uname") == "12345678901" || r.PostForm.Get("password") == "secret" {
			t.Fatal("账密未加密")
		}
		if r.URL.Path != "/fanyalogin" {
			t.Fatal(r.URL.Path)
		}
		http.SetCookie(w, &http.Cookie{Name: "_uid", Value: "42", Domain: ".chaoxing.com", Path: "/"})
		w.Write([]byte(`{"status":true}`))
	})
	if _, e := c.Login(context.Background(), "12345678901", "secret"); e != nil {
		t.Fatal(e)
	}
	data, e := os.ReadFile(c.Cookies)
	if e != nil || strings.Contains(string(data), "secret") || !strings.Contains(string(data), "_uid=42") {
		t.Fatalf("会话保存失败: %v", e)
	}
}
