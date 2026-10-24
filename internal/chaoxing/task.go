// 协议来源：CxStudy cxapi/chapters.py: fetch_points_by_index；cxapi/base.py: fetch_attachment；cxapi/utils.py: inf_enc_sign。见 UPSTREAM.md。
package chaoxing

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

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
			return nil, Failure("UPSTREAM_FORMAT", "无法读取章节卡片 "+ch.ID, 6)
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
	return j, Failure("JOB_NOT_FOUND", "任务已不在当前卡片中", 4)
}
func ChooseJob(jobs []Job, id string) (Job, error) {
	for _, j := range jobs {
		if j.ID == id || j.WorkID == id {
			return j, nil
		}
	}
	return Job{}, Failure("JOB_NOT_FOUND", "找不到任务 "+id, 4)
}
func (c *Client) LoadJobs(ctx context.Context, ref string) (Course, []Chapter, []Job, error) {
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
