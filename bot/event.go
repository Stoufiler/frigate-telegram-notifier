package main

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
)

// reviewItem is the subset of a Frigate review we actually use. Unknown JSON
// fields are ignored, so the bot tolerates schema additions (e.g. Frigate 0.18).
type reviewItem struct {
	ID        string   `json:"id"`
	Camera    string   `json:"camera"`
	StartTime float64  `json:"start_time"`
	EndTime   *float64 `json:"end_time"`
	Severity  string   `json:"severity"`
	Data      struct {
		Objects    []string `json:"objects"`
		Detections []string `json:"detections"`
		Zones      []string `json:"zones"`
	} `json:"data"`
}

// Review is a message published on the frigate/reviews MQTT topic.
type Review struct {
	Type  string     `json:"type"` // "new" | "update" | "end"
	After reviewItem `json:"after"`
}

// dispatch parses an MQTT payload and routes it to the right handler.
func (a *App) dispatch(ctx context.Context, payload []byte) {
	var r Review
	if err := json.Unmarshal(payload, &r); err != nil {
		logf(tr("json_error", map[string]any{"Error": err}))
		return
	}
	switch r.Type {
	case "new":
		a.handleNew(ctx, r.After)
	case "end":
		a.handleEnd(ctx, r.After)
	}
}

// allowed reports whether an event passes the camera and object filters.
func (a *App) allowed(item reviewItem) bool {
	if len(a.cfg.AllowedCameras) > 0 && !a.cfg.AllowedCameras[item.Camera] {
		return false
	}
	if len(a.cfg.ObjectFilter) > 0 {
		for _, obj := range item.Data.Objects {
			if a.cfg.ObjectFilter[obj] {
				return true
			}
		}
		return false
	}
	return true
}

// handleNew sends a snapshot when an event starts.
func (a *App) handleNew(ctx context.Context, item reviewItem) {
	if !a.allowed(item) {
		return
	}
	logf(tr("movement_detected", map[string]any{"CameraName": item.Camera}))

	if len(item.Data.Detections) == 0 {
		logf(tr("no_detection_id"))
		return
	}
	detectionID := item.Data.Detections[0]
	logf(tr("detection_id_extracted", map[string]any{"DetectionID": detectionID}))

	dest := filepath.Join(os.TempDir(), detectionID+".jpg")
	defer func() { _ = os.Remove(dest) }()
	if err := a.frigate.Snapshot(ctx, detectionID, dest); err != nil {
		logf(tr("snapshot_download_failed", map[string]any{"Error": err}))
		return
	}

	caption := a.caption("caption_snapshot_simple", "caption_snapshot_object", "caption_snapshot_zone", item, nil)
	if err := a.notifier.SendPhoto(dest, caption); err != nil {
		logf(tr("telegram_send_photo_error", map[string]any{"Error": err}))
		return
	}
	logf(tr("snapshot_sent", map[string]any{"CameraName": item.Camera}))
}

// handleEnd sends the review preview (GIF) when an event ends.
func (a *App) handleEnd(ctx context.Context, item reviewItem) {
	if !a.allowed(item) {
		return
	}
	logf(tr("end_of_event", map[string]any{"CameraName": item.Camera}))
	logf(tr("review_id_extracted", map[string]any{"ReviewID": item.ID}))

	dest := filepath.Join(os.TempDir(), item.ID+".gif")
	defer func() { _ = os.Remove(dest) }()
	if err := a.frigate.Preview(ctx, item.ID, dest); err != nil {
		logf(tr("preview_download_failed", map[string]any{"Error": err}))
		return
	}

	duration := tr("unknown_duration")
	if item.EndTime != nil {
		duration = tr("duration_seconds", map[string]any{"Seconds": fmt.Sprintf("%.0f", *item.EndTime-item.StartTime)})
	}
	caption := a.caption("caption_preview_simple", "caption_preview_object", "caption_preview_zone", item, map[string]any{"Duration": duration})
	if err := a.notifier.SendAnimation(dest, caption); err != nil {
		logf(tr("telegram_send_animation_error", map[string]any{"Error": err}))
		return
	}
	logf(tr("preview_sent", map[string]any{"CameraName": item.Camera}))
}

// caption builds a localized caption: a base line (with objects when known)
// plus an optional zone suffix. extra carries handler-specific fields (Duration).
func (a *App) caption(simpleID, objectID, zoneID string, item reviewItem, extra map[string]any) string {
	data := map[string]any{"CameraName": item.Camera}
	maps.Copy(data, extra)

	msgID := simpleID
	if len(item.Data.Objects) > 0 {
		translated := make([]string, 0, len(item.Data.Objects))
		for _, obj := range item.Data.Objects {
			translated = append(translated, translateLabel(obj))
		}
		data["Objects"] = titleCaser.String(strings.Join(translated, ", "))
		msgID = objectID
	}

	caption := tr(msgID, data)
	if len(item.Data.Zones) > 0 {
		caption += tr(zoneID, map[string]any{"Zones": strings.Join(item.Data.Zones, ", ")})
	}
	return caption
}
