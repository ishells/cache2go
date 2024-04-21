/*
 * Simple caching library with expiration capabilities
 *     Copyright (c) 2013-2017, Christian Muehlhaeuser <muesli@gmail.com>
 *
 *   For license see LICENSE.txt
 */

package cache2go

import (
	"log"
	"sort"
	"sync"
	"time"
)

// CacheTable is a table within the cache
type CacheTable struct {
	sync.RWMutex

	// The table's name.
	name string
	// [ 一个表中的所有条目都存在这个map里，map的key是interface即任意类型，value是CacheItem指针类型 ]
	// All cached items.
	items map[interface{}]*CacheItem

	// [ 负责触发清除操作的计时器 ]
	// Timer responsible for triggering cleanup.
	cleanupTimer *time.Timer
	// [ 触发清除操作的时间间隔 ]
	// Current timer duration.
	cleanupInterval time.Duration

	// The logger used for this table.
	logger *log.Logger

	// [ 尝试加载一个不存在的key时触发的回调函数 ]
	// Callback method triggered when trying to load a non-existing key.
	loadData func(key interface{}, args ...interface{}) *CacheItem

	// [ 添加一个新item时触发的回调函数 ]
	// Callback method triggered when adding a new item to the cache.
	addedItem []func(item *CacheItem)

	// [ 删除item前触发的回调函数 ]
	// Callback method triggered before deleting an item from the cache.
	aboutToDeleteItem []func(item *CacheItem)
}

// Count returns how many items are currently stored in the cache.
// Count 函数返回指定的CacheTable中item的条目数量
func (table *CacheTable) Count() int {
	table.RLock()
	defer table.RUnlock()
	// table.items 是一个map，len() 返回map的元素数量
	return len(table.items)
}

// Foreach all items
func (table *CacheTable) Foreach(trans func(key interface{}, item *CacheItem)) {
	table.RLock()
	defer table.RUnlock()
	// 遍历items，把每个key和value都丢给trans函数来处理
	for k, v := range table.items {
		trans(k, v)
	}
}

// SetDataLoader configures a data-loader callback, which will be called when
// trying to access a non-existing key. The key and 0...n additional arguments
// are passed to the callback function.
// 形参列表是一个函数，函数的参数是一个interface{}类型的key和不固定数目的额外参数，返回值是CacheItem指针
func (table *CacheTable) SetDataLoader(f func(interface{}, ...interface{}) *CacheItem) {
	table.Lock()
	defer table.Unlock()
	// 形参f函数被丢给了table的loadData属性，loadData所指向的方法什么时候被调用？
	// 作者注释说是当访问一个不存在的key时，需要调用一个方法，这个方法通过SetDataLoader设定，方法的实现由用户来定义
	table.loadData = f
}

// SetAddedItemCallback configures a callback, which will be called every time
// a new item is added to the cache.
// 创建新item时被调用的回调方法
// 这里 SetAddedItemCallback方法的形参名是f，类型是func(*CacheItem)，也就是说func(*CacheItem)被作为一个类型
// 这样定义的话，在函数内部调用该f时，传入一个*CacheItem类型的实参即可，传递实参的参数名并不重要（况且这里函数内部也）
/*
比如，定义变量show为func(int)类型的时候没有设置形参变量名称，调用时随便定义即可
func main() {
    var show func(int)
    show = func(num int) { fmt.Println(num) }
    show(123)
}
*/
func (table *CacheTable) SetAddedItemCallback(f func(*CacheItem)) {
	if len(table.addedItem) > 0 {
		table.RemoveAddedItemCallbacks()
	}
	table.Lock()
	defer table.Unlock()
	table.addedItem = append(table.addedItem, f)
}

// AddAddedItemCallback appends a new callback to the addedItem queue
func (table *CacheTable) AddAddedItemCallback(f func(*CacheItem)) {
	table.Lock()
	defer table.Unlock()
	table.addedItem = append(table.addedItem, f)
}

// RemoveAddedItemCallbacks empties the added item callback queue
func (table *CacheTable) RemoveAddedItemCallbacks() {
	table.Lock()
	defer table.Unlock()
	table.addedItem = nil
}

// SetAboutToDeleteItemCallback configures a callback, which will be called
// every time an item is about to be removed from the cache.
func (table *CacheTable) SetAboutToDeleteItemCallback(f func(*CacheItem)) {
	if len(table.aboutToDeleteItem) > 0 {
		table.RemoveAboutToDeleteItemCallback()
	}
	table.Lock()
	defer table.Unlock()
	table.aboutToDeleteItem = append(table.aboutToDeleteItem, f)
}

// AddAboutToDeleteItemCallback appends a new callback to the AboutToDeleteItem queue
func (table *CacheTable) AddAboutToDeleteItemCallback(f func(*CacheItem)) {
	table.Lock()
	defer table.Unlock()
	table.aboutToDeleteItem = append(table.aboutToDeleteItem, f)
}

// RemoveAboutToDeleteItemCallback empties the about to delete item callback queue
func (table *CacheTable) RemoveAboutToDeleteItemCallback() {
	table.Lock()
	defer table.Unlock()
	table.aboutToDeleteItem = nil
}

// SetLogger sets the logger to be used by this cache table.
// 把一个logger实例丢给table的logger属性
func (table *CacheTable) SetLogger(logger *log.Logger) {
	table.Lock()
	defer table.Unlock()
	table.logger = logger
}

// Expiration check loop, triggered by a self-adjusting timer.
// 由计时器触发的到期检查
func (table *CacheTable) expirationCheck() {
	table.Lock()
	// 负责触发清除操作的计时器暂停
	if table.cleanupTimer != nil {
		table.cleanupTimer.Stop()
	}
	// 计时器的时间间隔
	if table.cleanupInterval > 0 {
		table.log("Expiration check triggered after", table.cleanupInterval, "for table", table.name)
	} else {
		table.log("Expiration check installed for table", table.name)
	}

	// To be more accurate with timers, we would need to update 'now' on every
	// loop iteration. Not sure it's really efficient though.
	// 当前时间
	now := time.Now()
	// 定义一个最小时间间隔（后面用于赋值给table的cleanupInterval属性，即触发清除操作的时间间隔），初始化定义为0，下面会更新
	smallestDuration := 0 * time.Second
	// 遍历一个table中的items
	for key, item := range table.items {
		// Cache values so we don't keep blocking the mutex.
		item.RLock()
		// lifeSpan代表不再被访问后剩余存活时间
		lifeSpan := item.lifeSpan
		// item的最后一次访问时间
		accessedOn := item.accessedOn
		item.RUnlock()
		// 存活时间为0的item不作处理
		if lifeSpan == 0 {
			continue
		}
		// time.Now().Sub()是计算时间间隔的方法，这里即计算上一次访问时间到现在的时间间隔。
		// 如果时间间隔大于剩余存活时间，说明已经过期了，则删除该item
		if now.Sub(accessedOn) >= lifeSpan {
			// Item has excessed its lifespan.
			// 执行删除操作
			table.deleteInternal(key)
		} else {
			// Find the item chronologically closest to its end-of-lifespan.
			// 这一段else判断主要作用是为了确定 可执行清除item操作的时间间隔值
			// lifeSpan-now.Sub(accessedOn) item不再被访问后的剩余存活时间 - item上次被访问后到现在的时间间隔 = item到现在剩余的存活时间（过期时间）
			// 如果 item到现在剩余的存活时间 小于 这段函数里设置的最小时间间隔，则更新最小时间间隔的值（后面传递给table.cleanupInterval以确定清除间隔）
			if smallestDuration == 0 || lifeSpan-now.Sub(accessedOn) < smallestDuration {
				smallestDuration = lifeSpan - now.Sub(accessedOn)
			}
		}
	}

	// Setup the interval for the next cleanup run.
	// 上面已经找到了最近接过期时间的时间间隔值，这里将这个时间丢给了cleanupInterval（触发清除操作的时间间隔）
	table.cleanupInterval = smallestDuration
	//
	if smallestDuration > 0 {
		// time.Now().AfterFunc() 函数用于在指定的时间段后执行指定的函数
		// cleanupTimer（负责触发清除操作的计时器）被设置为 smallestDuration，时间到之后执行expirationCheck方法
		table.cleanupTimer = time.AfterFunc(smallestDuration, func() {
			// 这里并不是循环启动goroutine，而是启动一个新的goroutine后当前goroutine会退出，这里不会引起goroutine泄漏。
			go table.expirationCheck()
			// expirationCheck方法无非是做一个定期的数据过期检查操作
		})
	}
	table.Unlock()
}

// 先看上层调用者的定义
// 这里 addInternal方法的上层调用者分别为 Add()方法 和 NotFoundAdd() 方法
// 看完这个方法的代码，就会知道这个函数做了两件事情
// 1. 将item添加到table的items属性中，table.items[item.key] = item
// 2. 执行添加item时触发的回调函数，callback(item) 和 判断是否触发过期检查方法expirationCheck()
func (table *CacheTable) addInternal(item *CacheItem) {
	// Careful: do not run this method unless the table-mutex is locked!
	// 调用addInternal方法前，先要加锁
	// It will unlock it for the caller before running the callbacks and checks
	// 它将会在运行回调和检查之前为调用者解锁。
	table.log("Adding item with key", item.key, "and lifespan of", item.lifeSpan, "to table", table.name)
	table.items[item.key] = item

	// Cache values so we don't keep blocking the mutex.
	// cleanupInterval [ 触发清除操作的时间间隔 ]
	expDur := table.cleanupInterval
	// addedItem 保存的是 [ 添加一个新item时触发的回调函数 ]
	addedItem := table.addedItem
	// 将两个值保存到局部变量之后释放锁
	table.Unlock()

	// Trigger callback after adding an item to cache.
	// 局部变量 addedItem 保存的是 [ 添加一个新item时触发的回调函数 ]
	if addedItem != nil {
		// 调用 addedItem 中的回调函数，也就是添加一个item时需要调用的函数
		for _, callback := range addedItem {
			callback(item)
		}
	}

	// If we haven't set up any expiration check timer or found a more imminent item.
	// 注释：如果我们没有设置任何过期检查计时器或者找到一个更紧迫的项。
	// if的第一个条件: item.lifeSpan > 0, 表示当前item的存活时间还没到
	// expDur保存的是 table.cleanupInterval [ 触发清除操作的时间间隔 ],这个值为0，表示还没有设置任何过期检查计时器
	// item.lifeSpan < expDur 表示设置了触发清除操作的时间间隔，但是当前新增的item的存活时间要比时间间隔更短
	// 满足以上条件之后，就要触发expirationCheck方法
	if item.lifeSpan > 0 && (expDur == 0 || item.lifeSpan < expDur) {
		table.expirationCheck()
	}
	// lifeSpan 代表的是item的存活时间，而cleanupInterval是对于一个table来说触发检查还剩余的时间，
	// 如果item的存活时间比触发检查还短，那么就说明需要提前触发expirationCheck操作了
}

// Add adds a key/value pair to the cache.
// Parameter key is the item's cache-key.
// Parameter lifeSpan determines after which time period without an access the item
// will get removed from the cache.
// Parameter data is the item's value.
func (table *CacheTable) Add(key interface{}, lifeSpan time.Duration, data interface{}) *CacheItem {
	// NewCacheItem 函数是cacheitem.go中定义的一个创建CacheItem类型实例的函数，返回值是*CacheItem类型
	item := NewCacheItem(key, lifeSpan, data)

	// Add item to cache.
	table.Lock()
	// 将NewCacheItem()函数返回的*CacheItem指针丢给addInternal方法
	table.addInternal(item)

	return item
}

func (table *CacheTable) deleteInternal(key interface{}) (*CacheItem, error) {
	r, ok := table.items[key]
	if !ok {
		return nil, ErrKeyNotFound
	}

	// Cache value so we don't keep blocking the mutex.
	aboutToDeleteItem := table.aboutToDeleteItem
	table.Unlock()

	// Trigger callbacks before deleting an item from cache.
	if aboutToDeleteItem != nil {
		for _, callback := range aboutToDeleteItem {
			callback(r)
		}
	}

	r.RLock()
	defer r.RUnlock()
	if r.aboutToExpire != nil {
		for _, callback := range r.aboutToExpire {
			callback(key)
		}
	}

	table.Lock()
	table.log("Deleting item with key", key, "created on", r.createdOn, "and hit", r.accessCount, "times from table", table.name)
	delete(table.items, key)

	return r, nil
}

// Delete an item from the cache.
func (table *CacheTable) Delete(key interface{}) (*CacheItem, error) {
	table.Lock()
	defer table.Unlock()

	return table.deleteInternal(key)
}

// Exists returns whether an item exists in the cache. Unlike the Value method
// Exists neither tries to fetch data via the loadData callback nor does it
// keep the item alive in the cache.
func (table *CacheTable) Exists(key interface{}) bool {
	table.RLock()
	defer table.RUnlock()
	_, ok := table.items[key]

	return ok
}

// NotFoundAdd checks whether an item is not yet cached. Unlike the Exists
// method this also adds data if the key could not be found.
// 该方法检查item是否已经被缓存。和Exists方法不同，即使数据并没有被找到，该方法也会添加该数据
func (table *CacheTable) NotFoundAdd(key interface{}, lifeSpan time.Duration, data interface{}) bool {
	table.Lock()
	// 如果key已经被缓存，则返回false
	if _, ok := table.items[key]; ok {
		table.Unlock()
		return false
	}
	// 当item不存在，则添加该数据
	item := NewCacheItem(key, lifeSpan, data)
	table.addInternal(item)

	return true
}

// Value returns an item from the cache and marks it to be kept alive. You can
// pass additional arguments to your DataLoader callback function.
func (table *CacheTable) Value(key interface{}, args ...interface{}) (*CacheItem, error) {
	table.RLock()
	r, ok := table.items[key]
	loadData := table.loadData
	table.RUnlock()

	if ok {
		// Update access counter and timestamp.
		r.KeepAlive()
		return r, nil
	}

	// Item doesn't exist in cache. Try and fetch it with a data-loader.
	if loadData != nil {
		item := loadData(key, args...)
		if item != nil {
			table.Add(key, item.lifeSpan, item.data)
			return item, nil
		}

		return nil, ErrKeyNotFoundOrLoadable
	}

	return nil, ErrKeyNotFound
}

// Flush deletes all items from this cache table.
func (table *CacheTable) Flush() {
	table.Lock()
	defer table.Unlock()

	table.log("Flushing table", table.name)

	table.items = make(map[interface{}]*CacheItem)
	table.cleanupInterval = 0
	if table.cleanupTimer != nil {
		table.cleanupTimer.Stop()
	}
}

// CacheItemPair maps key to access counter
type CacheItemPair struct {
	Key         interface{}
	AccessCount int64
}

// CacheItemPairList is a slice of CacheItemPairs that implements sort.
// Interface to sort by AccessCount.
type CacheItemPairList []CacheItemPair

func (p CacheItemPairList) Swap(i, j int) { p[i], p[j] = p[j], p[i] }
func (p CacheItemPairList) Len() int      { return len(p) }

// Less 函数用来判断CacheItemPairList的第i个CacheItemPairList和第j个CacheItemPairList的AccessCount大小关系，前者大则返回true，反之false
func (p CacheItemPairList) Less(i, j int) bool { return p[i].AccessCount > p[j].AccessCount }

// MostAccessed returns the most accessed items in this cache table
func (table *CacheTable) MostAccessed(count int64) []*CacheItem {
	table.RLock()
	defer table.RUnlock()

	p := make(CacheItemPairList, len(table.items))
	i := 0
	for k, v := range table.items {
		p[i] = CacheItemPair{k, v.accessCount}
		i++
	}
	sort.Sort(p)

	var r []*CacheItem
	c := int64(0)
	for _, v := range p {
		if c >= count {
			break
		}

		item, ok := table.items[v.Key]
		if ok {
			r = append(r, item)
		}
		c++
	}

	return r
}

// Internal logging method for convenience.
func (table *CacheTable) log(v ...interface{}) {
	if table.logger == nil {
		return
	}

	table.logger.Println(v...)
}
