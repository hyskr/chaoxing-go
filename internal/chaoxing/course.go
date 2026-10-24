// 协议来源：CxStudy cxapi/api.py: fetch_classes；cxapi/classes.py: get_chapters_by_index；cxapi/chapters.py: fetch_point_status。见 UPSTREAM.md。
package chaoxing

import (
	"context"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const apiHost = "https://mooc1-api.chaoxing.com"

func (c *Client) getJSON(ctx context.Context, path string, q url.Values) (map[string]any, error) {
	b, err := c.request(ctx, "GET", apiHost+path, q, nil)
	if err != nil {
		return nil, err
	}
	var j map[string]any
	if decodeJSON(b, &j) != nil {
		return nil, Failure("UPSTREAM_FORMAT", "接口未返回 JSON："+path, 6)
	}
	return j, nil
}
func (c *Client) Courses(ctx context.Context) ([]Course, error) {
	j, err := c.getJSON(ctx, "/mycourse/backclazzdata", nil)
	if err != nil {
		return nil, err
	}
	if !truth(j["result"]) {
		return nil, Failure("AUTH_REQUIRED", "无法读取课程，请重新登录", 3)
	}
	out := []Course{}
	for _, row := range array(j["channelList"]) {
		r := object(row)
		content := object(r["content"])
		data := array(object(content["course"])["data"])
		if len(data) == 0 {
			continue
		}
		course := object(data[0])
		status := "active"
		if integer(content["state"]) != 0 {
			status = "ended"
		}
		out = append(out, Course{str(course["id"]), str(content["id"]), str(r["cpi"]), str(course["name"]), status})
	}
	return out, nil
}
func (c *Client) resolveCourse(ctx context.Context, ref string) (Course, error) {
	all, err := c.Courses(ctx)
	if err != nil {
		return Course{}, err
	}
	parts := strings.Split(ref, ":")
	var found []Course
	for _, x := range all {
		if x.ID == parts[0] && (len(parts) == 1 || x.ClassID == parts[1]) {
			found = append(found, x)
		}
	}
	if len(found) == 0 {
		return Course{}, Failure("COURSE_NOT_FOUND", "找不到课程 "+ref, 4)
	}
	if len(found) > 1 {
		return Course{}, Failure("AMBIGUOUS_COURSE", "该课程存在多个班级，请使用 courseID:classID", 2)
	}
	return found[0], nil
}
func (c *Client) chapters(ctx context.Context, course Course) ([]Chapter, error) {
	j, err := c.getJSON(ctx, "/gas/clazz", url.Values{"id": {course.ClassID}, "personid": {course.CPI}, "fields": {"id,course.fields(id,knowledge.fields(id,name,label,layer))"}, "view": {"json"}})
	if err != nil {
		return nil, err
	}
	data := array(j["data"])
	if len(data) == 0 {
		return nil, Failure("UPSTREAM_FORMAT", "章节列表为空或无权限", 6)
	}
	cd := array(object(object(data[0])["course"])["data"])
	if len(cd) == 0 {
		return nil, Failure("UPSTREAM_FORMAT", "课程章节数据缺失", 6)
	}
	result := []Chapter{}
	for _, v := range array(object(object(cd[0])["knowledge"])["data"]) {
		m := object(v)
		result = append(result, Chapter{ID: str(m["id"]), Label: str(m["label"]), Name: strings.TrimSpace(str(m["name"]))})
	}
	sort.SliceStable(result, func(i, j int) bool {
		a, b := strings.Split(result[i].Label, "."), strings.Split(result[j].Label, ".")
		for k := 0; k < len(a) && k < len(b); k++ {
			x, _ := strconv.Atoi(a[k])
			y, _ := strconv.Atoi(b[k])
			if x != y {
				return x < y
			}
		}
		return len(a) < len(b)
	})
	nodes := []string{}
	for _, x := range result {
		nodes = append(nodes, x.ID)
	}
	b, err := c.request(ctx, "POST", apiHost+"/job/myjobsnodesmap", nil, url.Values{"view": {"json"}, "nodes": {strings.Join(nodes, ",")}, "clazzid": {course.ClassID}, "time": {strconv.FormatInt(time.Now().UnixMilli(), 10)}, "userid": {first(c.cookie("_uid"), c.cookie("UID"))}, "cpi": {course.CPI}, "courseid": {course.ID}})
	if err != nil {
		return nil, err
	}
	var progress map[string]any
	if decodeJSON(b, &progress) != nil {
		return nil, Failure("UPSTREAM_FORMAT", "任务点状态不是 JSON", 6)
	}
	for i := range result {
		m := object(progress[result[i].ID])
		if len(m) == 0 {
			return nil, Failure("UPSTREAM_FORMAT", "缺少章节任务点状态", 6)
		}
		result[i].Total = integer(m["totalcount"])
		if result[i].Total == 0 {
			result[i].Total = integer(m["unfinishcount"])
		}
		result[i].Finished = integer(m["finishcount"])
	}
	return result, nil
}
