package promserver

import (
	"container/list"
	"context"
	"sync"
	"time"

	"github.com/jcodybaker/shellyctl/pkg/discovery"
)

const (
	defaultNotificationCacheTTL = 15 * time.Minute
)

type notificationCache struct {
	lock       sync.Mutex
	statuses   *list.List
	discoverer *discovery.Discoverer
	ttl        time.Duration
}

type timestampedStatus struct {
	localTS time.Time
	discovery.StatusNotification
}

func newNotificationCache(ttl time.Duration, d *discovery.Discoverer) *notificationCache {
	return &notificationCache{
		statuses:   list.New(),
		ttl:        ttl,
		discoverer: d,
	}
}

func (c *notificationCache) consumer(ctx context.Context) {
	fsnChan := c.discoverer.GetFullStatusNotifications(50)
	fsChan := c.discoverer.GetStatusNotifications(50)
	for {
		var notification timestampedStatus
		select {
		case <-ctx.Done():
			return
		case notification.StatusNotification = <-fsnChan:
		case notification.StatusNotification = <-fsChan:
		}
		now := time.Now()
		// The underlying statuses have a timestamp provided by the device. We'll report that for
		// the metric but some skew is expected, and there's non-trivial chance it's totally whacky wrong.
		// For that reason we capture the local timestamp here and base the cache-purge on this.
		notification.localTS = now
		c.lock.Lock()
		c.statuses.PushBack(notification)
		c.lockedPurge(now)
		c.lock.Unlock()
	}
}

func (c *notificationCache) lockedPurge(now time.Time) {
	expiry := now.Add(-1 * c.ttl)
	for e := c.statuses.Front(); e != nil; e = e.Next() {
		status := e.Value.(timestampedStatus)
		if status.localTS.Before(expiry) {
			c.statuses.Remove(e)
		} else {
			// The list is ordered by the monotonically increasing timestamp, so we only need to
			// inspect up to the first non-expired entry.
			break
		}
	}
}

func (c *notificationCache) getStatuses() (out []discovery.StatusNotification) {
	now := time.Now()
	c.lock.Lock()
	c.lockedPurge(now)
	for e := c.statuses.Front(); e != nil; e = e.Next() {
		status := e.Value.(timestampedStatus)
		out = append(out, status.StatusNotification)
	}
	c.lock.Unlock()
	return out
}
