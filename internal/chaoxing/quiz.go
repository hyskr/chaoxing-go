// 协议来源：CxStudy cxapi/task_point/work.py: parse_question、construct_questions_form、PointWorkDto。见 UPSTREAM.md。
package chaoxing

import (
	"context"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

func QuestionType(n string) string {
	switch n {
	case "0":
		return "single"
	case "1":
		return "multiple"
	case "2":
		return "completion"
	case "3":
		return "judgement"
	case "4":
		return "shortanswer"
	}
	return "unknown"
}
func typeNumber(s string) string {
	switch s {
	case "single":
		return "0"
	case "multiple":
		return "1"
	case "completion":
		return "2"
	case "judgement":
		return "3"
	case "shortanswer":
		return "4"
	}
	return ""
}
func parseQuiz(b []byte, j Job) (Paper, error) {
	p := Paper{ID: j.WorkID, Kind: "quiz", AnswerEncoding: "option_key", Questions: []Question{}, job: j}
	doc, err := document(b)
	if err != nil {
		return p, err
	}
	if tip := cls(doc, "blankTips"); tip != nil {
		return p, Failure("WORK_UNAVAILABLE", compact(tip), 5)
	}
	title := tag(doc, "title")
	p.Completed = strings.Contains(compact(title), "已批阅")
	p.Title = compact(cls(doc, "chapter-title"))
	if p.Title == "" {
		p.Title = compact(cls(doc, "py-Title"))
	}
	if p.Title == "" {
		p.Title = j.Name
	}
	if p.Completed {
		m := regexp.MustCompile(`本次得分[:：]?\s*([0-9.]+)`).FindStringSubmatch(compact(doc))
		if len(m) > 1 {
			p.Score = m[1]
		}
		return p, nil
	}
	form := byID(doc, "form1")
	if form == nil {
		return p, Failure("UPSTREAM_FORMAT", "没有找到章节测验表单", 6)
	}
	p.form = inputs(form)
	for _, node := range findAll(form, func(n *html.Node) bool { return hasClass(n, "Py-mian1") }) {
		input := find(node, func(n *html.Node) bool { return n.Data == "input" && strings.HasPrefix(attr(n, "id"), "answertype") })
		if input == nil {
			return p, Failure("UPSTREAM_FORMAT", "题目缺少题型与ID", 6)
		}
		id := strings.TrimPrefix(attr(input, "id"), "answertype")
		qt := QuestionType(attr(input, "value"))
		stem := cls(node, "Py-m1-title")
		q := Question{ID: id, Type: qt, HTML: render(stem), Options: []Option{}}
		// 题号和题型是题干容器的前两个文本片段，保留后续完整正文。
		var pieces []string
		var walk func(*html.Node)
		walk = func(n *html.Node) {
			if n == nil {
				return
			}
			if n.Type == html.TextNode {
				pieces = append(pieces, n.Data)
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
		}
		walk(stem)
		if len(pieces) >= 2 {
			q.Text = strings.TrimSpace(strings.Join(pieces[2:], ""))
		} else {
			q.Text = compact(stem)
		}
		if strings.Contains(render(stem), "<img") {
			q.Text = compact(stem)
		}
		for _, o := range findAll(node, func(n *html.Node) bool { return n.Data == "li" && hasClass(n, "more-choose-item") }) {
			key := attr(cls(o, "choose-opt"), "id-param")
			q.Options = append(q.Options, Option{Key: key, Label: compact(cls(o, "choose-opt")), Text: compact(tag(cls(o, "choose-desc"), "cc"))})
		}
		for _, o := range findAll(node, func(n *html.Node) bool { return n.Data == "input" && hasClass(n, "blankInp2") }) {
			_ = o
			q.Blanks++
		}
		if q.ID == "" || q.Text == "" {
			return p, Failure("UPSTREAM_FORMAT", "题目解析不完整", 6)
		}
		p.Questions = append(p.Questions, q)
	}
	if len(p.Questions) == 0 {
		return p, Failure("UPSTREAM_FORMAT", "表单中没有解析到题目", 6)
	}
	return p, nil
}
func (c *Client) Quiz(ctx context.Context, course Course, j Job) (Paper, error) {
	j, err := c.refreshJob(ctx, course, j)
	if err != nil {
		return Paper{}, err
	}
	workID := j.WorkID
	if j.SchoolID != "" {
		workID = j.SchoolID + "-" + workID
	}
	b, err := c.request(ctx, "GET", apiHost+"/android/mworkspecial", url.Values{"courseid": {course.ID}, "workid": {workID}, "jobid": {j.ID}, "needRedirect": {"true"}, "knowledgeid": {j.ChapterID}, "userid": {first(c.cookie("_uid"), c.cookie("UID"))}, "ut": {"s"}, "clazzId": {course.ClassID}, "cpi": {course.CPI}, "ktoken": {str(j.defaults["ktoken"])}, "enc": {str(j.raw["enc"])}}, nil)
	if err != nil {
		return Paper{}, err
	}
	return parseQuiz(b, j)
}
func (c *Client) SubmitQuiz(ctx context.Context, course Course, p Paper, answers map[string]any) (any, error) {
	if p.Completed {
		return nil, Failure("ALREADY_COMPLETED", "此测验已经提交并批阅", 5)
	}
	form := url.Values(p.form)
	ids := []string{}
	for _, q := range p.Questions {
		if q.Type == "shortanswer" || q.Type == "unknown" {
			return nil, Failure("UNSUPPORTED_QUESTION", "当前测验提交不支持题型 "+q.Type, 5)
		}
		answer, err := NormalizeAnswer(q, answers[q.ID])
		if err != nil {
			return nil, err
		}
		ids = append(ids, q.ID)
		form.Set("answertype"+q.ID, typeNumber(q.Type))
		switch v := answer.(type) {
		case bool:
			form.Set("answer"+q.ID, strconv.FormatBool(v))
		case string:
			form.Set("answer"+q.ID, v)
		case []any:
			form.Set("tiankongsize"+q.ID, strconv.Itoa(len(v)))
			for i, a := range v {
				form.Set("answer"+q.ID+strconv.Itoa(i+1), str(a))
			}
		}
	}
	form.Set("answerwqbid", strings.Join(ids, ","))
	form.Set("pyFlag", "")
	form.Set("courseId", course.ID)
	form.Set("classId", course.ClassID)
	form.Set("api", "1")
	form.Set("mooc", "0")
	form.Set("isphone", "true")
	form.Set("userId", first(c.cookie("_uid"), c.cookie("UID")))
	form.Set("oldWorkId", p.ID)
	form.Set("jobid", p.job.ID)
	form.Set("knowledgeid", p.job.ChapterID)
	query := url.Values{"keyboardDisplayRequiresUserAction": {"1"}, "_classId": {course.ClassID}, "courseid": {course.ID}, "token": {form.Get("enc_work")}, "workAnswerId": {form.Get("workAnswerId")}, "workid": {form.Get("workRelationId")}, "cpi": {course.CPI}, "jobid": {p.job.ID}, "knowledgeid": {p.job.ChapterID}, "ua": {"app"}}
	b, err := c.request(ctx, "POST", apiHost+"/work/addStudentWorkNew", query, form)
	if err != nil {
		return nil, err
	}
	var response map[string]any
	if decodeJSON(b, &response) != nil {
		return nil, Failure("SUBMISSION_UNCONFIRMED", "未取得提交结果，请在网页确认后再重试", 7)
	}
	if !truth(response["status"]) {
		return nil, Failure("SUBMIT_REJECTED", first(response["msg"], "平台拒绝提交"), 6)
	}
	review, err := c.Quiz(ctx, course, p.job)
	if err != nil {
		return map[string]any{"submitted": true, "verified": false, "work_id": p.ID}, Failure("REVIEW_UNAVAILABLE", "提交成功，但暂未读取到批阅结果", 7)
	}
	return map[string]any{"submitted": true, "verified": review.Completed, "paper": review}, nil
}
