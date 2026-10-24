package chaoxing

import (
	"strings"
)

func NormalizeAnswer(q Question, value any) (any, error) {
	if value == nil {
		return nil, Failure("MISSING_ANSWER", "题目 "+q.ID+" 缺少答案", 2)
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
		return nil, Failure("INVALID_ANSWER", "判断题 "+q.ID+" 需要 true/false", 2)
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
			return nil, Failure("INVALID_ANSWER", "题目 "+q.ID+" 的选项数量不正确", 2)
		}
		valid := map[string]bool{}
		for _, o := range q.Options {
			valid[o.Key] = true
		}
		seen := map[string]bool{}
		for _, k := range keys {
			if !valid[k] || seen[k] {
				return nil, Failure("INVALID_ANSWER", "题目 "+q.ID+" 包含无效或重复选项键 "+k, 2)
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
			return nil, Failure("INVALID_ANSWER", "填空题 "+q.ID+" 需要与空数一致的字符串数组", 2)
		}
		for _, v := range xs {
			if _, ok := v.(string); !ok {
				return nil, Failure("INVALID_ANSWER", "填空答案必须是字符串", 2)
			}
		}
		return xs, nil
	case "shortanswer":
		s, ok := value.(string)
		if !ok || strings.TrimSpace(s) == "" {
			return nil, Failure("INVALID_ANSWER", "简答题需要非空字符串", 2)
		}
		return s, nil
	}
	return nil, Failure("UNSUPPORTED_QUESTION", "不支持题型 "+q.Type, 5)
}
