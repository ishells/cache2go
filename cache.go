/*
 * Simple caching library with expiration capabilities
 *     Copyright (c) 2012, Radu Ioan Fericean
 *                   2013-2017, Christian Muehlhaeuser <muesli@gmail.com>
 *
 *   For license see LICENSE.txt
 */

package cache2go

import (
	"sync"
)

var (
	cache = make(map[string]*CacheTable)
	mutex sync.RWMutex
)

// Cache returns the existing cache table with given name or creates a new one
// if the table does not exist yet.
// Cache函数，返回指定名字的表，如果表不存在则创建一个空表返回
func Cache(table string) *CacheTable {
	mutex.RLock()
	// cache的类型，是一个用于存CacheTable的map
	t, ok := cache[table]
	mutex.RUnlock()

	if !ok {
		// 如果表不存在的时候需要创建一个空表，这时候同时做了一个读写锁和二次检查，为的是并发安全
		mutex.Lock()
		t, ok = cache[table]
		// Double check whether the table exists or not.
		if !ok {
			t = &CacheTable{
				name:  table,
				items: make(map[interface{}]*CacheItem),
			}
			cache[table] = t
		}
		mutex.Unlock()
	}

	return t
}
