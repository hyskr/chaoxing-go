package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func testClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	c, err := newClient(filepath.Join(t.TempDir(), "session.txt"))
	if err != nil {
		t.Fatal(err)
	}
	target, _ := url.Parse(server.URL)
	c.HTTP.Transport = transportFunc(func(req *http.Request) (*http.Response, error) {
		copy := req.Clone(req.Context())
		u := *req.URL
		u.Scheme = target.Scheme
		u.Host = target.Host
		copy.URL = &u
		r, e := http.DefaultTransport.RoundTrip(copy)
		if r != nil {
			r.Request = req
		}
		return r, e
	})
	return c
}

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
func TestAssignedJSONHandlesScriptOrderAndQuotedBraces(t *testing.T) {
	source := `<script>var irrelevant=true;</script><script>window.AttachmentSetting = {"name":"} ; {","attachments":[]};</script>`
	j, e := assignedJSON(source, "AttachmentSetting")
	if e != nil || str(j["name"]) != "} ; {" {
		t.Fatalf("%v %+v", e, j)
	}
}
func TestExamImportPreservesCxStudyOrder(t *testing.T) {
	file := filepath.Join(t.TempDir(), "exam.json")
	os.WriteFile(file, []byte(`{"id":7,"title":"考试","questions":[{"id":9,"type":0,"value":"题干","options":{"C":"显示A","A":"显示B"},"answer":null},{"id":10,"type":2,"value":"填空","options":["第一空"],"answer":null}]}`), 0600)
	p, e := readPaper(file)
	if e != nil {
		t.Fatal(e)
	}
	if p.Questions[0].Options[0].Key != "C" || p.Questions[0].Options[0].Label != "A" {
		t.Fatal("丢失输入对象顺序")
	}
	p, e = fillPaper(p, map[string]any{"9": "C", "10": []any{"内容"}})
	if e != nil {
		t.Fatal(e)
	}
	target := filepath.Join(t.TempDir(), "filled.json")
	if e = writeJSON(target, p); e != nil {
		t.Fatal(e)
	}
	again, e := readPaper(target)
	if e != nil || again.Questions[0].Answer != "C" || again.Questions[1].Blanks != 1 {
		t.Fatalf("试卷往返读写失败: %v %+v", e, again)
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
	p, e := c.quiz(context.Background(), course, job)
	if e != nil {
		t.Fatal(e)
	}
	got, e := c.submitQuiz(context.Background(), course, p, map[string]any{"101": "D", "102": "AC", "103": false})
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
	_, e = c.submitQuiz(context.Background(), Course{}, p, map[string]any{"101": "D"})
	if e == nil || requests != 0 {
		t.Fatal("缺失答案仍然提交")
	}
}
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
	p, e := c.exportExam(context.Background(), "https://mooc1.chaoxing.com/exam-ans/exam/test/reVersionTestStartNew?courseId=1&classId=2&cpi=3&tId=4&id=5&enc=old&tag=1")
	if e != nil {
		t.Fatal(e)
	}
	if len(p.Questions) != 1 || p.Questions[0].Text != "完整题干" || p.Questions[0].Options[0].Key != "C" || p.Questions[0].Options[0].Label != "A" {
		t.Fatalf("导出内容不正确: %+v", p)
	}
}
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
	result, e := c.study(context.Background(), Course{ID: "1", ClassID: "2", CPI: "3"}, []Job{{ID: "doc1", Type: "document", ChapterID: "4"}}, 1, false, func(any) {})
	if e != nil || reports != 1 || len(result) != 1 || result[0].State != "completed" {
		t.Fatalf("未识别文档完成标记: %v %+v", e, result)
	}
}
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
	if _, e := c.login(context.Background(), "12345678901", "secret"); e != nil {
		t.Fatal(e)
	}
	data, e := os.ReadFile(c.Cookies)
	if e != nil || strings.Contains(string(data), "secret") || !strings.Contains(string(data), "_uid=42") {
		t.Fatalf("会话保存失败: %v", e)
	}
}
func TestCLIJSONAndConfirmContract(t *testing.T) {
	var out, events bytes.Buffer
	code := run(context.Background(), []string{"study", "run", "--course", "1"}, strings.NewReader(""), &out, &events)
	var envelope map[string]any
	if e := json.Unmarshal(out.Bytes(), &envelope); e != nil {
		t.Fatal(e)
	}
	if code != 5 || envelope["ok"] != false || object(envelope["error"])["code"] != "CONFIRM_REQUIRED" {
		t.Fatalf("不一致的退出码: %d %s", code, out.String())
	}
	out.Reset()
	code = run(context.Background(), []string{"schema", "quiz", "submit"}, strings.NewReader(""), &out, &events)
	if code != 0 || !strings.Contains(out.String(), "answers") {
		t.Fatal("schema 与命令脱节")
	}
}
