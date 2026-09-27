package domain

import "strings"

const (
	ProcessorFFmpeg     = "ffmpeg"
	ProcessorGStreamer  = "gstreamer"
)

func ParseProcessor(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", ProcessorFFmpeg:
		return ProcessorFFmpeg, nil
	case ProcessorGStreamer, "gst":
		return ProcessorGStreamer, nil
	default:
		return "", ErrUnknownProcessor
	}
}
