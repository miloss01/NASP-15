package KVEngine

import (
	"Projekat/Settings"
	"Projekat/Structures/Cache"
	"Projekat/Structures/Memtable"
	"Projekat/Structures/SSTable"
	"Projekat/Structures/TokenBucket"
	"Projekat/Structures/Wal"
	"fmt"
)

type KVEngine struct {
	tokenBucket TokenBucket.TokenBucket
	cache Cache.Cache
	wal Wal.WAL
	memtable Memtable.Memtable
}

func (kve *KVEngine) Get(key string) (bool, []byte) {

	if !kve.tokenBucket.UseToken() {
		fmt.Println("Nema dovoljno tokena.")
		return false, nil
	}

	if content, err := kve.memtable.Get(key); err == nil && !content.Tombstone {
		kve.cache.Put(content.Key, content.Value)
		fmt.Println("Nadjeno u memtable.")
		return true, content.Value
	}

	if found, data := kve.cache.Get(key); found {
		fmt.Println("Nadjeno u cache.")
		return true, data
	}

	if data := SSTable.Find(key); data != nil {
		fmt.Println("Nadjeno u data.")
		kve.cache.Put(key, data)
		return true, data
	}

	return false, nil
}

func (kve *KVEngine) Put(key string, data []byte) bool {

	if !kve.tokenBucket.UseToken() {
		fmt.Println("Nema dovoljno tokena.")
		return false
	}

	return true
}

func (kve *KVEngine) Delete(key string) bool {

	if !kve.tokenBucket.UseToken() {
		fmt.Println("Nema dovoljno tokena.")
		return false
	}

	return true
}

func MakeKVEngine() KVEngine {
	settings := Settings.Settings{Path: "settings.json"}
	settings.LoadFromJSON()

	wal := Wal.WAL{}
	wal.Constuct(int(settings.MemtableMaxElements), int(settings.WalMaxSegments))

	kvengine := KVEngine{}
	kvengine.cache = Cache.MakeCache(uint64(settings.CacheMaxElements))
	kvengine.tokenBucket = TokenBucket.MakeTokenBucket(uint64(settings.TokenBucketMaxTokens), int64(settings.TokenBucketInterval))
	kvengine.wal = wal
	kvengine.memtable = *Memtable.New(5, int(settings.MemtableMaxElements))

	return kvengine
}
