package auth

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
)

// Registered legacy callbacks promise synchronous cleanup for empty 200/204
// acknowledgements. A response body must explicitly acknowledge a terminal
// outcome; no envelope may fall back to the legacy acknowledgement semantics.
func deletionResponseOutcome(status int, raw []byte) (string, string) {
	if status == http.StatusNoContent {
		return "confirmed_cleanup", ""
	}
	if status >= 300 && status < 400 {
		return "unresolved", "callback_redirect"
	}
	if status != http.StatusOK {
		return "pending", "callback_not_completed"
	}
	if len(raw) == 0 {
		return "confirmed_cleanup", ""
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return "pending", "invalid_response"
	}
	fields := map[string]json.RawMessage{}
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return "pending", "invalid_response"
		}
		key, ok := keyToken.(string)
		if !ok || (key != "version" && key != "outcome") || fields[key] != nil {
			return "pending", "invalid_response"
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return "pending", "invalid_response"
		}
		fields[key] = value
	}
	if end, err := decoder.Token(); err != nil || end != json.Delim('}') {
		return "pending", "invalid_response"
	}
	if decoder.Decode(new(any)) != io.EOF {
		return "pending", "invalid_response"
	}
	if version, exists := fields["version"]; exists && !bytes.Equal(bytes.TrimSpace(version), []byte("1")) {
		return "pending", "invalid_response"
	}
	var outcome string
	if err := json.Unmarshal(fields["outcome"], &outcome); err != nil || (outcome != "confirmed_cleanup" && outcome != "no_current_mapping") {
		return "pending", "invalid_response"
	}
	return outcome, ""
}

func readDeletionResponse(response *http.Response) (string, string) {
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 32*1024+1))
	if err != nil || len(raw) > 32*1024 {
		return "pending", "invalid_response"
	}
	return deletionResponseOutcome(response.StatusCode, raw)
}
