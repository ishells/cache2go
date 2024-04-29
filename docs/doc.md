#### cache2go参考 [源码阅读参考文章2](https://www.cnblogs.com/zhouweixin/p/16538769.html)
## 核心方法

### Add 新增条目
![add_item](imgs/add_item.jpg)

### expirationCheck 过期检查
- 每次新增条目时，扫描得到最近过期条目的过期时间，仅定义一个定时器。该定时器触发时清除缓存，并生成下一个定时器，如此接力处理。
- 过期检查中会调用方法 table.deleteInternal 来清除过期的 key

![expirationCheck](imgs/expirationCheck.jpg)

### Delete 方法
- 从流程图可以看出，这块儿大部分逻辑是在加锁、释放锁，有这么多锁主要是有如下几个原因：
- 一部分是表级别的，一部分是条目级别的；
表级别锁出现两次获取与释放，这种实现主要是考虑到 deleteInternal 的复用性，同时支持 Delete 和 expirationCheck 的调用，做了一些锁回溯的逻辑。思考：假如 Mutex 是可重入锁，是不是不需要回溯处理了？

![delete](imgs/delete.png)

### Value取值
取值本身是比较简单的，只不过这里要进行一些额外处理：
- 取不到值时，是否有自定义逻辑，比如降级查询后缓存进去
- 取到值时，更新访问时间，达到保活的目的

![value](imgs/value.png)








