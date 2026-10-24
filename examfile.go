package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// CxStudy 的对象键是提交用的内部键；对象中的出现顺序才对应显示字母。
func readOptions(raw json.RawMessage) ([]Option, error) {
	result := []Option{}
	s := bytes.TrimSpace(raw)
	if len(s) == 0 || string(s) == "null" {
		return result, nil
	}
	if s[0] == '[' {
		var blanks []string
		if err := json.Unmarshal(s, &blanks); err == nil {
			for i, label := range blanks {
				result = append(result, Option{Key: strconv.Itoa(i + 1), Label: label, Text: ""})
			}
			return result, nil
		}
		if err := json.Unmarshal(s, &result); err != nil {
			return nil, err
		}
		return result, nil
	}
	d := json.NewDecoder(bytes.NewReader(s))
	tok, err := d.Token()
	if err != nil || tok != json.Delim('{') {
		return nil, fmt.Errorf("options 必须是对象或选项数组")
	}
	for d.More() {
		key, err := d.Token()
		if err != nil {
			return nil, err
		}
		var value string
		if err = d.Decode(&value); err != nil {
			return nil, err
		}
		label := string(rune('A' + len(result)))
		result = append(result, Option{fmt.Sprint(key), label, value})
	}
	_, err = d.Token()
	return result, err
}
func readPaper(path string) (Paper, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Paper{}, err
	}
	var raw struct {
		ID        json.RawMessage   `json:"id"`
		Title     string            `json:"title"`
		Kind      string            `json:"kind"`
		Questions []json.RawMessage `json:"questions"`
	}
	if err = json.Unmarshal(b, &raw); err != nil {
		return Paper{}, failure("INVALID_PAPER", "试卷 JSON 解析失败", 2)
	}
	var id any
	_ = decodeJSON(raw.ID, &id)
	p := Paper{ID: str(id), Title: raw.Title, Kind: raw.Kind, AnswerEncoding: "option_key", Questions: []Question{}}
	if p.Kind == "" {
		p.Kind = "exam"
	}
	if len(raw.Questions) == 0 {
		return p, failure("INVALID_PAPER", "试卷没有 questions", 2)
	}
	seen := map[string]bool{}
	for _, r := range raw.Questions {
		var row struct {
			ID      any             `json:"id"`
			Value   string          `json:"value"`
			Text    string          `json:"text"`
			Type    any             `json:"type"`
			Options json.RawMessage `json:"options"`
			Answer  any             `json:"answer"`
			Blanks  int             `json:"blanks"`
		}
		if err = decodeJSON(r, &row); err != nil {
			return p, err
		}
		typ := str(row.Type)
		if _, err := strconv.Atoi(typ); err == nil {
			typ = questionType(typ)
		}
		options, e := readOptions(row.Options)
		if e != nil {
			return p, failure("INVALID_PAPER", e.Error(), 2)
		}
		q := Question{ID: str(row.ID), Type: typ, Text: first(row.Text, row.Value), Options: options, Answer: row.Answer, Blanks: row.Blanks}
		if typ == "completion" && q.Blanks == 0 {
			q.Blanks = len(options)
		}
		if q.ID == "" || seen[q.ID] || q.Text == "" {
			return p, failure("INVALID_PAPER", "题目ID为空、重复或缺少题干", 2)
		}
		seen[q.ID] = true
		p.Questions = append(p.Questions, q)
	}
	return p, nil
}
func readAnswers(path string) (map[string]any, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw map[string]any
	if err = decodeJSON(b, &raw); err != nil {
		return nil, failure("INVALID_ANSWERS", "答案 JSON 解析失败", 2)
	}
	if qs, ok := raw["questions"]; ok {
		out := map[string]any{}
		for _, v := range array(qs) {
			q := object(v)
			id := str(q["id"])
			if id == "" {
				return nil, failure("INVALID_ANSWERS", "答案题目缺少ID", 2)
			}
			out[id] = q["answer"]
		}
		return out, nil
	}
	return raw, nil
}
func fillPaper(p Paper, answers map[string]any) (Paper, error) {
	for i, q := range p.Questions {
		value, ok := answers[q.ID]
		if !ok {
			value = q.Answer
		}
		a, err := normalizeAnswer(q, value)
		if err != nil {
			return p, err
		}
		p.Questions[i].Answer = a
	}
	return p, nil
}
func markdown(p Paper) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n选项显示字母用于网页查看；提交使用括号中的 key。\n\n", p.Title)
	for i, q := range p.Questions {
		fmt.Fprintf(&b, "## %d. %s\n\n%s\n\n", i+1, q.Type, q.Text)
		for _, o := range q.Options {
			fmt.Fprintf(&b, "- %s [key=%s] %s\n", o.Label, o.Key, o.Text)
		}
		if q.Answer != nil {
			a, _ := json.Marshal(q.Answer)
			fmt.Fprintf(&b, "\n答案（内部键）：%s\n", a)
		}
		b.WriteByte('\n')
	}
	return b.String()
}
func writeJSON(path string, value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0600)
}
