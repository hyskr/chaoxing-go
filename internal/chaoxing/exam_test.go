package chaoxing

import (
	"context"
	"net/http"
	"testing"
)

func TestExamExportUsesCurrentSessionAndOptionKeys(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/exam-ans/exam/test/reVersionTestStartNew":
			if r.URL.Query().Get("tag") != "0" {
				t.Error("读取考试不应重新开始计时")
			}
			w.Write([]byte(`<form id="submitTest"><input id="enc" value="current"><input id="encRemainTime" value="3000"><input id="encLastUpdateTime" value="1234"></form>`))
		case "/exam-ans/exam/phone/preview":
			if r.URL.Query().Get("enc") != "current" || r.URL.Query().Get("remainTimeParam") != "3000" {
				t.Error("未使用页面的当前会话参数")
			}
			w.Write([]byte(`<form id="submitTest"><div class="allAnswerList questionWrap singleQuesId ans-cc-exam"><input name="questionId" value="101"><input name="type101" value="0"><input id="answer101" value="C"><div class="tit"><h3>单选题</h3>1.完整题干</div><div class="answerList radioList" name="C"><span class="No">A</span><cc>选项正文</cc></div></div></form>`))
		default:
			http.NotFound(w, r)
		}
	})
	p, e := c.ExportExam(context.Background(), "https://mooc1.chaoxing.com/exam-ans/exam/test/reVersionTestStartNew?courseId=1&classId=2&cpi=3&tId=4&id=5&enc=old&tag=1")
	if e != nil {
		t.Fatal(e)
	}
	if len(p.Questions) != 1 || p.Questions[0].Text != "完整题干" || p.Questions[0].Options[0].Key != "C" || p.Questions[0].Options[0].Label != "A" {
		t.Fatalf("导出内容不正确: %+v", p)
	}
}
