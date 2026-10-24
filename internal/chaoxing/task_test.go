package chaoxing

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestAllCardsAreScanned(t *testing.T) {
	cards := []string{}
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/gas/knowledge":
			w.Write([]byte(`{"data":[{"card":{"data":[{"description":"<iframe module='insertvideo'></iframe>"},{"description":"<iframe module='work'></iframe>"}]}}]}`))
		case "/knowledge/cards":
			num := r.URL.Query().Get("num")
			cards = append(cards, num)
			if num == "0" {
				w.Write([]byte(`window.AttachmentSetting={"attachments":[{"jobid":"v1","type":"video","isPassed":true,"property":{"objectid":"object"}}],"defaults":{}};`))
			} else {
				w.Write([]byte(`window.AttachmentSetting={"attachments":[{"jobid":"work-2","type":"workid","property":{"workid":"w2"}}],"defaults":{}};`))
			}
		default:
			http.NotFound(w, r)
		}
	})
	jobs, e := c.jobs(context.Background(), Course{ID: "1", ClassID: "2", CPI: "3"}, []Chapter{{ID: "4", Total: 2}})
	if e != nil {
		t.Fatal(e)
	}
	if len(jobs) != 2 || jobs[1].Type != "workid" || strings.Join(cards, ",") != "0,1" {
		t.Fatalf("遗漏后续任务卡: %+v %v", jobs, cards)
	}
	completed, e := c.jobs(context.Background(), Course{ID: "1", ClassID: "2", CPI: "3"}, []Chapter{{ID: "4", Total: 2, Finished: 2}})
	if e != nil || !completed[1].Passed {
		t.Fatalf("完整章节的测验应视为已完成: %v %+v", e, completed)
	}
}
