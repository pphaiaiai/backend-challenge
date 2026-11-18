// Package mongodb implements application.UserRepository with the official
// MongoDB Go driver.
package mongodb

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

// Connect creates a client and pings the server. opTimeout applies to every
// operation whose context has no deadline.
func Connect(ctx context.Context, uri string, opTimeout time.Duration) (*mongo.Client, error) {
	opts := options.Client().
		ApplyURI(uri).
		SetTimeout(opTimeout)

	client, err := mongo.Connect(opts)
	if err != nil {
		return nil, fmt.Errorf("connect to mongodb: %w", err)
	}

	// mongo.Connect does not contact the server; Ping does.
	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, fmt.Errorf("ping mongodb: %w", err)
	}
	return client, nil
}
