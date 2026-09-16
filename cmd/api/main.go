package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"agem/internal/config"
	"agem/internal/db"
	"agem/internal/router"
	"agem/internal/store"

	"go.mongodb.org/mongo-driver/mongo"
)

// @title AGEM Backend API
// @version 3.0
// @description Backend API for AGEM/QRing wearable users, devices, pairing, multi-day sensor synchronization, sleep sessions, activity, events, raw samples, and workouts.
// @BasePath /
func main() {
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	client, err := db.Connect(ctx, cfg.MongoURI)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer func() {
		disconnectCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = client.Disconnect(disconnectCtx)
	}()

	var regeneClient *mongo.Client
	if cfg.RegeneMongoURI != "" {
		if cfg.RegeneMongoDBName == "" {
			log.Fatal("regene db: database name is empty")
		}
		regeneClient, err = db.Connect(ctx, cfg.RegeneMongoURI)
		if err != nil {
			log.Fatalf("regene db: %v", err)
		}
		defer func() {
			disconnectCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = regeneClient.Disconnect(disconnectCtx)
		}()
	}

	st := store.New(client.Database(cfg.MongoDBName))
	if regeneClient != nil {
		st = store.NewWithUserSource(
			client.Database(cfg.MongoDBName),
			regeneClient.Database(cfg.RegeneMongoDBName),
			cfg.RegeneUsersCollection,
		)
		log.Printf("using Regene user source database=%s collection=%s", cfg.RegeneMongoDBName, cfg.RegeneUsersCollection)
	}
	if err := st.EnsureIndexes(ctx); err != nil {
		log.Fatalf("db indexes: %v", err)
	}
	if err := st.EnsureSDKV3Schema(ctx); err != nil {
		log.Fatalf("db v3 schema: %v", err)
	}

	handler := router.Router(st)
	srv := &http.Server{
		Addr:         cfg.Addr,
		Handler:      handler,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	go func() {
		log.Printf("listening on %s", cfg.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	log.Println("shutdown complete")
}
