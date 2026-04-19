package crawler

import (
	"encoding/json"
	"log"

	"helios-backend/internal/db"
)

func recordMetric(jobID, nodeID int64, name string, value float64, unit string, tags map[string]any) {
	var tagJSON any
	if tags != nil {
		if b, err := json.Marshal(tags); err == nil {
			tagJSON = string(b)
		}
	}
	if _, err := db.DB.Exec(
		`INSERT INTO crawl_metrics (job_id, node_id, metric_name, metric_value, unit, tags)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		nullInt64(jobID), nullInt64(nodeID), name, value, nullStr(unit), tagJSON,
	); err != nil {
		log.Printf("crawler metric %s: %v", name, err)
	}
}

func writeLog(jobID, nodeID int64, level, msg string, ctx map[string]any) {
	var ctxJSON any
	if ctx != nil {
		if b, err := json.Marshal(ctx); err == nil {
			ctxJSON = string(b)
		}
	}
	if _, err := db.DB.Exec(
		`INSERT INTO crawl_logs (job_id, node_id, level, message, context)
		 VALUES (?, ?, ?, ?, ?)`,
		jobID, nullInt64(nodeID), level, msg, ctxJSON,
	); err != nil {
		log.Printf("crawler log: %v", err)
	}
}

func nullInt64(v int64) any {
	if v <= 0 {
		return nil
	}
	return v
}

func nullStr(v string) any {
	if v == "" {
		return nil
	}
	return v
}
