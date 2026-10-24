package chaoxing

import (
	"testing"
)

func TestAssignedJSONHandlesScriptOrderAndQuotedBraces(t *testing.T) {
	source := `<script>var irrelevant=true;</script><script>window.AttachmentSetting = {"name":"} ; {","attachments":[]};</script>`
	j, e := assignedJSON(source, "AttachmentSetting")
	if e != nil || str(j["name"]) != "} ; {" {
		t.Fatalf("%v %+v", e, j)
	}
}
