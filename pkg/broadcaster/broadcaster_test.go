package broadcaster

import (
	"fmt"
	"sync/atomic"
	"sync"
	"time"
	"testing"
)

var lockerr sync.Mutex
var totalMessages int

// Add and remove Subscribers
// PoolSize() should reflect proper size
func TestNormalSubUnsub(t *testing.T) {
	c := make(chan SocketData)
	broadcaster := NewBroadcaster(c)
	subs, _, _ := createSubscribers(t, broadcaster, 1)
	expectPoolSize(t, broadcaster, 1)
	broadcaster.Unsubscribe(subs[0])
	if err := broadcaster.Unsubscribe(subs[0]); err == nil {
		t.Error("Unsubscribing same subscriber twice did not return error")
	}
	expectPoolSize(t, broadcaster, 0)
}

// Ensure Broadcaster expires and blocks sub/unsub
func TestExpiredSubUnsub(t *testing.T) {
	c := make(chan SocketData)
	broadcaster := NewBroadcaster(c)
	closeAwaitExpired(c, broadcaster)
	processFunc := func(s SocketData) bool{return true}
	sub, err := broadcaster.Subscribe(processFunc)
	if err == nil {
		t.Error("Expired broadcaster did NOT error on creating new subscription")
	}
	if err = broadcaster.Unsubscribe(sub); err == nil {
		t.Error("Expired broadcaster did NOT error unsubscribing")
	}
}

// Ensure closing the underlying channel clears the subscriber pool properly
func TestBroadcasterClose(t *testing.T) {
	c := make(chan SocketData)
	broadcaster := NewBroadcaster(c)
	createSubscribers(t, broadcaster, 1)
	closeAwaitExpired(c, broadcaster)
	expectPoolSize(t, broadcaster, 0)
}

// Ensure all subscribers receive all messages sent out
// No subscribers unsubscribe during message broadcast/fan out
func TestSimpleDataSend(t *testing.T) {
	c := make(chan SocketData)
	broadcaster := NewBroadcaster(c)
	const numberOfSubs int32 = 100
	_, getMessageFuncs, _ := createSubscribers(t, broadcaster, numberOfSubs)
	const numberOfMessages int32 = 10
	sendData(c, numberOfMessages)
	time.Sleep(time.Second)
	fmt.Println("POOL SIZE:",broadcaster.PoolSize())
	for i := range getMessageFuncs {
		expectSubscriberSynced(t, numberOfMessages, getMessageFuncs[i], i)
	}
	expectPoolSize(t, broadcaster, numberOfSubs)
}

// Ensure no blocking when subscriber unsubs during message broadcast
// func TestUnsubDataSend(t *testing.T) {
// 	c := make(chan SocketData)
// 	broadcaster := NewBroadcaster(c)
// 	// First will listen
// 	// Second will unsubscribe
// 	const numberOfSubs int32 = 2
// 	subs, getMessageFuncs, _ := createSubscribers(t, broadcaster, numberOfSubs)
// 	sendData(c, 1)
// 	// Data has already been received by broadcaster
// 	broadcaster.Unsubscribe(subs[1])
// 	sendData(c, 1)
// 	time.Sleep(time.Second)
// 	fmt.Println("POOL SIZE:",broadcaster.PoolSize())
// 	// Ensure remaining subscriber received all messages
// 	expectSubscriberSynced(t, 2, getMessageFuncs[0], 0)
// 	expectPoolSize(t, broadcaster, 1)
// }

// Testing utility functions below

func expectSubscriberSynced(t *testing.T, expectedMessages int32, messagesReceived func()int32, identifier int) {
	t.Helper()
	fatalTimeout := time.Now().Add(time.Second)
	for {
		actualMessages := messagesReceived()
		if actualMessages == expectedMessages {
			return
		}
		if time.Now().After(fatalTimeout) {
			t.Errorf("All messages were not returned: expected %d, actual %d\n", expectedMessages, actualMessages)
			t.Fatalf("Timeout waiting for subscriber %d to be synced\n",identifier)
		}
	}
}

// Returns functions that have closure on shared counter
func subscriberRwFuncs() (func() int32, func()) {
	var messages int32
	getMessages := func() int32 {
		return atomic.LoadInt32(&messages)
	}
	incrementMessages := func() {
		atomic.AddInt32(&messages, 1)
	}
	return getMessages, incrementMessages
}

// Return subscriber slice with requested number of subscribers
func createSubscribers(t *testing.T, b *Broadcaster, reqSubs int32) ([]*Subscriber, []func()int32, error) {
	subscriberList := []*Subscriber{}
	getMessagesList := []func()int32{}
	for i := 0; int32(i) < reqSubs; i++ {
		reader, writer := subscriberRwFuncs()
		// Increment counter for each message received
		processFunc := func(s SocketData) bool {
			writer()
			lockerr.Lock()
			totalMessages++
			fmt.Println("subscriber messages",totalMessages)
			lockerr.Unlock()
			return true
		}
		sub, err := b.Subscribe(processFunc)
		if err != nil {
			return []*Subscriber{}, nil, err
		}
		subscriberList = append(subscriberList, sub)
		getMessagesList = append(getMessagesList, reader)
	}
	return subscriberList, getMessagesList, nil
}

// Sends data to broadcaster channel
// Blocking operation
func sendData(c chan SocketData, numberOfTimes int32) {
	for i := 0; int32(i) < numberOfTimes; i++ {
		c <- SocketData{}
	}
}

// Close broadcaster channel
// Waits until all subscribers have been removed
func closeAwaitExpired(c chan SocketData, b *Broadcaster) {
	close(c)
	for {
		if b.Expired() {
			break
		}
	}
}

func expectPoolSize(t *testing.T, b *Broadcaster, expected int32) {
	if poolSize := b.PoolSize(); int32(poolSize) != expected {
		t.Errorf("Expected Poolsize: %d, Actual: %d\n", expected, poolSize)
	}
}
