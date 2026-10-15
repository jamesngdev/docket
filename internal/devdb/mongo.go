package devdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

// mongoDB only exposes read operations; aggregation stages that write are rejected.
type mongoDB struct {
	client *mongo.Client
	db     *mongo.Database
}

// writeStages are aggregation stages that write to the database.
var writeStages = []string{"$out", "$merge"}

func openMongo(ctx context.Context, raw string) (Engine, error) {
	cs := strings.TrimSpace(raw)
	client, err := mongo.Connect(options.Client().ApplyURI(cs).SetReadPreference(readpref.PrimaryPreferred()))
	if err != nil {
		return nil, fmt.Errorf("connect mongodb: %w", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, fmt.Errorf("connect mongodb: %w", err)
	}
	name := mongoDBName(cs)
	if name == "" {
		_ = client.Disconnect(context.Background())
		return nil, errors.New("mongodb URL must name the database: mongodb://host:27017/<database>")
	}
	return &mongoDB{client: client, db: client.Database(name)}, nil
}

// mongoDBName is the database path segment of a mongodb:// URL.
func mongoDBName(cs string) string {
	rest := cs[strings.Index(cs, "://")+3:]
	i := strings.IndexByte(rest, '/')
	if i < 0 {
		return ""
	}
	name, _, _ := strings.Cut(rest[i+1:], "?")
	return name
}

func (m *mongoDB) Close() { _ = m.client.Disconnect(context.Background()) }

func (m *mongoDB) Tools() []Tool {
	coll := str("collection name")
	return []Tool{
		{Name: "db_schema", Description: "MongoDB dev database structure. Without `collection`: every collection with its estimated " +
			"document count. With `collection`: its indexes and the field types seen in a few sample documents.",
			InputSchema: schema(map[string]any{"collection": str("optional collection name")})},
		{Name: "mongo_find", Description: fmt.Sprintf("Read documents from a collection of the MongoDB dev database (read-only). "+
			"filter/projection/sort are MongoDB Extended JSON, e.g. {\"_id\": {\"$oid\": \"…\"}}, {\"createdAt\": -1}. "+
			"Returns at most %d documents; use mongo_count for totals.", maxRows),
			InputSchema: schema(map[string]any{"collection": coll, "filter": str("query filter, default {}"),
				"projection": str("optional projection"), "sort": str("optional sort"),
				"limit": map[string]any{"type": "integer", "description": fmt.Sprintf("max documents (≤ %d)", maxRows)}}, "collection")},
		{Name: "mongo_count", Description: "Count documents matching a filter (Extended JSON) in a collection.",
			InputSchema: schema(map[string]any{"collection": coll, "filter": str("query filter, default {}")}, "collection")},
		{Name: "mongo_aggregate", Description: fmt.Sprintf("Run a read-only aggregation pipeline (JSON array, Extended JSON). "+
			"$out and $merge are rejected. Returns at most %d documents.", maxRows),
			InputSchema: schema(map[string]any{"collection": coll, "pipeline": str("pipeline as a JSON array")}, "collection", "pipeline")},
	}
}

type mongoArgs struct {
	Collection, Filter, Projection, Sort, Pipeline string
	Limit                                          int64
}

func (m *mongoDB) Call(ctx context.Context, tool string, raw json.RawMessage) (string, error) {
	var a mongoArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return "", fmt.Errorf("bad arguments: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	if tool == "db_schema" {
		if a.Collection == "" {
			return m.collections(ctx)
		}
		return m.describe(ctx, a.Collection)
	}
	if a.Collection == "" {
		return "", errors.New("collection is required")
	}
	c := m.db.Collection(a.Collection)
	filter, err := extJSON(a.Filter, "filter")
	if err != nil {
		return "", err
	}
	switch tool {
	case "mongo_count":
		n, err := c.CountDocuments(ctx, filter)
		return fmt.Sprintf("%d documents", n), err
	case "mongo_find":
		limit := a.Limit
		if limit <= 0 || limit > maxRows {
			limit = maxRows
		}
		opts := options.Find().SetLimit(limit + 1) // ctx deadline also bounds server time
		if a.Projection != "" {
			p, err := extJSON(a.Projection, "projection")
			if err != nil {
				return "", err
			}
			opts.SetProjection(p)
		}
		if a.Sort != "" {
			s, err := extJSON(a.Sort, "sort")
			if err != nil {
				return "", err
			}
			opts.SetSort(s)
		}
		cur, err := c.Find(ctx, filter, opts)
		if err != nil {
			return "", err
		}
		return collect(ctx, cur, int(limit))
	case "mongo_aggregate":
		pipe, err := pipeline(a.Pipeline)
		if err != nil {
			return "", err
		}
		cur, err := c.Aggregate(ctx, append(pipe, bson.D{{Key: "$limit", Value: maxRows + 1}}))
		if err != nil {
			return "", err
		}
		return collect(ctx, cur, maxRows)
	}
	return "", fmt.Errorf("unknown tool %q", tool)
}

// pipeline parses an aggregation pipeline and rejects stages that write.
func pipeline(raw string) ([]bson.D, error) {
	var doc struct { // Extended JSON must be a document, so wrap the array
		P []bson.D `bson:"p"`
	}
	if err := bson.UnmarshalExtJSON([]byte(`{"p":`+raw+`}`), false, &doc); err != nil {
		return nil, fmt.Errorf("pipeline must be a JSON array of stages: %w", err)
	}
	stages := doc.P
	for _, st := range stages { // $out/$merge are only valid as top-level stages
		for _, e := range st {
			for _, w := range writeStages {
				if e.Key == w {
					return nil, fmt.Errorf("%s writes to the database and is not allowed", w)
				}
			}
		}
	}
	return stages, nil
}

func extJSON(raw, what string) (bson.D, error) {
	if strings.TrimSpace(raw) == "" {
		return bson.D{}, nil
	}
	var d bson.D
	if err := bson.UnmarshalExtJSON([]byte(raw), false, &d); err != nil {
		return nil, fmt.Errorf("%s is not valid Extended JSON: %w", what, err)
	}
	return d, nil
}

// collect renders up to limit documents as relaxed Extended JSON lines.
func collect(ctx context.Context, cur *mongo.Cursor, limit int) (string, error) {
	defer cur.Close(context.Background())
	var rows []string
	for cur.Next(ctx) {
		if len(rows) == limit {
			return render(rows, true), nil
		}
		b, err := bson.MarshalExtJSON(cur.Current, false, false)
		if err != nil {
			return "", err
		}
		rows = append(rows, string(b))
	}
	return render(rows, false), cur.Err()
}
