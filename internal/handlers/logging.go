package handlers

import "github.com/HammerMeetNail/yearofbingo/internal/logging"

func logError(message string, err error, extraFields ...map[string]interface{}) {
	if err == nil {
		return
	}

	fields := map[string]interface{}{
		"error": err.Error(),
	}
	for _, extra := range extraFields {
		for k, v := range extra {
			fields[k] = v
		}
	}

	logging.Error(message, fields)
}
