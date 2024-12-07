// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package worker

import (
	"sync"

	"golang.org/x/pkgsite/internal/postgres"
)

type loadShedder struct {
	// The maximum size of requests that can be processed at once. If an
	// incoming request would cause sizeInFlight to exceed this value, it won't
	// be processed.
	maxSizeInFlight uint64

	// Function to get information about DB status.
	getDBInfo func() *postgres.UserInfo

	// Protects the variables below, and also serializes shedding decisions so
	// multiple simultaneous requests are handled properly.
	mu sync.Mutex

	sizeInFlight     uint64 // size of requests currently in progress.
	requestsInFlight int    // number of request currently in progress
	requestsTotal    int    // total fetch requests ever seen
	requestsShed     int    // number of requests that were shedded
}

// Don't load-shed based on DB lock contention unless there are at least this
// many DB worker processes.
const minDBProcessesToShed = 5

// shouldShed reports whether a request of size should be shed (not processed).
// Its second return value is a function that should be deferred by the caller.
func (ls *loadShedder) shouldShed(size uint64) (_ bool, deferFunc func()) {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	ls.requestsTotal++
	// Shed if size exceeds our limit--except that if nothing is being
	// processed, accept this request to avoid starving it forever.
	if ls.sizeInFlight > 0 && ls.sizeInFlight+size > ls.maxSizeInFlight {
		ls.requestsShed++
		return true, func() {}
	}

	if ls.getDBInfo != nil {
		// Shed if the DB is too busy.
		// That is, if there are more than a handful of worker processes, and
		// a large fraction of them is waiting for locks.
		ui := ls.getDBInfo()
		if ui.NumTotal >= minDBProcessesToShed && ui.NumWaiting > ui.NumTotal/2 {
			ls.requestsShed++
			return true, func() {}
		}
	}

	// Don't shed.
	ls.sizeInFlight += size
	ls.requestsInFlight++
	return false, func() {
		ls.mu.Lock()
		defer ls.mu.Unlock()
		ls.sizeInFlight -= size
		ls.requestsInFlight--
	}
}

// LoadShedStats holds statistics about load shedding.
type LoadShedStats struct {
	SizeInFlight     uint64
	MaxSizeInFlight  uint64
	RequestsInFlight int
	RequestsShed     int
	RequestsTotal    int
}

func (ls *loadShedder) stats() LoadShedStats {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	return LoadShedStats{
		RequestsInFlight: ls.requestsInFlight,
		SizeInFlight:     ls.sizeInFlight,
		MaxSizeInFlight:  ls.maxSizeInFlight,
		RequestsShed:     ls.requestsShed,
		RequestsTotal:    ls.requestsTotal,
	}
}
