package api

import (
	"context"
	"time"

	"github.com/google/uuid"
	"k8s.io/client-go/kubernetes"

	cddb "github.com/OA879/K8Sense/backend/pkg/clusterdoctor/db"
	"github.com/OA879/K8Sense/backend/pkg/logger"
)

// schedulerTick is how often the scheduler wakes to look for due scans. The
// per-schedule interval floor is 5 minutes, so a 1-minute tick is plenty
// granular while staying cheap.
const schedulerTick = time.Minute

// ScheduledClientFunc resolves a cluster to a clientset without an inbound
// HTTP request. Scheduled scans have no user request to derive a token from,
// so setup supplies a kubeconfig-backed factory instead.
type ScheduledClientFunc func(clusterName string) (kubernetes.Interface, error)

// StartScheduler runs the recurring-scan loop until ctx is cancelled. It is
// started once at setup; if no schedules are enabled it costs one cheap query
// per minute and nothing else.
func (s *Server) StartScheduler(ctx context.Context, getClient ScheduledClientFunc) {
	go func() {
		ticker := time.NewTicker(schedulerTick)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.runDueScans(ctx, getClient)
			}
		}
	}()
}

// runDueScans launches a scan for every schedule whose interval has elapsed.
// Each cluster is stamped before the scan starts so a long-running scan can't
// be double-triggered by the next tick.
func (s *Server) runDueScans(ctx context.Context, getClient ScheduledClientFunc) {
	due, err := cddb.DueSchedules(ctx, s.db, time.Now().Unix())
	if err != nil {
		logger.Log(logger.LevelError, nil, err, "cluster-doctor: querying due scan schedules")
		return
	}

	for _, sched := range due {
		clientset, err := getClient(sched.ClusterID)
		if err != nil {
			logger.Log(logger.LevelError, map[string]string{"cluster": sched.ClusterID}, err,
				"cluster-doctor: scheduled scan could not resolve cluster")

			continue
		}

		if err := cddb.MarkScheduleRun(ctx, s.db, sched.ClusterID, time.Now().Unix()); err != nil {
			logger.Log(logger.LevelError, map[string]string{"cluster": sched.ClusterID}, err,
				"cluster-doctor: stamping schedule run")
		}

		scanID := uuid.NewString()
		live := newLiveScan()
		s.registerActive(scanID, live)

		logger.Log(logger.LevelInfo, map[string]string{"cluster": sched.ClusterID, "scanId": scanID}, nil,
			"cluster-doctor: starting scheduled scan")

		go s.runScan(clientset, sched.ClusterID, scanID, live)
	}
}
