package search

import "sync"

const (
	FieldTitle       = 0
	FieldFirstLine   = 1
	FieldBody        = 2
	FieldTranslation = 3
	FieldPreface     = 4
)

var fieldBoost = map[int]float64{
	FieldTitle:       5.0,
	FieldFirstLine:   3.0,
	FieldBody:        1.0,
	FieldTranslation: 1.0,
	FieldPreface:     1.0,
}

func fieldName(f int) string {
	switch f {
	case FieldTitle:
		return "title"
	case FieldFirstLine:
		return "first_line"
	case FieldBody:
		return "body"
	case FieldTranslation:
		return "translation"
	case FieldPreface:
		return "preface"
	}
	return "unknown"
}

type Posting struct {
	PoemID    int64
	Field     int
	Positions []int
}

type Doc struct {
	ID             int64
	Title          string
	FirstLine      string
	Body           string
	Translation    string
	Preface        string
	AuthorID       *int64
	DynastyID      *int64
	MeterPatternID *int64
	Status         string
	TagIDs         []int64
}

type Index struct {
	mu       sync.RWMutex
	postings map[string][]Posting
	docs     map[int64]*Doc
	synonyms map[string][]string
}

func NewIndex() *Index {
	return &Index{
		postings: make(map[string][]Posting),
		docs:     make(map[int64]*Doc),
		synonyms: make(map[string][]string),
	}
}

func (idx *Index) Upsert(d *Doc) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.removeDocUnsafe(d.ID)
	idx.docs[d.ID] = d
	idx.indexAllFieldsUnsafe(d)
}

func (idx *Index) Remove(id int64) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.removeDocUnsafe(id)
}

func (idx *Index) Replace(docs []*Doc, synonyms map[string][]string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.postings = make(map[string][]Posting)
	idx.docs = make(map[int64]*Doc)
	for _, d := range docs {
		idx.docs[d.ID] = d
		idx.indexAllFieldsUnsafe(d)
	}
	if synonyms == nil {
		idx.synonyms = make(map[string][]string)
	} else {
		idx.synonyms = synonyms
	}
}

func (idx *Index) Size() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.docs)
}

func (idx *Index) removeDocUnsafe(id int64) {
	if _, ok := idx.docs[id]; !ok {
		return
	}
	delete(idx.docs, id)
	for term, list := range idx.postings {
		kept := list[:0]
		for _, p := range list {
			if p.PoemID != id {
				kept = append(kept, p)
			}
		}
		if len(kept) == 0 {
			delete(idx.postings, term)
		} else {
			idx.postings[term] = kept
		}
	}
}

func (idx *Index) indexAllFieldsUnsafe(d *Doc) {
	idx.indexFieldUnsafe(d.ID, FieldTitle, d.Title)
	idx.indexFieldUnsafe(d.ID, FieldFirstLine, d.FirstLine)
	idx.indexFieldUnsafe(d.ID, FieldBody, d.Body)
	idx.indexFieldUnsafe(d.ID, FieldTranslation, d.Translation)
	idx.indexFieldUnsafe(d.ID, FieldPreface, d.Preface)
}

func (idx *Index) indexFieldUnsafe(id int64, field int, text string) {
	toks := Tokenize(text)
	if len(toks) == 0 {
		return
	}
	posMap := make(map[string][]int)
	for i, t := range toks {
		posMap[t] = append(posMap[t], i)
	}
	for term, positions := range posMap {
		idx.postings[term] = append(idx.postings[term], Posting{
			PoemID:    id,
			Field:     field,
			Positions: positions,
		})
	}
}
