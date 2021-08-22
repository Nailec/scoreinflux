package db

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
)

var ColCompet string = "competitions"

func (c *MongoDBClient) InsertCompetition(url string) error {
	_, err := c.Client.Database(c.Database).Collection(ColCompet).InsertOne(
		context.TODO(),
		bson.M{"_id": url},
	)
	return err
}
