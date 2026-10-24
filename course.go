package main

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"golang.org/x/net/html"
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
		return nil, failure("UPSTREAM_FORMAT", "接口未返回 JSON："+path, 6)
	}
	return j, nil
}
func (c *Client) courses(ctx context.Context) ([]Course, error) {
	j, err := c.getJSON(ctx, "/mycourse/backclazzdata", nil)
	if err != nil {
		return nil, err
	}
	if !truth(j["result"]) {
		return nil, failure("AUTH_REQUIRED", "无法读取课程，请重新登录", 3)
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
	all, err := c.courses(ctx)
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
		return Course{}, failure("COURSE_NOT_FOUND", "找不到课程 "+ref, 4)
	}
	if len(found) > 1 {
		return Course{}, failure("AMBIGUOUS_COURSE", "该课程存在多个班级，请使用 courseID:classID", 2)
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
		return nil, failure("UPSTREAM_FORMAT", "章节列表为空或无权限", 6)
	}
	cd := array(object(object(data[0])["course"])["data"])
	if len(cd) == 0 {
		return nil, failure("UPSTREAM_FORMAT", "课程章节数据缺失", 6)
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
		return nil, failure("UPSTREAM_FORMAT", "任务点状态不是 JSON", 6)
	}
	for i := range result {
		m := object(progress[result[i].ID])
		if len(m) == 0 {
			return nil, failure("UPSTREAM_FORMAT", "缺少章节任务点状态", 6)
		}
		result[i].Total = integer(m["totalcount"])
		if result[i].Total == 0 {
			result[i].Total = integer(m["unfinishcount"])
		}
		result[i].Finished = integer(m["finishcount"])
	}
	return result, nil
}
func signParams(q url.Values) url.Values {
	sum := md5.Sum([]byte(q.Encode() + "&DESKey=Z(AfY@XS"))
	q.Set("inf_enc", hex.EncodeToString(sum[:]))
	return q
}
func (c *Client) attachment(ctx context.Context, course Course, chapter string, card int) (map[string]any, error) {
	b, err := c.request(ctx, "GET", apiHost+"/knowledge/cards", url.Values{"clazzid": {course.ClassID}, "courseid": {course.ID}, "knowledgeid": {chapter}, "num": {strconv.Itoa(card)}, "isPhone": {"1"}, "control": {"true"}, "cpi": {course.CPI}}, nil)
	if err != nil {
		return nil, err
	}
	return assignedJSON(string(b), "AttachmentSetting")
}
func (c *Client) jobs(ctx context.Context, course Course, chapters []Chapter) ([]Job, error) {
	out := []Job{}
	for _, ch := range chapters {
		if ch.Total == 0 {
			continue
		}
		j, err := c.getJSON(ctx, "/gas/knowledge", signParams(url.Values{"id": {ch.ID}, "courseid": {course.ID}, "fields": {"id,card.fields(id,title,description,cardorder).contentcard(all)"}, "view": {"json"}, "token": {"4faa8662c59590c6f43ae9fe5b002b42"}, "_time": {strconv.FormatInt(time.Now().UnixMilli(), 10)}}))
		if err != nil {
			return nil, err
		}
		data := array(j["data"])
		if len(data) == 0 {
			return nil, failure("UPSTREAM_FORMAT", "无法读取章节卡片 "+ch.ID, 6)
		}
		cards := array(object(object(data[0])["card"])["data"])
		for index, v := range cards {
			card := object(v)
			doc, _ := document([]byte(str(card["description"])))
			frames := findAll(doc, func(n *html.Node) bool { return n.Data == "iframe" && attr(n, "module") != "" })
			if len(frames) == 0 {
				continue
			}
			att, err := c.attachment(ctx, course, ch.ID, index)
			if err != nil {
				return nil, err
			}
			defaults := object(att["defaults"])
			for _, v := range array(att["attachments"]) {
				m := object(v)
				prop := object(m["property"])
				id := str(m["jobid"])
				if id == "" {
					continue
				}
				typ := strings.ToLower(first(m["type"], prop["type"], prop["module"]))
				if typ == "work" {
					typ = "workid"
				}
				out = append(out, Job{ID: id, Type: typ, Name: first(prop["name"], prop["title"], m["name"]), ChapterID: ch.ID, Card: index, Passed: truth(m["isPassed"]) || ch.Finished >= ch.Total || typ == "document" && !truth(m["job"]), ObjectID: first(m["objectId"], prop["objectid"]), WorkID: str(prop["workid"]), SchoolID: str(prop["schoolid"]), PlayTime: integer(m["playTime"]), raw: m, defaults: defaults})
			}
		}
	}
	return out, nil
}
func (c *Client) refreshJob(ctx context.Context, course Course, j Job) (Job, error) {
	att, err := c.attachment(ctx, course, j.ChapterID, j.Card)
	if err != nil {
		return j, err
	}
	for _, v := range array(att["attachments"]) {
		m := object(v)
		if str(m["jobid"]) == j.ID {
			j.raw = m
			j.defaults = object(att["defaults"])
			j.Passed = truth(m["isPassed"]) || j.Type == "document" && !truth(m["job"])
			j.PlayTime = integer(m["playTime"])
			return j, nil
		}
	}
	return j, failure("JOB_NOT_FOUND", "任务已不在当前卡片中", 4)
}
func chooseJob(jobs []Job, id string) (Job, error) {
	for _, j := range jobs {
		if j.ID == id || j.WorkID == id {
			return j, nil
		}
	}
	return Job{}, failure("JOB_NOT_FOUND", "找不到任务 "+id, 4)
}
func (c *Client) loadJobs(ctx context.Context, ref string) (Course, []Chapter, []Job, error) {
	course, err := c.resolveCourse(ctx, ref)
	if err != nil {
		return course, nil, nil, err
	}
	chs, err := c.chapters(ctx, course)
	if err != nil {
		return course, nil, nil, err
	}
	jobs, err := c.jobs(ctx, course, chs)
	return course, chs, jobs, err
}
