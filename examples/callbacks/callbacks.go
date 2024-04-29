package main

import (
	"fmt"
	"time"

	"github.com/muesli/cache2go"
)

func main() {
	// 创建一个 myCache 的缓存表
	cache := cache2go.Cache("myCache")

	// This callback will be triggered every time a new item
	// gets added to the cache.
	// 设置每次新item被添加到缓存表时会被触发的回调函数
	// SetAddedItemCallback 方法仅仅执行append操作将传入的参数添加到 table.addedItem 切片中，即添加新item时的回调函数清单
	cache.SetAddedItemCallback(func(entry *cache2go.CacheItem) {
		fmt.Println("Added Callback 1:", entry.Key(), entry.Data(), entry.CreatedOn())
	})
	// AddAddedItemCallback 方法与 SetAddedItemCallback 区别是
	// AddAddedItemCallback是首次设置添加新item时的回调函数清单，会执行 table.addedItem 清空操作，而 AddAddedItemCallback仅执行追加操作
	cache.AddAddedItemCallback(func(entry *cache2go.CacheItem) {
		fmt.Println("Added Callback 2:", entry.Key(), entry.Data(), entry.CreatedOn())
	})
	// 上面这两个函数的作用就是在添加新item时，输出新item的Key、Data、CreatedOn值

	// This callback will be triggered every time an item
	// is about to be removed from the cache.
	// 作用与上面两个函数类似，设置item被删除时调用的回调函数，打印要删除的item的Key、Data、CreatedOn值
	cache.SetAboutToDeleteItemCallback(func(entry *cache2go.CacheItem) {
		fmt.Println("Deleting:", entry.Key(), entry.Data(), entry.CreatedOn())
	})

	// Caching a new item will execute the AddedItem callback.
	// 向缓存表添加一个测试item
	cache.Add("someKey", 0, "This is a test!")

	// Let's retrieve the item from the cache
	// 从缓存表读取上面的测试item
	res, err := cache.Value("someKey")
	if err == nil {
		fmt.Println("Found value in cache:", res.Data())
	} else {
		fmt.Println("Error retrieving value from cache:", err)
	}

	// Deleting the item will execute the AboutToDeleteItem callback.
	// 删除测试item，同时会触发AboutToDeleteItem回调函数（该函数在deleteInternal方法中被调用）
	cache.Delete("someKey")

	// 清除要删除item的所有addedItem回调函数
	cache.RemoveAddedItemCallbacks()
	// Caching a new item that expires in 3 seconds
	// 添加一个新的测试item，设置3s的存活时间
	res = cache.Add("anotherKey", 3*time.Second, "This is another test")

	// This callback will be triggered when the item is about to expire
	// 一旦触发了删除操作就会调用到下面这个回调函数，在这里也就是3s到期时被执行
	res.SetAboutToExpireCallback(func(key interface{}) {
		fmt.Println("About to expire:", key.(string))
	})

	// 为了等待上面的3s时间到
	time.Sleep(5 * time.Second)
}
