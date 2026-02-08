package handlers

import "github.com/HammerMeetNail/yearofbingo/internal/logging"

func logError(message string, err error, extraFields ...map[string]interface{}) {
	if err == nil {
		return
	}

	fields := map[string]interface{}{}
	for _, extra := range extraFields {
		for k, v := range extra {
			if k == "error" {
				continue
			}
			fields[k] = v
		}
	}
	fields["error"] = err.Error()

	logging.Error(message, fields)
}
