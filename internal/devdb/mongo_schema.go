package devdb

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// sampleDocs is how many documents describe() inspects to infer field types.
const sampleDocs = 20

// collections lists every collection with its estimated document count.
func (m *mongoDB) collections(ctx context.Context) (string, error) {
	names, err := m.db.ListCollectionNames(ctx, bson.D{})
	if err != nil {
		return "", err
	}
	sort.Strings(names)
	var b strings.Builder
	for _, n := range names {
		cnt, err := m.db.Collection(n).EstimatedDocumentCount(ctx)
		if err != nil {
			fmt.Fprintf(&b, "%s (count unavailable)\n", n)
			continue
		}
		fmt.Fprintf(&b, "%s (~%d docs)\n", n, cnt)
	}
	fmt.Fprintf(&b, "(%d collections)", len(names))
	return b.String(), nil
}

// describe shows a collection's indexes and the top-level field types seen in
// its newest documents (MongoDB has no fixed schema to read).
func (m *mongoDB) describe(ctx context.Context, name string) (string, error) {
	c := m.db.Collection(name)
	var b strings.Builder
	b.WriteString("## indexes\n")
	if cur, err := c.Indexes().List(ctx); err == nil {
		var idx []bson.M
		if err := cur.All(ctx, &idx); err == nil {
			for _, i := range idx {
				fmt.Fprintf(&b, "%v key=%v unique=%v\n", i["name"], i["key"], i["unique"] == true)
			}
		}
	}
	cur, err := c.Find(ctx, bson.D{}, options.Find().SetSort(bson.D{{Key: "_id", Value: -1}}).SetLimit(sampleDocs))
	if err != nil {
		return "", err
	}
	var docs []bson.D
	if err := cur.All(ctx, &docs); err != nil {
		return "", err
	}
	types := map[string]map[string]bool{}
	var order []string
	for _, d := range docs {
		for _, e := range d {
			if types[e.Key] == nil {
				types[e.Key] = map[string]bool{}
				order = append(order, e.Key)
			}
			types[e.Key][fmt.Sprintf("%T", e.Value)] = true
		}
	}
	fmt.Fprintf(&b, "\n## fields (from the %d newest documents)\n", len(docs))
	for _, k := range order {
		var ts []string
		for t := range types[k] {
			ts = append(ts, strings.TrimPrefix(strings.TrimPrefix(t, "bson."), "primitive."))
		}
		sort.Strings(ts)
		fmt.Fprintf(&b, "%s: %s\n", k, strings.Join(ts, " | "))
	}
	return b.String(), nil
}
