package sync

import (
	"container/list"
	"sync"
)

// SyncQueue is a thread-safe queue for sync actions.
type SyncQueue struct {
	mu    sync.Mutex
	items *list.List
}

// NewSyncQueue creates a new sync queue.
func NewSyncQueue() *SyncQueue {
	return &SyncQueue{
		items: list.New(),
	}
}

// Push adds an item to the queue.
func (q *SyncQueue) Push(action SyncAction) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.items.PushBack(action)
}

// Pop removes and returns the first item from the queue.
func (q *SyncQueue) Pop() (SyncAction, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.items.Len() == 0 {
		return SyncAction{}, false
	}

	elem := q.items.Front()
	q.items.Remove(elem)
	return elem.Value.(SyncAction), true
}

// Len returns the number of items in the queue.
func (q *SyncQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.items.Len()
}

// Clear removes all items from the queue.
func (q *SyncQueue) Clear() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.items.Init()
}
