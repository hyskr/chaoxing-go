package main

import (
	"context"
	"golang.org/x/net/html"
	"net/url"
	"strings"
)

func parseExamHTML(b []byte, id string) (Paper, error) {
	p := Paper{ID: id, Kind: "exam", AnswerEncoding: "option_key", Questions: []Question{}}
	doc, err := document(b)
	if err != nil {
		return p, err
	}
	if tip := cls(doc, "blankTips"); tip != nil {
		return p, failure("EXAM_UNAVAILABLE", compact(tip), 5)
	}
	if tip := cls(doc, "msg"); tip != nil && byID(doc, "submitTest") == nil {
		return p, failure("EXAM_UNAVAILABLE", compact(tip), 5)
	}
	p.Title = compact(cls(doc, "examTit"))
	if p.Title == "" {
		p.Title = compact(tag(doc, "h1"))
	}
	if p.Title == "" {
		p.Title = "考试 " + id
	}
	nodes := findAll(doc, func(n *html.Node) bool {
		return hasClass(n, "questionWrap") && hasClass(n, "singleQuesId") && hasClass(n, "ans-cc-exam")
	})
	for _, node := range nodes {
		qid := attr(find(node, func(n *html.Node) bool { return n.Data == "input" && attr(n, "name") == "questionId" }), "value")
		typ := attr(find(node, func(n *html.Node) bool { return n.Data == "input" && attr(n, "name") == "type"+qid }), "value")
		stem := cls(node, "tit")
		q := Question{ID: qid, Type: questionType(typ), Options: []Option{}, HTML: render(stem)}
		var pieces []string
		for ch := stemFirst(stem); ch != nil; ch = ch.NextSibling {
			if ch.Type == html.ElementNode && ch.Data == "h3" {
				continue
			}
			s := ""
			if ch.Type == html.TextNode {
				s = ch.Data
			} else {
				s = textContent(ch)
			}
			pieces = append(pieces, s)
		}
		q.Text = strings.TrimSpace(strings.Join(pieces, ""))
		if dot := strings.Index(q.Text, "."); dot >= 0 {
			prefix := strings.TrimSpace(q.Text[:dot])
			digits := prefix != ""
			for _, r := range prefix {
				if r < '0' || r > '9' {
					digits = false
				}
			}
			if digits {
				q.Text = strings.TrimSpace(q.Text[dot+1:])
			}
		}
		for _, o := range findAll(node, func(n *html.Node) bool { return hasClass(n, "answerList") && hasClass(n, "radioList") }) {
			q.Options = append(q.Options, Option{Key: attr(o, "name"), Label: compact(cls(o, "No")), Text: compact(tag(o, "cc"))})
		}
		input := find(node, func(n *html.Node) bool { return n.Data == "input" && attr(n, "id") == "answer"+qid })
		a := attr(input, "value")
		if q.Type == "judgement" {
			if a == "true" {
				q.Answer = true
			} else if a == "false" {
				q.Answer = false
			}
		} else if a != "" {
			q.Answer = a
		}
		if q.Type == "completion" {
			blanks := findAll(node, func(n *html.Node) bool { return n.Data == "textarea" && hasClass(n, "blanktextarea") })
			q.Blanks = len(blanks)
			values := []any{}
			for _, x := range blanks {
				values = append(values, textContent(x))
			}
			q.Answer = values
		}
		if qid == "" || q.Text == "" {
			return p, failure("UPSTREAM_FORMAT", "考试题目解析不完整", 6)
		}
		p.Questions = append(p.Questions, q)
	}
	if len(p.Questions) == 0 {
		return p, failure("EXAM_NOT_READY", "当前页面没有整卷题目，请先在学习通进入考试并取得当前试卷URL", 5)
	}
	return p, nil
}
func stemFirst(n *html.Node) *html.Node {
	if n == nil {
		return nil
	}
	return n.FirstChild
}
func (c *Client) exportExam(ctx context.Context, entry string) (Paper, error) {
	u, err := url.Parse(entry)
	if err != nil || u.Scheme != "https" || !strings.HasSuffix(u.Hostname(), ".chaoxing.com") {
		return Paper{}, failure("INVALID_ARGUMENT", "--url 需要已进入试卷的超星HTTPS地址", 2)
	}
	q := u.Query()
	examID := first(q.Get("tId"), q.Get("examRelationId"))
	answerID := first(q.Get("id"), q.Get("examRelationAnswerId"))
	course := q.Get("courseId")
	class := q.Get("classId")
	cpi := q.Get("cpi")
	if examID == "" || answerID == "" || course == "" || class == "" || cpi == "" {
		return Paper{}, failure("INVALID_ARGUMENT", "试卷URL缺少考试ID、作答ID、课程、班级或cpi", 2)
	}
	if !strings.Contains(u.Path, "reVersionTestStartNew") && !strings.Contains(u.Path, "/preview") {
		return Paper{}, failure("INVALID_ARGUMENT", "请传入答题页或整卷预览URL", 2)
	}
	// 沿用已启动的答题会话，首次请求只读取当前题目及有效时间参数。
	if q.Has("tag") {
		q.Set("tag", "0")
	}
	u.RawQuery = q.Encode()
	b, err := c.request(ctx, "GET", u.String(), nil, nil)
	if err != nil {
		return Paper{}, err
	}
	doc, err := document(b)
	if err != nil {
		return Paper{}, err
	}
	if tip := cls(doc, "blankTips"); tip != nil {
		return Paper{}, failure("EXAM_UNAVAILABLE", compact(tip), 5)
	}
	form := byID(doc, "submitTest")
	if form == nil {
		return Paper{}, failure("EXAM_NOT_READY", "尚未进入试卷，或会话需要重新验证", 5)
	}
	val := func(key string) string { return attr(byID(form, key), "value") }
	if val("enc") == "" || val("encRemainTime") == "" || val("encLastUpdateTime") == "" {
		return Paper{}, failure("UPSTREAM_FORMAT", "试卷会话参数缺失", 6)
	}
	query := url.Values{"courseId": {course}, "classId": {class}, "source": {"0"}, "start": {"0"}, "imei": {c.DeviceID}, "cpi": {cpi}, "examRelationId": {examID}, "examRelationAnswerId": {answerID}, "monitorStatus": {"0"}, "monitorOp": {"-1"}, "remainTimeParam": {val("encRemainTime")}, "relationAnswerLastUpdateTime": {val("encLastUpdateTime")}, "enc": {val("enc")}}
	b, err = c.request(ctx, "GET", apiHost+"/exam-ans/exam/phone/preview", query, nil)
	if err != nil {
		return Paper{}, err
	}
	return parseExamHTML(b, examID)
}
