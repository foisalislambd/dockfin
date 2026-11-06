package git

import (
	"encoding/json"
)

// NormalizeGitLabSystemHook gives a GitLab system-hook body the project-webhook
// shape. Current GitLab already sends project data. Older documented push
// payloads only have event_name and a top-level project_id.
func NormalizeGitLabSystemHook(body []byte) ([]byte, error) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if _, ok := payload["object_kind"]; !ok {
		if name, ok := payload["event_name"].(string); ok && name != "" {
			payload["object_kind"] = name
		}
	}
	project, _ := payload["project"].(map[string]any)
	if project == nil {
		project = map[string]any{}
	}
	if _, ok := project["id"]; !ok {
		if id, ok := payload["project_id"]; ok {
			project["id"] = id
		}
	}
	if path, _ := project["path_with_namespace"].(string); path == "" {
		if p, _ := project["path"].(string); p != "" && project["namespace"] != nil {
			project["path_with_namespace"] = p
		}
	}
	payload["project"] = project
	return json.Marshal(payload)
}
