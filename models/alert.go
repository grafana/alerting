package models

import "github.com/prometheus/common/model"

type TestReceiversConfigAlertParams struct {
	Annotations model.LabelSet `yaml:"annotations,omitempty" json:"annotations,omitempty"`
	Labels      model.LabelSet `yaml:"labels,omitempty" json:"labels,omitempty"`
	// Status is the alert status to simulate. Valid values are "firing" (default) and "resolved".
	Status model.AlertStatus `yaml:"status,omitempty" json:"status,omitempty"`
}
