package main

import (
	"encoding/json"

	"github.com/google/uuid"
)

type eventGridEvent struct {
	Subject     string          `json:"subject"`
	Id          string          `json:"id"`
	EventType   string          `json:"eventType"`
	Data        json.RawMessage `json:"data"`
	DataVersion string          `json:"dataVersion"`
}

func newEventGridEvent(subject string, eventType string, dataVersion string, data interface{}) (*eventGridEvent, error) {
	j, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	return &eventGridEvent{
		Id:          uuid.New().String(),
		Subject:     subject,
		EventType:   eventType,
		DataVersion: dataVersion,
		Data:        j,
	}, nil
}
