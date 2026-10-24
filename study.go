package main

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type StudyResult struct {
	JobID string `json:"job_id"`
	Name  string `json:"name"`
	State string `json:"state"`
}

func wait(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
func videoSignature(course Course, j Job, uid string, playing, duration int) string {
	value := fmt.Sprintf("[%s][%s][%s][%s][%d][d_yHJ!$pdA~5][%d][0_%d]", course.ClassID, uid, j.ID, j.ObjectID, playing*1000, duration*1000, duration)
	sum := md5.Sum([]byte(value))
	return hex.EncodeToString(sum[:])
}
func (c *Client) videoReport(ctx context.Context, course Course, j Job, token string, duration, playing int) (bool, error) {
	uid := first(c.cookie("_uid"), c.cookie("UID"))
	rt := first(object(j.raw["property"])["rt"], j.raw["rt"], "0.9")
	other := first(j.raw["otherInfo"], j.raw["otherinfo"])
	q := url.Values{"otherInfo": {other}, "playingTime": {strconv.Itoa(playing)}, "duration": {strconv.Itoa(duration)}, "jobid": {j.ID}, "clipTime": {"0_" + strconv.Itoa(duration)}, "clazzId": {course.ClassID}, "objectId": {j.ObjectID}, "userid": {uid}, "isdrag": {"0"}, "enc": {videoSignature(course, j, uid, playing, duration)}, "rt": {rt}, "dtype": {"Video"}, "view": {"pc"}, "_t": {strconv.FormatInt(time.Now().UnixMilli(), 10)}}
	if j.Type == "audio" {
		q.Set("dtype", "Audio")
	}
	encoded := strings.ReplaceAll(strings.ReplaceAll(q.Encode(), "%26", "&"), "%3D", "=")
	b, err := c.request(ctx, "GET", apiHost+"/multimedia/log/a/"+course.CPI+"/"+token+"?"+encoded, nil, nil)
	if err != nil {
		return false, err
	}
	var response map[string]any
	if decodeJSON(b, &response) != nil {
		return false, failure("UPSTREAM_FORMAT", "视频进度响应不是 JSON", 6)
	}
	if str(response["error"]) != "" {
		return false, failure("REPORT_REJECTED", str(response["error"]), 6)
	}
	return truth(response["isPassed"]), nil
}
func (c *Client) learnVideo(ctx context.Context, course Course, j Job, speed float64, emit func(any)) error {
	info, err := c.getJSON(ctx, "/ananas/status/"+j.ObjectID, url.Values{"k": {first(j.defaults["fid"], c.cookie("fid"))}, "flag": {"normal"}, "_dc": {strconv.FormatInt(time.Now().UnixMilli(), 10)}})
	if err != nil {
		return err
	}
	if str(info["status"]) != "success" {
		return failure("VIDEO_UNAVAILABLE", "视频资源暂不可用", 6)
	}
	duration := integer(info["duration"])
	if duration <= 0 {
		return failure("UPSTREAM_FORMAT", "视频时长缺失", 6)
	}
	token := str(info["dtoken"])
	if token == "" {
		return failure("UPSTREAM_FORMAT", "视频缺少 dtoken", 6)
	}
	start := j.PlayTime / 1000
	if start > duration {
		start = duration
	}
	began := time.Now()
	endReports := 0
	for {
		playing := min(duration, start+int(time.Since(began).Seconds()*speed))
		passed, e := c.videoReport(ctx, course, j, token, duration, playing)
		if e != nil {
			return e
		}
		emit(map[string]any{"event": "video.progress", "job_id": j.ID, "seconds": playing, "duration": duration, "passed": passed})
		if passed {
			return nil
		}
		if playing >= duration {
			endReports++
			if endReports >= 3 {
				return failure("COMPLETION_UNCONFIRMED", "播放时长已达到，但平台未确认完成："+j.ID, 7)
			}
		}
		pause := time.Duration(math.Min(58/speed, math.Max(1, float64(duration-playing)/speed)) * float64(time.Second))
		if err = wait(ctx, pause); err != nil {
			return err
		}
	}
}
func (c *Client) study(ctx context.Context, course Course, jobs []Job, speed float64, dry bool, emit func(any)) ([]StudyResult, error) {
	result := []StudyResult{}
	for _, job := range jobs {
		if job.Type != "video" && job.Type != "audio" && job.Type != "document" {
			continue
		}
		state := "planned"
		if job.Passed {
			state = "already_completed"
		}
		if dry || job.Passed {
			result = append(result, StudyResult{job.ID, job.Name, state})
			continue
		}
		j, err := c.refreshJob(ctx, course, job)
		if err != nil {
			return result, err
		}
		if j.Passed {
			result = append(result, StudyResult{j.ID, j.Name, "already_completed"})
			continue
		}
		if j.Type == "document" {
			b, e := c.request(ctx, "GET", "https://mooc1.chaoxing.com/ananas/job/document", url.Values{"jobid": {j.ID}, "knowledgeid": {j.ChapterID}, "courseid": {course.ID}, "clazzid": {course.ClassID}, "jtoken": {str(j.raw["jtoken"])}, "_dc": {strconv.FormatInt(time.Now().UnixMilli(), 10)}}, nil)
			if e != nil {
				return result, e
			}
			var ack map[string]any
			if decodeJSON(b, &ack) != nil {
				return result, failure("UPSTREAM_FORMAT", "文档进度响应不是 JSON", 6)
			}
			if str(ack["error"]) != "" {
				return result, failure("REPORT_REJECTED", str(ack["error"]), 6)
			}
			checked, e := c.refreshJob(ctx, course, j)
			if e != nil {
				return result, e
			}
			if !checked.Passed {
				return result, failure("COMPLETION_UNCONFIRMED", "文档进度未确认完成", 7)
			}
		} else if err = c.learnVideo(ctx, course, j, speed, emit); err != nil {
			return result, err
		}
		result = append(result, StudyResult{j.ID, j.Name, "completed"})
		emit(map[string]any{"event": "study.completed", "job_id": j.ID})
	}
	return result, nil
}
