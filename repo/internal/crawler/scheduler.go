package crawler

import (
	"log"
	"time"

	"helios-backend/internal/db"
)

const (
	schedulerTick  = 5 * time.Second
	nodeStaleAfter = 60 * time.Second
)

// StartScheduler runs the elastic scheduler loop. Responsibilities:
//   - mark nodes whose heartbeat has aged out as offline
//   - reassign orphaned jobs (node offline) back to the queue
//   - assign unassigned queued jobs to online nodes (round-robin by load)
//   - re-queue 'failed' jobs whose retry timer has elapsed (safety net; the
//     worker writes retries itself)
func StartScheduler() {
	go func() {
		t := time.NewTicker(schedulerTick)
		defer t.Stop()
		tick()
		for range t.C {
			tick()
		}
	}()
	log.Println("crawler scheduler started (tick=5s)")
}

func tick() {
	if err := markOfflineNodes(); err != nil {
		log.Printf("scheduler mark offline: %v", err)
	}
	if err := reassignOrphaned(); err != nil {
		log.Printf("scheduler reassign: %v", err)
	}
	if err := assignQueued(); err != nil {
		log.Printf("scheduler assign: %v", err)
	}
	if err := requeueRetries(); err != nil {
		log.Printf("scheduler requeue: %v", err)
	}
}

func markOfflineNodes() error {
	_, err := db.DB.Exec(
		`UPDATE crawl_nodes
		 SET status='offline'
		 WHERE status='online'
		   AND (last_heartbeat_at IS NULL OR last_heartbeat_at < DATE_SUB(NOW(), INTERVAL ? SECOND))`,
		int(nodeStaleAfter.Seconds()),
	)
	return err
}

func reassignOrphaned() error {
	// Jobs whose assigned node has gone offline while running: unassign so
	// another worker can pick them up and resume from checkpoint.
	_, err := db.DB.Exec(`
		UPDATE crawl_jobs j
		JOIN crawl_nodes n ON n.id = j.node_id
		SET j.node_id = NULL, j.status = 'queued'
		WHERE j.status = 'running' AND n.status = 'offline'`)
	return err
}

func assignQueued() error {
	rows, err := db.DB.Query(
		`SELECT id FROM crawl_nodes WHERE status='online' ORDER BY id`)
	if err != nil {
		return err
	}
	var nodeIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err == nil {
			nodeIDs = append(nodeIDs, id)
		}
	}
	rows.Close()
	if len(nodeIDs) == 0 {
		return nil
	}

	// Pull up to N unassigned queued jobs that are ready to run.
	jobRows, err := db.DB.Query(`
		SELECT id FROM crawl_jobs
		WHERE status='queued' AND node_id IS NULL
		  AND (scheduled_at    IS NULL OR scheduled_at    <= NOW())
		  AND (next_attempt_at IS NULL OR next_attempt_at <= NOW())
		ORDER BY priority DESC, id ASC
		LIMIT 50`)
	if err != nil {
		return err
	}
	var jobIDs []int64
	for jobRows.Next() {
		var id int64
		if err := jobRows.Scan(&id); err == nil {
			jobIDs = append(jobIDs, id)
		}
	}
	jobRows.Close()

	// Round-robin assignment
	for i, jobID := range jobIDs {
		node := nodeIDs[i%len(nodeIDs)]
		if _, err := db.DB.Exec(
			`UPDATE crawl_jobs SET node_id = ? WHERE id = ? AND node_id IS NULL`,
			node, jobID,
		); err != nil {
			log.Printf("assign job %d -> node %d: %v", jobID, node, err)
		}
	}
	return nil
}

func requeueRetries() error {
	// The worker usually sets status='queued' itself when scheduling a retry;
	// this is a safety net for any rows that ended up as 'failed' but still
	// have attempts remaining and a due timer.
	_, err := db.DB.Exec(`
		UPDATE crawl_jobs
		SET status = 'queued', node_id = NULL
		WHERE status = 'failed'
		  AND attempts < max_attempts
		  AND next_attempt_at IS NOT NULL
		  AND next_attempt_at <= NOW()`)
	return err
}
