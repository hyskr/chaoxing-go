package chaoxing

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

func document(b []byte) (*html.Node, error) { return html.Parse(bytes.NewReader(b)) }
func attr(n *html.Node, key string) string {
	if n != nil {
		for _, a := range n.Attr {
			if a.Key == key {
				return a.Val
			}
		}
	}
	return ""
}
func hasClass(n *html.Node, c string) bool {
	for _, v := range strings.Fields(attr(n, "class")) {
		if v == c {
			return true
		}
	}
	return false
}
func findAll(n *html.Node, p func(*html.Node) bool) []*html.Node {
	var result []*html.Node
	var walk func(*html.Node)
	walk = func(x *html.Node) {
		if x == nil {
			return
		}
		if x.Type == html.ElementNode && p(x) {
			result = append(result, x)
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return result
}
func find(n *html.Node, p func(*html.Node) bool) *html.Node {
	a := findAll(n, p)
	if len(a) > 0 {
		return a[0]
	}
	return nil
}
func tag(n *html.Node, t string) *html.Node {
	return find(n, func(x *html.Node) bool { return x.Data == t })
}
func cls(n *html.Node, c string) *html.Node {
	return find(n, func(x *html.Node) bool { return hasClass(x, c) })
}
func byID(n *html.Node, id string) *html.Node {
	return find(n, func(x *html.Node) bool { return attr(x, "id") == id })
}
func textContent(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(x *html.Node) {
		if x == nil {
			return
		}
		if x.Type == html.TextNode {
			b.WriteString(x.Data)
		}
		if x.Type == html.ElementNode {
			switch x.Data {
			case "script", "style":
				return
			case "img":
				b.WriteString(" ![" + attr(x, "alt") + "](" + attr(x, "src") + ") ")
				return
			case "br", "p", "div":
				b.WriteByte('\n')
			}
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.TrimSpace(b.String())
}
func compact(n *html.Node) string { return strings.Join(strings.Fields(textContent(n)), " ") }
func render(n *html.Node) string {
	if n == nil {
		return ""
	}
	var b bytes.Buffer
	_ = html.Render(&b, n)
	return b.String()
}
func inputs(n *html.Node) map[string][]string {
	v := map[string][]string{}
	for _, x := range findAll(n, func(x *html.Node) bool { return x.Data == "input" || x.Data == "textarea" }) {
		name := attr(x, "name")
		if name == "" {
			continue
		}
		if t := attr(x, "type"); t == "radio" || t == "checkbox" || t == "button" || t == "submit" {
			continue
		}
		s := attr(x, "value")
		if x.Data == "textarea" {
			s = textContent(x)
		}
		v[name] = append(v[name], s)
	}
	return v
}

// 解码 JS 赋值中的 JSON 对象，忽略字符串内的括号与分号。
func assignedJSON(source, name string) (map[string]any, error) {
	re := regexp.MustCompile(`(?:window\.)?` + regexp.QuoteMeta(name) + `\s*=\s*\{`)
	loc := re.FindStringIndex(source)
	if loc == nil {
		return nil, Failure("UPSTREAM_FORMAT", "页面缺少 "+name, 6)
	}
	start := loc[1] - 1
	depth := 0
	quoted := false
	escaped := false
	for i := start; i < len(source); i++ {
		c := source[i]
		if quoted {
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				quoted = false
			}
			continue
		}
		if c == '"' {
			quoted = true
		} else if c == '{' {
			depth++
		} else if c == '}' {
			depth--
			if depth == 0 {
				var m map[string]any
				if err := json.Unmarshal([]byte(source[start:i+1]), &m); err != nil {
					return nil, Failure("UPSTREAM_FORMAT", "任务数据不是有效 JSON", 6)
				}
				return m, nil
			}
		}
	}
	return nil, Failure("UPSTREAM_FORMAT", "任务 JSON 未闭合", 6)
}
