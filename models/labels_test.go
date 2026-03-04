package models

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFilterAlertmanagerLabel(t *testing.T) {
	cases := []struct {
		name  string
		label string
		value string
		want  bool
	}{
		{
			name:  "empty label name",
			label: "",
			value: "v",
			want:  true,
		},
		{
			name:  "empty label value",
			label: "alertname",
			value: "",
			want:  true,
		},
		{
			name:  "NamespaceUIDLabel",
			label: NamespaceUIDLabel,
			value: "some-uid",
			want:  true,
		},
		{
			name:  "valid label",
			label: "severity",
			value: "critical",
			want:  false,
		},
		{
			name:  "both empty",
			label: "",
			value: "",
			want:  true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := FilterAlertmanagerLabel(tc.label, tc.value)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestFilterAlertmanagerAnnotation(t *testing.T) {
	cases := []struct {
		name       string
		annotation string
		value      string
		want       bool
	}{
		{
			name:       "empty annotation name",
			annotation: "",
			value:      "v",
			want:       true,
		},
		{
			name:       "empty annotation value",
			annotation: "summary",
			value:      "",
			want:       true,
		},
		{
			name:       "valid annotation",
			annotation: "summary",
			value:      "alert fired",
			want:       false,
		},
		{
			name:       "NamespaceUIDLabel is not filtered for annotations",
			annotation: NamespaceUIDLabel,
			value:      "some-uid",
			want:       false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := FilterAlertmanagerAnnotation(tc.annotation, tc.value)
			require.Equal(t, tc.want, got)
		})
	}
}
