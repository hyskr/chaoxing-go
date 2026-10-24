package chaoxing

import (
	"context"
	"net/http"
	"testing"
)

const quizFixture = `<html><title>章节测验</title><h3 class="chapter-title">随堂练习</h3><form id="form1">
<input id="workAnswerId" name="workAnswerId" value="501"><input name="workRelationId" value="601"><input name="totalQuestionNum" value="3"><input name="fullScore" value="100"><input name="enc_work" value="token">
<div class="Py-mian1"><input id="answertype101" value="0"><div class="Py-m1-title"><span>1.</span><span>[单选题]</span><p>完整题目：请选择安全操作。</p></div><ul><li class="more-choose-item"><em class="choose-opt" id-param="D">A</em><div class="choose-desc"><cc>正确选项</cc></div></li><li class="more-choose-item"><em class="choose-opt" id-param="B">B</em><div class="choose-desc"><cc>错误选项</cc></div></li></ul></div>
<div class="Py-mian1"><input id="answertype102" value="1"><div class="Py-m1-title"><span>2.</span><span>[多选题]</span><p>选择两项。</p></div><ul><li class="more-choose-item"><em class="choose-opt" id-param="C">A</em><div class="choose-desc"><cc>内容一</cc></div></li><li class="more-choose-item"><em class="choose-opt" id-param="A">B</em><div class="choose-desc"><cc>内容二</cc></div></li></ul></div>
<div class="Py-mian1"><input id="answertype103" value="3"><div class="Py-m1-title"><span>3.</span><span>[判断题]</span><p>应遵守操作规程。</p></div></div></form></html>`

func TestQuizPreservesOptionKeysAndFullText(t *testing.T) {
	p, e := parseQuiz([]byte(quizFixture), Job{WorkID: "work1"})
	if e != nil {
		t.Fatal(e)
	}
	if len(p.Questions) != 3 || p.Questions[0].Text != "完整题目：请选择安全操作。" {
		t.Fatalf("题目解析不完整: %+v", p)
	}
	o := p.Questions[0].Options[0]
	if o.Key != "D" || o.Label != "A" || o.Text != "正确选项" {
		t.Fatalf("选项键和显示字母混淆: %+v", o)
	}
}
func TestQuizSubmissionAndReview(t *testing.T) {
	submitted := false
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/knowledge/cards":
			w.Write([]byte(`window.AttachmentSetting={"attachments":[{"jobid":"work-1","enc":"enc","property":{"workid":"work1"}}],"defaults":{"ktoken":"kt"}};`))
		case "/android/mworkspecial":
			if r.URL.Query().Get("ktoken") != "kt" || r.URL.Query().Get("enc") != "enc" {
				t.Error("未使用当前卡片令牌")
			}
			if submitted {
				w.Write([]byte(`<title>已批阅</title><div>本次得分: 100 已完成</div>`))
			} else {
				w.Write([]byte(quizFixture))
			}
		case "/work/addStudentWorkNew":
			r.ParseForm()
			if r.Method != "POST" || r.PostForm.Get("answer101") != "D" || r.PostForm.Get("answer102") != "CA" || r.PostForm.Get("answer103") != "false" || r.URL.Query().Get("cpi") != "3" || r.PostForm.Get("answerwqbid") != "101,102,103" {
				t.Errorf("错误提交表单: %v %v", r.PostForm, r.URL.Query())
			}
			submitted = true
			w.Write([]byte(`{"status":true,"msg":"success!"}`))
		default:
			http.NotFound(w, r)
		}
	})
	course := Course{ID: "1", ClassID: "2", CPI: "3"}
	job := Job{ID: "work-1", Type: "workid", WorkID: "work1", ChapterID: "4", Card: 1}
	p, e := c.Quiz(context.Background(), course, job)
	if e != nil {
		t.Fatal(e)
	}
	got, e := c.SubmitQuiz(context.Background(), course, p, map[string]any{"101": "D", "102": "AC", "103": false})
	if e != nil {
		t.Fatal(e)
	}
	if !submitted || got.(map[string]any)["verified"] != true {
		t.Fatalf("没有平台完成证据: %+v", got)
	}
}
func TestMissingAnswerDoesNotSubmit(t *testing.T) {
	requests := 0
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		t.Error("缺失答案不应发出提交请求")
	})
	p, e := parseQuiz([]byte(quizFixture), Job{})
	if e != nil {
		t.Fatal(e)
	}
	_, e = c.SubmitQuiz(context.Background(), Course{}, p, map[string]any{"101": "D"})
	if e == nil || requests != 0 {
		t.Fatal("缺失答案仍然提交")
	}
}
