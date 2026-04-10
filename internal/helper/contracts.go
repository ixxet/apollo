package helper

import "errors"

var (
	ErrUnsupportedWhyTopic  = errors.New("helper why topic is invalid")
	ErrUnsupportedVariation = errors.New("helper variation is invalid")
)

type PreviewMode string

const PreviewModeReadOnly PreviewMode = "read_only"

type Summary struct {
	Headline    string   `json:"headline"`
	Detail      string   `json:"detail"`
	Bullets     []string `json:"bullets"`
	Limitations []string `json:"limitations"`
}

type Option struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}
