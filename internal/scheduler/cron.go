package scheduler

import (
	"fmt"
	"time"
)

// GetCronExpression generates a cron expression for the scheduler.
func GetCronExpression(t time.Time) string {
	return fmt.Sprintf("%d %d * * *", t.Minute(), t.Hour())
}
