package main

import (
	"bytes"
	"encoding/json"
	"os/exec"
)

type videoAspectRatio struct {
	Streams []struct {
		Width              int    `json:"width"`
		Height             int    `json:"height"`
		DisplayAspectRatio string `json:"display_aspect_ratio"`
	} `json:"streams"`
}

func getVideoAspectRatio(filePath string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)

	var b bytes.Buffer
	cmd.Stdout = &b
	cmd.Run()

	aspectRatio := videoAspectRatio{}
	if err := json.Unmarshal(b.Bytes(), &aspectRatio); err != nil {
		return "", err
	}

	width := aspectRatio.Streams[0].Width
	height := aspectRatio.Streams[0].Height
	if height > width && (9*height/width > 14) {
		return "portrait", nil
	} else if width > height && (9*width/height > 14) {
		return "landscape", nil
	}
	return "other", nil
}
