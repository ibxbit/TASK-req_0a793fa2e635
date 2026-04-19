package search

import (
	"log"
	"sync"
	"time"
)

const refreshInterval = 10 * time.Minute

var (
	indexInstance *Index
	initOnce      sync.Once
)

func Engine() *Index {
	initOnce.Do(func() {
		indexInstance = NewIndex()
	})
	return indexInstance
}

func Rebuild() error {
	docs, err := LoadAllDocs()
	if err != nil {
		return err
	}
	syns, _ := LoadSynonyms()
	Engine().Replace(docs, syns)
	log.Printf("search index rebuilt: %d docs, %d synonym entries", len(docs), len(syns))
	return nil
}

// RefreshPoem is the publish/update hook. If the poem is status='published',
// it is upserted into the index; otherwise it is removed.
func RefreshPoem(id int64) {
	doc, err := LoadDoc(id)
	if err != nil {
		log.Printf("search refresh poem %d: %v", id, err)
		Engine().Remove(id)
		return
	}
	if doc.Status != "published" {
		Engine().Remove(id)
		return
	}
	Engine().Upsert(doc)
}

// RemoveFromIndex is called when a poem is deleted.
func RemoveFromIndex(id int64) {
	Engine().Remove(id)
}

func StartScheduler() {
	go func() {
		if err := Rebuild(); err != nil {
			log.Printf("initial index build: %v", err)
		}
		t := time.NewTicker(refreshInterval)
		defer t.Stop()
		for range t.C {
			if err := Rebuild(); err != nil {
				log.Printf("scheduled rebuild: %v", err)
			}
		}
	}()
	log.Println("search index scheduler started (interval=10m)")
}
