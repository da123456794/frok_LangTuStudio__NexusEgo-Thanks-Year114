package blob_hash_cache

import (
	"fmt"
	"sort"
	"time"

	blob_hash_packet "github.com/LangTuStudio/Conbit/Conbit/modules/blob_hash/packet"
)

// gcBySchedule 对底层缓存数据集进行垃圾回收。
//
// 与 gc 的不同之处在于，其保证每次垃圾回收的
// 时间总是相隔 GCScheduleTime。
//
// 在设计上，gcBySchedule 作为底层的实现细节，
// 应当只被 UpdateBlobHash 调用并只可能在底层
// 缓存集的长度超过最大长度后发生
func (c *Cache) gcBySchedule() {
	if c.gcIsInSchedule {
		return
	}

	if time.Since(c.lastGCTime) >= GCScheduleTime {
		c.gc()
		c.lastGCTime = time.Now()
		return
	}

	c.gcIsInSchedule = true
	go func() {
		time.Sleep(time.Until(c.lastGCTime.Add(time.Second * 30)))

		c.mu.Lock()
		defer c.mu.Unlock()

		c.gc()
		c.lastGCTime = time.Now()
		c.gcIsInSchedule = false
	}()
}

// gc 对底层缓存数据集进行垃圾回收。
//
// 在设计上，gc 作为底层的实现细节，
// 应当只被 gcBySchedule 调用。
//
// 调用 gc 后，将会从底层缓存集删除前
// 百分之 BlobHashListPopPercent 的
// 元素。
//
// 删除的元素必须是最后更新时间距今超过
// BlobHashCacheValidTime 秒的元素。
//
// 对于长度为 n 的缓存集，回收一次的时间
// 复杂度是 O(n*log(n))
func (c *Cache) gc() {
	if BlobHashCacheDebug {
		fmt.Printf("Cache: Call gc [len(c.mapping) = %d]\n", len(c.mapping))
	}

	slice := make([]hashTimePayload, 0)
	for key, value := range c.mapping {
		slice = append(slice, hashTimePayload{
			Hash:               key,
			LastReciveUnixTime: value.LastReciveUnixTime,
			Payload:            value.Payload,
		})
	}

	sort.SliceStable(slice, func(i, j int) bool {
		return slice[i].LastReciveUnixTime <= slice[j].LastReciveUnixTime
	})
	slice = c.pop(slice, 1-float32(c.maxHashAllow)/float32(len(slice)), BlobHashCacheValidTime)
	slice = c.pop(slice, BlobHashListPopPercent, BlobHashCacheValidTime)

	c.mapping = make(map[blob_hash_packet.Hash]timeAndPayload)
	for _, value := range slice {
		c.mapping[value.Hash] = timeAndPayload{
			LastReciveUnixTime: value.LastReciveUnixTime,
			Payload:            value.Payload,
		}
	}

	if BlobHashCacheDebug {
		fmt.Printf("Cache: Finish gc [len(c.mapping) = %d]\n", len(c.mapping))
	}
}

// pop 将 slice 头部的元素弹出。
//
// 弹出的元素必须满足以下条件。
//   - 这个元素在前 n*percent 处
//   - 这个元素自最后使用已经超过了
//     popMaxThenTime 秒
//
// n 指示 slice 中已有的元素总量。
//
// 您同时需要确保 slice 中的元素
// 已按 LastReciveUnixTime 从小到
// 大排序
func (c *Cache) pop(slice []hashTimePayload, percent float32, popMaxThenTime int) []hashTimePayload {
	if len(slice) == 0 || len(c.mapping) == 0 {
		return slice
	}

	l := 0
	r := len(slice) - 1
	idx := -1

	for {
		if l > r {
			break
		}
		mid := (l + r) / 2
		if time.Since(time.Unix(slice[mid].LastReciveUnixTime, 0)) > time.Second*time.Duration(popMaxThenTime) {
			idx = mid
			l = mid + 1
		} else {
			r = mid - 1
		}
	}

	if idx == -1 {
		return slice
	}

	cut := int(float32(len(slice)) * percent)
	if cut <= 0 {
		return slice
	}

	slice = slice[min(cut, idx)+1:]
	return slice
}
