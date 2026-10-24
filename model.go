package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type AppError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Exit    int    `json:"-"`
}

func (e *AppError) Error() string                  { return e.Message }
func failure(code, message string, exit int) error { return &AppError{code, message, exit} }

type Course struct {
	ID      string `json:"id"`
	ClassID string `json:"class_id"`
	CPI     string `json:"cpi"`
	Name    string `json:"name"`
	Status  string `json:"status"`
}
type Chapter struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Name     string `json:"name"`
	Total    int    `json:"total"`
	Finished int    `json:"finished"`
}
type Job struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Name      string `json:"name"`
	ChapterID string `json:"chapter_id"`
	Card      int    `json:"card"`
	Passed    bool   `json:"passed"`
	ObjectID  string `json:"object_id,omitempty"`
	WorkID    string `json:"work_id,omitempty"`
	SchoolID  string `json:"school_id,omitempty"`
	PlayTime  int    `json:"play_time_ms,omitempty"`
	raw       map[string]any
	defaults  map[string]any
}
type Option struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Text  string `json:"text"`
}
type Question struct {
	ID      string   `json:"id"`
	Type    string   `json:"type"`
	Text    string   `json:"text"`
	Options []Option `json:"options"`
	Answer  any      `json:"answer"`
	HTML    string   `json:"html,omitempty"`
	Blanks  int      `json:"blanks,omitempty"`
}
type Paper struct {
	ID             string     `json:"id"`
	Title          string     `json:"title"`
	Kind           string     `json:"kind"`
	Completed      bool       `json:"completed"`
	Score          string     `json:"score,omitempty"`
	AnswerEncoding string     `json:"answer_encoding"`
	Questions      []Question `json:"questions"`
	form           map[string][]string
	job            Job
}

func str(x any) string {
	if x == nil {
		return ""
	}
	return fmt.Sprint(x)
}
func integer(x any) int { n, _ := strconv.Atoi(str(x)); return n }
func object(x any) map[string]any {
	m, _ := x.(map[string]any)
	if m == nil {
		return map[string]any{}
	}
	return m
}
func array(x any) []any { a, _ := x.([]any); return a }
func truth(x any) bool  { return x == true || str(x) == "1" || str(x) == "true" }
func first(xs ...any) string {
	for _, x := range xs {
		if s := str(x); s != "" {
			return s
		}
	}
	return ""
}
func normalizeAnswer(q Question, value any) (any, error) {
	if value == nil {
		return nil, failure("MISSING_ANSWER", "题目 "+q.ID+" 缺少答案", 2)
	}
	switch q.Type {
	case "judgement":
		if b, ok := value.(bool); ok {
			return b, nil
		}
		switch strings.ToLower(strings.TrimSpace(str(value))) {
		case "true", "对", "正确", "是", "1":
			return true, nil
		case "false", "错", "错误", "否", "0":
			return false, nil
		}
		return nil, failure("INVALID_ANSWER", "判断题 "+q.ID+" 需要 true/false", 2)
	case "single", "multiple":
		var keys []string
		if xs, ok := value.([]any); ok {
			for _, v := range xs {
				keys = append(keys, str(v))
			}
		} else {
			for _, r := range strings.ToUpper(str(value)) {
				if r != ',' && r != ' ' {
					keys = append(keys, string(r))
				}
			}
		}
		if len(keys) == 0 || q.Type == "single" && len(keys) != 1 {
			return nil, failure("INVALID_ANSWER", "题目 "+q.ID+" 的选项数量不正确", 2)
		}
		valid := map[string]bool{}
		for _, o := range q.Options {
			valid[o.Key] = true
		}
		seen := map[string]bool{}
		for _, k := range keys {
			if !valid[k] || seen[k] {
				return nil, failure("INVALID_ANSWER", "题目 "+q.ID+" 包含无效或重复选项键 "+k, 2)
			}
			seen[k] = true
		}
		// 按页面选项顺序构造内部键，显示字母只用于展示。
		result := ""
		for _, o := range q.Options {
			if seen[o.Key] {
				result += o.Key
			}
		}
		return result, nil
	case "completion":
		xs, ok := value.([]any)
		if !ok || len(xs) != q.Blanks {
			return nil, failure("INVALID_ANSWER", "填空题 "+q.ID+" 需要与空数一致的字符串数组", 2)
		}
		for _, v := range xs {
			if _, ok := v.(string); !ok {
				return nil, failure("INVALID_ANSWER", "填空答案必须是字符串", 2)
			}
		}
		return xs, nil
	case "shortanswer":
		s, ok := value.(string)
		if !ok || strings.TrimSpace(s) == "" {
			return nil, failure("INVALID_ANSWER", "简答题需要非空字符串", 2)
		}
		return s, nil
	}
	return nil, failure("UNSUPPORTED_QUESTION", "不支持题型 "+q.Type, 5)
}
func decodeJSON(b []byte, v any) error {
	d := json.NewDecoder(strings.NewReader(string(b)))
	d.UseNumber()
	return d.Decode(v)
}
