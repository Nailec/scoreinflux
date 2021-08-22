package db

import "go.mongodb.org/mongo-driver/mongo"

type MongoDBClient struct {
	Client   *mongo.Client
	Database string
}
