/*
Copyright 2019 The Tekton Authors
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
		http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package broadcaster

import (
	"errors"
	"sync"
)

type messageType string

// Reference outside of package
const (
	Log                     messageType = "Log"
	NamespaceCreated        messageType = "NamespaceCreated"
	NamespaceDeleted        messageType = "NamespaceDeleted"
	PipelineCreated         messageType = "PipelineCreated"
	PipelineDeleted         messageType = "PipelineDeleted"
	PipelineUpdated         messageType = "PipelineUpdated"
	TaskCreated             messageType = "TaskCreated"
	TaskDeleted             messageType = "TaskDeleted"
	TaskUpdated             messageType = "TaskUpdated"
	PipelineResourceCreated messageType = "PipelineResourceCreated"
	PipelineResourceDeleted messageType = "PipelineResourceDeleted"
	PipelineResourceUpdated messageType = "PipelineResourceUpdated"
	PipelineRunCreated      messageType = "PipelineRunCreated"
	PipelineRunDeleted      messageType = "PipelineRunDeleted"
	PipelineRunUpdated      messageType = "PipelineRunUpdated"
	TaskRunCreated          messageType = "TaskRunCreated"
	TaskRunDeleted          messageType = "TaskRunDeleted"
	TaskRunUpdated          messageType = "TaskRunUpdated"
)

type workQueue struct {
	sync.Mutex
	head *socketDataNode
	tail *socketDataNode
}

type SocketData struct {
	MessageType messageType
	Payload     interface{}
}

type socketDataNode struct {
	SocketData
	next *socketDataNode
}

// Only a pointer to the struct should be used
type Broadcaster struct {
	expired bool
	// Explicit name to specify locking condition
	expiredLock sync.Mutex
	subscribers *sync.Map //map[*Subscriber]struct{}
	c           chan SocketData
}

// Only a pointer to the struct should be used
type Subscriber struct {
	workQueue workQueue
	// Handles each item enqueued
	// true = work, false = unsubscribe
	process func(s SocketData) bool
	// Signal subscriber to read when populating empty queue
	// true = work, false = expired
	work chan bool
}

var expiredError error = errors.New("Broadcaster expired")

var processed int

// Creates broadcaster from channel parameter and immediately starts broadcasting
// Without any subscribers, received data will be discarded
// Broadcaster should be the only channel reader
func NewBroadcaster(c chan SocketData) *Broadcaster {
	if c == nil {
		panic("Channel passed cannot be nil")
	}

	b := &Broadcaster{subscribers: new(sync.Map)}
	b.c = c
	go func() {
		for {
			msg, channelOpen := <- b.c
			if channelOpen {
				b.subscribers.Range(func(key, value interface{}) bool {
					subscriber := key.(*Subscriber)
					subscriber.workQueue.Lock()
					newQueueNode := &socketDataNode{
						SocketData: msg,
					}
					if subscriber.workQueue.head == nil {
						subscriber.workQueue.head, subscriber.workQueue.tail = newQueueNode, newQueueNode
						// Signal the subscriber to work
						subscriber.work <- true
					} else {
						subscriber.workQueue.tail.next = newQueueNode
						subscriber.workQueue.tail = newQueueNode
					}
					subscriber.workQueue.Unlock()
					return true
				})
			} else {
				b.expiredLock.Lock()
				b.expired = true
				// Kill all subscribers
				b.subscribers.Range(func(key, value interface{}) bool {
					subscriber := key.(*Subscriber)
					close(subscriber.work)
					return true
				})
				// Remove references
				b.subscribers = nil
				b.expiredLock.Unlock()
				return
			}
		}
	}()
	return b
}

func (b *Broadcaster) Expired() bool {
	b.expiredLock.Lock()
	defer b.expiredLock.Unlock()
	return b.expired
}

// Passed function will be invoked for each item in Subscriber work queue
// The work queue is populated by data ingested by the broadcaster channel
// Subscriber return can be used to unsubscribe
func (b *Broadcaster) Subscribe(processFunc func(s SocketData)bool) (*Subscriber, error) {
	b.expiredLock.Lock()
	defer b.expiredLock.Unlock()

	if b.expired {
		return &Subscriber{}, expiredError
	}
	if processFunc == nil {
		return &Subscriber{}, errors.New("Nil function not allowed")
	}
	newSub := &Subscriber{
		process: processFunc,
		work: make(chan bool, 1),
	}
	// Generate unique key
	b.subscribers.Store(newSub, struct{}{})
	go func() {
		for {
			// Wait until signal is received to work
			work := <- newSub.work
			if !work {
				return
			}
			// Process the workQueue until empty
			var endOfQueue bool
			for {
				// Obtain work
				var workItem SocketData
				newSub.workQueue.Lock()
				workItem = newSub.workQueue.head.SocketData
				newSub.workQueue.head = newSub.workQueue.head.next // Reset head
				if newSub.workQueue.head == nil {
					endOfQueue = true
				}
				newSub.workQueue.Unlock()
				// Process
				success := newSub.process(workItem)
				if !success {
					b.Unsubscribe(newSub)
					return
				}
				if endOfQueue {
					break
				}
			}
		}
	}()
	return newSub, nil
}

func (b *Broadcaster) Unsubscribe(sub *Subscriber) error {
	b.expiredLock.Lock()
	defer b.expiredLock.Unlock()

	if b.expired {
		return expiredError
	}
	if _, ok := b.subscribers.Load(sub); ok {
		b.subscribers.Delete(sub)
		// Signal end of work
		sub.work <- false
		return nil
	}
	return errors.New("Subscription not found")
}

// Iterates over sync.Map and returns number of elements
// Response can be oversized if counted subscriptions are cancelled while counting
func (b *Broadcaster) PoolSize() (size int) {
	b.expiredLock.Lock()
	defer b.expiredLock.Unlock()

	if b.expired {
		return 0
	}
	b.subscribers.Range(func(key, value interface{}) bool {
		size++
		return true
	})
	return size
}
