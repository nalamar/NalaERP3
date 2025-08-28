package db

import (
    "context"
    "time"

    "go.mongodb.org/mongo-driver/mongo"
    "go.mongodb.org/mongo-driver/mongo/options"
)

func ConnectMongo(ctx context.Context, uri string) (*mongo.Client, error) {
    opts := options.Client().ApplyURI(uri)
    cli, err := mongo.NewClient(opts)
    if err != nil {
        return nil, err
    }
    ctxDial, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    if err := cli.Connect(ctxDial); err != nil {
        return nil, err
    }
    ctxPing, cancel2 := context.WithTimeout(ctx, 3*time.Second)
    defer cancel2()
    if err := cli.Ping(ctxPing, nil); err != nil {
        _ = cli.Disconnect(context.Background())
        return nil, err
    }
    return cli, nil
}

