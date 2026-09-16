package db

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

const (
	connectTimeout                = 10 * time.Second
	pingTimeout                   = 5 * time.Second
	serverSelectionTimeout        = 5 * time.Second
	maxConnIdleTime               = 30 * time.Second
	maxPoolSize            uint64 = 50
	minPoolSize            uint64 = 5
)

func Connect(ctx context.Context, uri string) (*mongo.Client, error) {
	if uri == "" {
		return nil, fmt.Errorf("env MONGODB_URI wajib diisi")
	}

	clientOptions := options.Client().
		ApplyURI(uri).
		SetMaxPoolSize(maxPoolSize).
		SetMinPoolSize(minPoolSize).
		SetConnectTimeout(connectTimeout).
		SetServerSelectionTimeout(serverSelectionTimeout).
		SetMaxConnIdleTime(maxConnIdleTime)

	connectCtx, cancelConnect := context.WithTimeout(ctx, connectTimeout)
	defer cancelConnect()

	client, err := mongo.Connect(connectCtx, clientOptions)
	if err != nil {
		return nil, fmt.Errorf("gagal membuat koneksi MongoDB: %w", err)
	}

	if err := Ping(ctx, client); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, err
	}

	return client, nil
}

func Ping(ctx context.Context, client *mongo.Client) error {
	pingCtx, cancelPing := context.WithTimeout(ctx, pingTimeout)
	defer cancelPing()

	if err := client.Ping(pingCtx, readpref.Primary()); err != nil {
		return fmt.Errorf("gagal ping MongoDB: %w", err)
	}
	return nil
}
