package chaoxing

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestVideoCompletionRequiresPlatformAck(t *testing.T) {
	reports := 0
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/ananas/status/"):
			w.Write([]byte(`{"status":"success","duration":1,"dtoken":"test-token"}`))
		case strings.HasPrefix(r.URL.Path, "/multimedia/log/a/"):
			reports++
			if r.URL.Query().Get("playingTime") != "1" {
				t.Error("未使用断点进度")
			}
			w.Write([]byte(`{"isPassed":false}`))
		default:
			http.NotFound(w, r)
		}
	})
	e := c.learnVideo(context.Background(), Course{ID: "1", ClassID: "2", CPI: "3"}, Job{ID: "v1", ObjectID: "o1", Type: "video", PlayTime: 1000, raw: map[string]any{}, defaults: map[string]any{}}, 1, func(any) {})
	if e == nil || reports != 3 {
		t.Fatalf("错误宣告视频完成: %v reports=%d", e, reports)
	}
}

func TestDocumentCompletionUsesJobMarker(t *testing.T) {
	reports := 0
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/knowledge/cards":
			if reports == 0 {
				w.Write([]byte(`window.AttachmentSetting={"attachments":[{"jobid":"doc1","type":"document","job":true,"jtoken":"jt"}],"defaults":{}};`))
			} else {
				w.Write([]byte(`window.AttachmentSetting={"attachments":[{"jobid":"doc1","type":"document"}],"defaults":{}};`))
			}
		case "/ananas/job/document":
			if r.URL.Query().Get("jtoken") != "jt" {
				t.Error("缺少文档令牌")
			}
			reports++
			w.Write([]byte(`{"status":true}`))
		default:
			http.NotFound(w, r)
		}
	})
	result, e := c.Study(context.Background(), Course{ID: "1", ClassID: "2", CPI: "3"}, []Job{{ID: "doc1", Type: "document", ChapterID: "4"}}, 1, false, func(any) {})
	if e != nil || reports != 1 || len(result) != 1 || result[0].State != "completed" {
		t.Fatalf("未识别文档完成标记: %v %+v", e, result)
	}
}
