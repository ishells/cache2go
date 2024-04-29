package main

import (
	"fmt"
	"github.com/muesli/cache2go"
	"strconv"
)

func main() {
	cache := cache2go.Cache("myCache")

	// The data loader gets called automatically whenever something
	// tries to retrieve a non-existing key from the cache.
	// 当从cache中访问一个不存在的key时会触发这个回调函数
	cache.SetDataLoader(func(key interface{}, args ...interface{}) *cache2go.CacheItem {
		// Apply some clever loading logic here, e.g. read values for
		// this key from database, network or file.
		// 这里可以做一些机智的处理，比如说从数据库、网络或者文件中读取数据，当然这也是缓存的意义
		// key.(string)是类型断言，将interface{}类型的数据转回到string类型
		val := "This is a test with key " + key.(string)

		// This helper method creates the cached item for us. Yay!
		item := cache2go.NewCacheItem(key, 0, val)
		return item
	})

	// Let's retrieve a few auto-generated items from the cache.
	for i := 0; i < 10; i++ {
		res, err := cache.Value("someKey_" + strconv.Itoa(i))
		if err == nil {
			fmt.Println("Found value in cache:", res.Data())
		} else {
			fmt.Println("Error retrieving value from cache:", err)
		}
	}
}
